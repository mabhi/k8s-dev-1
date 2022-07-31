package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var kubeconfig = flag.String("kubeconfig", "/home/abhijeet/.kube/config", "(optional) absolute path to the kubeconfig file")

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

	ctx := context.Background()
	ingressList, err := clientset.NetworkingV1().Ingresses("ingress-read").List(ctx, metav1.ListOptions{})
	if err != nil {
		// handle err
		log.Fatalln(err)
	}

	ingressCtrls := ingressList.Items
	// fmt.Printf("%v\n", ingressList)

	if len(ingressCtrls) > 0 {
		for _, ingress := range ingressCtrls {

			// fmt.Println(ingress.ObjectMeta)
			fmt.Printf("\n=====\ningress %s exists in namespace %s %0.0f %v\n", ingress.Name, ingress.Namespace, time.Since(ingress.CreationTimestamp.Time).Minutes(), ingress.Status.LoadBalancer.Ingress)
		}
	} else {
		fmt.Println("no ingress found")
	}

}
