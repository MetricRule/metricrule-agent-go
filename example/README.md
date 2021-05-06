# E2E Example

This folder has Docker Compose and Kubernetes configurations to run the TFServing agent together with a TFServing model, Prometheus and Grafana.

## Requirements

Docker Compose may already be present if you are using a desktop Docker installation. Else [see here](https://docs.docker.com/compose/install/).

[kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) should be present for command-line commands against Kubernetes clusters.

> Kubernetes configs have been tested against [GKE](https://cloud.google.com/kubernetes-engine) only.

## Docker Compose

From the `docker-compose` subdirectory,

```bash
docker compose up
```

This serves a HTTP API on `localhost:8551` by default, with a Grafana instance exposed on `localhost:3000`.

## Kubernetes

With `kubectl` configured to access a cluster, run

```bash
kubectl apply -f petfinder_k8s.yaml
kubectl apply -f monitoring_k8s.yaml --namespace=monitoring
```