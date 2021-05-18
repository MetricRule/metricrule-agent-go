# KFServing Example

This folder has Kubernetes configurations to run the KFServing agent with a KFServing model.

## Requirements

[kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) should be present for command-line commands against Kubernetes clusters.

[KFServing](https://github.com/kubeflow/kfserving) needs to be installed in the Kubernetes
cluster. This example was testing using the KFServing installed with Kubeflow.

> Tested with [AWS EKS](https://www.kubeflow.org/docs/distributions/aws/aws-e2e/).

## Usage

With Kubernetes configured to access a cluster, run

```bash
kubectl apply -f metricrule-agent.yaml
```

Run the following to get the URL to use as the logger sink

```bash
kubectl describe service metricrule-agent | grep "LoadBalancer Ingress:"
```

Update `http://dummy.agent.url` in `sklearn.yaml` to the loadbalancer ingress. Then run

```bash
kubectl apply -f sklearn.yaml
```

## End to end

Install Prometheus and Grafana in the cluster to visualize the metrics and build alerting.