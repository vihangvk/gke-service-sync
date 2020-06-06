package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type serviceEndpoints struct {
	Service   *v1.Service   `json:"service"`
	Endpoints *v1.Endpoints `json:"endpoints"`
}

func (se *serviceEndpoints) sanitize() {
	// cleanup
	se.Service.SelfLink = ""
	se.Service.Status = v1.ServiceStatus{}
	se.Service.UID = ""
	se.Service.ResourceVersion = ""
	se.Service.CreationTimestamp = metav1.Time{}
	se.Service.Spec.ClusterIP = ""
	se.Service.Spec.Selector = nil

	se.Endpoints.SelfLink = ""
	se.Endpoints.UID = ""
	se.Endpoints.ResourceVersion = ""
	se.Endpoints.CreationTimestamp = metav1.Time{}

	for sb := range se.Endpoints.Subsets {
		for ad := range se.Endpoints.Subsets[sb].Addresses {
			se.Endpoints.Subsets[sb].Addresses[ad].NodeName = nil
			se.Endpoints.Subsets[sb].Addresses[ad].TargetRef = nil
		}
		se.Endpoints.Subsets[sb].NotReadyAddresses = nil
	}

	delete(se.Service.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	delete(se.Endpoints.Annotations, "endpoints.kubernetes.io/last-change-trigger-time")
}

func (se *serviceEndpoints) MarshalJSON() ([]byte, error) {
	setemp := *se
	setemp.sanitize()

	return json.Marshal(setemp)
}

func watchServicesAndEndpoints(svcChan chan *serviceEndpoints) {
	var stop bool
	var stopChan = make(chan bool)

	// handle shutdown
	shutdown := make(chan os.Signal)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-shutdown
		stop = true
		stopChan <- stop
	}()

	go func() {
	outer:
		for !stop {
			debug("create endpoints watcher")
			epslices, err := clientset.DiscoveryV1beta1().EndpointSlices("").List(metav1.ListOptions{})
			if epslices != nil && len(epslices.Items) > 0 {
				log.Fatalf("Detected Endpoint slices but this tool doesn't support it.")
			}
			epWatcher, err := clientset.CoreV1().Endpoints("").Watch(metav1.ListOptions{})
			if err != nil {
				log.Printf("error getting watcher for endpoints: %v", err)
				continue
			}

			for {
				var (
					ep *v1.Endpoints
					s  *v1.Service
					ok bool
				)
				select {
				case event := <-epWatcher.ResultChan():
					if stop {
						debug("stopping watcher")
						epWatcher.Stop()
						break outer
					}

					if event.Type == watch.Added || event.Type == watch.Modified {
						debug("received event: %v", event.Type)
						ep, ok = event.Object.(*v1.Endpoints)
						if !ok {
							log.Printf("incorrect type found, expected Endpoints: %T", event.Object)
							continue
						}
						if i := defaultConfig.SyncNamespaces.Search(ep.Namespace); i == len(defaultConfig.SyncNamespaces) ||
							defaultConfig.SyncNamespaces[i] != ep.Namespace {
							debug("skipping namespace '%s' for endpoint '%s' as its not in %v", ep.Namespace, ep.Name, defaultConfig.SyncNamespaces)
							continue
						}
						regex := regexp.MustCompile(defaultConfig.SkipServicesRegex)
						if regex.MatchString(ep.Name) {
							debug("skipping service '%s' (ns: %s)", ep.Name, ep.Namespace)
							continue
						}
						s, err = clientset.CoreV1().Services(ep.Namespace).Get(ep.Name, metav1.GetOptions{})
						if err != nil {
							log.Printf("failed to get service '%s' (ns:%s)", ep.Name, ep.Namespace)
							continue
						}
						debug("Service: '%s' (ns:%s)", s.Name, s.Namespace)
						a, ok := s.Annotations[syncedResourceAnnotation]
						if ok && a == syncedResourceAnnotationVal {
							debug("skipping annotated service '%s' (ns:%s)", s.Name, s.Namespace)
							continue
						}
						if s.Spec.Type == v1.ServiceTypeLoadBalancer {
							log.Printf("WARNNING: Serice of type LoadBalancer detected: '%s' (ns: %s)", s.Name, s.Namespace)
						}
						se := new(serviceEndpoints)
						se.Service = s
						se.Endpoints = ep
						debugSpew(se)

						svcChan <- se
					} else {
						debug("ignoring event %v: %v", event.Type, event.Object)
						if event.Object == nil {
							epWatcher.Stop()
							continue outer
						}
						continue
					}
				case <-stopChan:
					debug("received stop channel")
					epWatcher.Stop()
					break outer
				}
			}
		}
		debug("stop watching services")
		svcChan <- nil
	}()
}

func syncServicesController() {
	var svcChan = make(chan *serviceEndpoints)
	watchServicesAndEndpoints(svcChan)

	for {
		select {
		case se := <-svcChan:
			if se == nil {
				debug("recieved no service, return")
				return
			}
			debugSpew(se)
			js, err := json.Marshal(se)
			if err != nil {
				log.Fatalf("failed to marshal service and endpoints into JSON: %v", err)
			}
			debug(string(js))
			for _, peer := range defaultConfig.Peers {
				url := strings.TrimSuffix(peer, "/") + targetPath
				debug("sending services to '%s'", peer)
				go func() {
					resp, err := http.Post(url, "", bytes.NewBuffer(js))
					if err != nil {
						log.Printf("failed to post to target '%s': %v", url, err)
					}
					msg, _ := ioutil.ReadAll(resp.Body)
					debug("(url: %s) response status: '%s', message: %s", url, resp.Status, msg)
				}()
			}
		}
	}
}
