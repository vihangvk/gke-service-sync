package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	listenAddress               = ":8080"
	targetPath                  = "/services"
	syncedResourceAnnotation    = "gke-service-sync/synced"
	syncedResourceAnnotationVal = "synced"
)

var (
	webServer *http.Server
)

func targetHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		log.Printf("received '%s' method instead of %s", req.Method, http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "Only POST method supported.")
	} else {
		input, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := fmt.Sprintf("failed to read request data: %v", err)
			log.Printf(msg)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, msg)
			return
		}
		var se serviceEndpoints
		debug("input: %s", input)
		err = json.Unmarshal(input, &se)
		if err != nil {
			msg := fmt.Sprintf("failed to unmarshal data: %v", err)
			log.Printf(msg)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, msg)
			return
		}
		se.sanitize()

		// annotate
		if se.Service.Annotations == nil {
			se.Service.Annotations = make(map[string]string)
		}
		se.Service.Annotations[syncedResourceAnnotation] = syncedResourceAnnotationVal
		debug("Set annotation for service: '%s' (ns:%s)", se.Service.Name, se.Service.Namespace)
		if se.Endpoints.Annotations == nil {
			se.Endpoints.Annotations = make(map[string]string)
		}
		se.Endpoints.Annotations[syncedResourceAnnotation] = syncedResourceAnnotationVal
		debug("Set annotation for endpoints: '%s' (ns:%s)", se.Endpoints.Name, se.Endpoints.Namespace)

		debugSpew(se)

		// create service
		svc, err := clientset.CoreV1().Services(se.Service.Namespace).Get(se.Service.Name, metav1.GetOptions{})
		if err != nil {
			debug("failed to get service '%s' (ns:%s) : %v", se.Service.Name, se.Service.Namespace, err)
			debug("create service and endpoints")
			_, err := clientset.CoreV1().Namespaces().Get(se.Service.Namespace, metav1.GetOptions{})
			if err != nil {
				debug("failed to get ns: %s : %v", se.Service.Namespace, err)
				var ns = new(v1.Namespace)
				ns.Name = se.Service.Namespace
				ns.Annotations = make(map[string]string)
				ns.Annotations[syncedResourceAnnotation] = syncedResourceAnnotationVal
				_, err = clientset.CoreV1().Namespaces().Create(ns)
				if err != nil {
					msg := fmt.Sprintf("failed to create namespace '%s' : %v", se.Service.Namespace, err)
					log.Printf(msg)
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, msg)
					return
				}
			}
			_, err = clientset.CoreV1().Services(se.Service.Namespace).Create(se.Service)
			if err != nil {
				msg := fmt.Sprintf("failed to create service '%s' (ns:%s) : %v", se.Service.Name, se.Service.Namespace, err)
				log.Printf(msg)
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, msg)
				return
			}
			_, err = clientset.CoreV1().Endpoints(se.Endpoints.Namespace).Create(se.Endpoints)
			if err != nil {
				msg := fmt.Sprintf("failed to create endpoints '%s' (ns:%s) : %v", se.Service.Name, se.Service.Namespace, err)
				log.Printf(msg)
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, msg)
				return
			}
			w.WriteHeader(http.StatusCreated)
		} else {
			a, ok := svc.Annotations[syncedResourceAnnotation]
			if ok && a == syncedResourceAnnotationVal {
				// update endpoints
				ep, err := clientset.CoreV1().Endpoints(se.Endpoints.Namespace).Get(se.Endpoints.Name, metav1.GetOptions{})
				if err != nil {
					msg := fmt.Sprintf("failed to get endpoints '%s' (ns:%s) : %v", se.Service.Name, se.Service.Namespace, err)
					log.Printf(msg)
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, msg)
					return
				}
				se.Endpoints.ObjectMeta.ResourceVersion = ep.ObjectMeta.ResourceVersion
				_, err = clientset.CoreV1().Endpoints(se.Endpoints.Namespace).Update(se.Endpoints)
				if err != nil {
					msg := fmt.Sprintf("failed to update endpoints '%s' (ns:%s) : %v", se.Service.Name, se.Service.Namespace, err)
					log.Printf(msg)
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, msg)
					return
				}
				w.WriteHeader(http.StatusCreated)
			} else {
				msg := fmt.Sprintf("skipping existing service '%s' (ns:%s)", se.Service.Name, se.Service.Namespace)
				log.Printf(msg)
				w.WriteHeader(http.StatusNotModified)
				io.WriteString(w, msg)
				return
			}
		}
	}
}

func startTargetListener() {
	http.HandleFunc(targetPath, targetHandler)
	webServer = &http.Server{Addr: listenAddress}
	debug("starting to listen on '%s'", listenAddress)
	err := webServer.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Printf("error starting web server or closing listener - %v\n", err)
	}
}

func syncServicesTarget() {
	// handle shutdown
	shutdown := make(chan os.Signal)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-shutdown
		log.Fatal(webServer.Shutdown(context.TODO()))
	}()

	startTargetListener()
}
