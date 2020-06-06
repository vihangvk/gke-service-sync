package main

import (
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	clientset *kubernetes.Clientset
)

func main() {

	// setup kubernetes client
	clsConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}
	clientset, err = kubernetes.NewForConfig(clsConfig)
	if err != nil {
		log.Fatal(err)
	}

	loadConfig()

	// check run mode and start controller or target
	log.Printf("Starting gke-service-sync in '%s' mode.", defaultConfig.RunMode)
	switch defaultConfig.RunMode {
	case runModeController:
		syncServicesController()
	case runModeTarget:
		syncServicesTarget()
	default:
		syncServicesController()
		syncServicesTarget()
	}

	log.Printf("Exit")
}
