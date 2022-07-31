package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// reference video: https://www.youtube.com/watch?v=lzoWSfvE2yA
// watch vs informer: https://www.youtube.com/watch?v=soyOjOH-Vjc
/*
Dont use watch: watch query api server lot many times for resource events, increase load of api server.
Due to which api server response is affected. Hence informer used: internally it uses watc but efficient in its
usage wit in-memory store, hence dont query api server frequently. List() from api server stores result in in-memory.
Then call watch to update in-memory store. All Get / List serviced by in-memory store.
In case of network failures, watch handling to be done manually but informer, it restores requested event state from previous
state and continues with the next watch without us worrying about.
Use shared informers so we don't crete many informers and eah start to watch for their in-memory store updates and again increase the load. Hence shared informrner via factory.
*/
func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("internal to the cluster config fail: %s\n resorting to external.\n", err.Error())
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			log.Fatalf("External fails %s", err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println("hlo")
		panic(err)
	}
	/*
		dynaclient, err := dynamic.NewForConfig(config)
		if err != nil {
			log.Fatal(err)
		}
	*/
	labelOptions := informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = "app=kubernetes-test"
	})
	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, 30*time.Second, informers.WithNamespace("lets-expose"), labelOptions)
	// log.Println(informerFactory)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	stopper := make(chan struct{})
	controller := NewDeploymentListenerController(clientset, deploymentInformer)
	informerFactory.Start(stopper)
	controller.run(stopper)
	defer close(stopper)

}
