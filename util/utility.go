package util

import (
	"log"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	apiutil "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	config *rest.Config
)

func init() {
	flag, err := genericclioptions.NewConfigFlags(false).ToRESTConfig()
	if err != nil {
		log.Fatalf("generic cli config error %s\n", err.Error())
	}
	config = flag
}

func GetGVKFor(source string) {

	restMapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		log.Fatal(err.Error())
	}

	gvk, err := restMapper.KindFor(schema.GroupVersionResource{
		Resource: source,
	})

	if err != nil {
		log.Fatalf("gvk kindfor error %s", err.Error())
	}

	log.Printf("GVK for %s is %v", source, gvk)
}

func GetGVRFor(source string) {
	restMapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		log.Fatal(err.Error())
	}

	gvk, err := restMapper.ResourceFor(schema.GroupVersionResource{
		Resource: source,
	})

	if err != nil {
		log.Fatalf("gvk resource error %s", err.Error())
	}

	log.Printf("GVR for %s is %v", source, gvk)
}
