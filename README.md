# GKE [multi-cluster] Service Sync

![CI](https://github.com/vihangvk/gke-service-sync/workflows/CI/badge.svg)

## Motivation

GKE supports VPC-Native Kubernetes clusters which use [Alias-IPs](https://cloud.google.com/vpc/docs/alias-ip) where Pod IPs are reachable in the VPC. But Kubernetes services in GKE clusters get IPs from address ranges which are only accessible inside the same GKE cluster.

This controller helps sync Kubernetes services across multiple GKE clusters to make it available in different clusters while being accessible via service name. Actual service runs only in original (or source) cluster and at destination cluster (or clusters) the service and endpoints are synced.

This is useful when there is no service mesh used and you need to migrate multiple workloads from one cluster to another but there is no defined order of deployment to make sure all dependant workloads are migrated first.

## Usage

This controller needs to run in all the GKE clusters you want to sync services.

The controller container image is available on [Docker Hub](https://hub.docker.com/r/vihangvk/gke-service-sync).

To deploy Kubernetes resources:

```bash
kubectl apply -f <checkout-directory>/k8s
```

### Configuration

Configuration for the controller is deployed as a ConfigMap which should be mounted at `/defaults/config.yaml`. There are configuration options:

```yaml
# the controller can run in three modes:
# 1. controller - where it only sends services and endpoints to target peers
# 2. target - where it only accepts services and endpoints and creates and updates those in current cluster
# 3. sync - where it does both of above. This is default.
runMode: sync
# debug enables verbose logging.
logLevel: debug
# list of peers to send Services and Endpoints
peers:
-  http://10.0.0.0:8080/
# list of Kubernetes Namespaces to check for Services to sync
syncNamespaces:
  - default
  - apps
# any services matching this regex will be skipped
skipServiceRegex: "kubernetes|nginx-ingress"
```

After deployment you need to update `peers` in source cluster to have Pod IP of the controller running in the destination cluster. By default controller listens on port `8080`, so the peer URL will be `http://<controller-pod-ip>:8080`
