package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	corenetv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	appinformer "k8s.io/client-go/informers/apps/v1"

	"k8s.io/client-go/kubernetes"
	deplister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type DeploymentListenerController struct {
	clientset      kubernetes.Interface
	depCacheSynced cache.InformerSynced
	queue          workqueue.RateLimitingInterface
	depLister      deplister.DeploymentLister
}

func NewDeploymentListenerController(clientset kubernetes.Interface, depInformer appinformer.DeploymentInformer) *DeploymentListenerController {
	dl := &DeploymentListenerController{
		clientset:      clientset,
		depLister:      depInformer.Lister(),
		depCacheSynced: depInformer.Informer().HasSynced,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "lets-expose"),
	}

	depInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    dl.addHandler,
		DeleteFunc: dl.deleteHandler,
	})

	return dl
}

func (dc *DeploymentListenerController) run(ch <-chan struct{}) {
	log.Println("Starting deployment controller ....")
	if !cache.WaitForCacheSync(ch, dc.depCacheSynced) {
		log.Println("waiting for cache to sync")
	}

	go wait.Until(dc.doWork, 2*time.Second, ch)
	<-ch
}

func (dc *DeploymentListenerController) doWork() {
	ok, err := dc.doProcessing()
	if !ok {
		log.Printf("item processed with error %s \n", err.Error())
	}

}

func (dc *DeploymentListenerController) doProcessing() (bool, error) {
	item, shutdown := dc.queue.Get()
	if shutdown {
		return false, errors.New("queue received shutdown, ending")
	}
	defer dc.queue.Forget(item)
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {

		return false, fmt.Errorf("getting key from cache error %s", err.Error())
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {

		return false, fmt.Errorf("splitting key into ns and name error %s", err.Error())
	}

	ctx := context.TODO()
	_, err = dc.clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	// check object is deleted
	if apierrors.IsNotFound(err) {
		// delete service
		err := dc.clientset.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {

			return false, fmt.Errorf("deleting service %s, error %s", name, err.Error())
		}
		// delete ingress
		err = dc.clientset.NetworkingV1().Ingresses(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {

			return false, fmt.Errorf("deleting ingrss %s, error %s", name, err.Error())
		}
		return true, nil
	}

	err = dc.syncDeployment(ns, name)
	if err != nil {
		// re-try
		return false, fmt.Errorf("syncing deployment %s", err.Error())
	}

	return true, nil
}

func (dc *DeploymentListenerController) syncDeployment(ns, name string) error {
	ctx := context.TODO()
	dep, err := dc.depLister.Deployments(ns).Get(name)
	if err != nil {
		return fmt.Errorf("getting deployment from lister %s", err.Error())
	}

	// create service
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dep.Name,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: dep.Spec.Template.Labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "expose-http",
					Port:       5000,
					TargetPort: intstr.Parse(dep.Spec.Template.Spec.Containers[0].Ports[0].Name),
				},
			},
		},
	}

	coresvc, err := dc.clientset.CoreV1().Services(ns).Create(ctx, &svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating service %s", err.Error())
	}

	return dc.createIngress(ctx, coresvc)
}

func (dc *DeploymentListenerController) createIngress(ctx context.Context, svc *corev1.Service) error {
	if svc == nil {
		return errors.New("service is nil. required for creating ingress")
	}
	pathType := "Prefix"
	ingressObj := corenetv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: corenetv1.IngressSpec{
			Rules: []corenetv1.IngressRule{
				{
					IngressRuleValue: corenetv1.IngressRuleValue{
						HTTP: &corenetv1.HTTPIngressRuleValue{
							Paths: []corenetv1.HTTPIngressPath{
								{
									Path:     fmt.Sprintf("/%s", svc.Name),
									PathType: (*corenetv1.PathType)(&pathType),
									Backend: corenetv1.IngressBackend{
										Service: &corenetv1.IngressServiceBackend{
											Name: svc.Name,
											Port: corenetv1.ServiceBackendPort{
												Number: svc.Spec.Ports[0].Port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err := dc.clientset.NetworkingV1().Ingresses(svc.Namespace).Create(ctx, &ingressObj, metav1.CreateOptions{})
	return err

}

// handlers
func (dc *DeploymentListenerController) deleteHandler(obj interface{}) {
	log.Println("deploy delete")
	dc.queue.Add(obj)

}

func (dc *DeploymentListenerController) addHandler(obj interface{}) {
	log.Println("add handler")
	dc.queue.Add(obj)
	/*
		c, ok := obj.(runtime.Object)
		if !ok {
			log.Fatal("not converted")
			return
		}

		s, _, err := scheme.Scheme.ObjectKinds(c)
		if err != nil {
			log.Printf("error is %s %v", err.Error(), ok)
			return
		}

		log.Printf("2. deploy add %v\n", s[0])
	*/
}
