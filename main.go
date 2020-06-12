package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

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
		go syncServicesController()
	case runModeTarget:
		go syncServicesTarget()
	default:
		go syncServicesController()
		go syncServicesTarget()
	}

	// handle shutdown
	shutdown := make(chan os.Signal)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)

	// wait for shutdown
	<-shutdown

	log.Printf("Exit")
}
