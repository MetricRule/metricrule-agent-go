# 📏 MetricRule

Easy open source monitoring for ML models.

> A sidecar agent that creates metrics for monitoring deployed machine learning models.

[![Continuous Integration](https://github.com/MetricRule/metricrule-agent-go/actions/workflows/ci.yaml/badge.svg)](https://github.com/MetricRule/metricrule-agent-go/actions/workflows/ci.yaml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

----

## Motivation

MetricRule agents are designed to be deployed with a serving model endpoint to generate input feature and output distribution metrics based on the model's endpoint usage. Integrations with
[Tensorflow Serving](https://github.com/tensorflow/serving) and [KFServing](https://github.com/kubeflow/kfserving) are supported.

The motivation of this project is to make it easier to monitor feature distributions in production to better catch real world ML issues like training-serving skew, feature drifts, poor model performance on specific slices of input.

### TFServing

The recommended usage with TFServing is to deploy the agent as a sidecar with the model.

The executable used is [cmd/proxy](cmd/proxy). The latest release is available at [Docker Hub](https://hub.docker.com/repository/docker/metricrule/metricrule-sidecar-tfserving/).

### KFServing

The recommended usage with KFServing is to use the agent as a logger sink.

The executable used is [cmd/eventlistener](cmd/eventlistener). The latest release is available at
[Docker Hub](https://hub.docker.com/repository/docker/metricrule/metricrule-kfserving/).

## Screenshots and Demo

Taken with a demo Grafana instance of this agent running with a toy [PetFinder](https://www.tensorflow.org/datasets/catalog/pet_finder) model with simulated traffic.

> The model is built using [this Tensorflow tutorial](https://www.tensorflow.org/tutorials/structured_data/feature_columns)

### Input Feature Distributions

![Count of Input Pets By Gender](<https://metricrule.github.io/img/inputcount.png>)

### Output Distributions

![Distribution of Output Predictions](<https://metricrule.github.io/img/outputlogits.png>)

### Output Distribution by Input Slice

![Distribution of Outputs by Input Slice](<https://metricrule.github.io/img/outputsbyinput.png>)

## Get Started

Images of the agent are maintained on [Docker Hub](https://hub.docker.com/repository/docker/metricrule/).

The agent takes as input a configuration to define what metrics to create based on input and output JSONs. Based on this, the agent provides a HTTP endpoint (by default `/metrics`) with metric aggregates.

The expected usage is to use [Prometheus](https://github.com/prometheus/prometheus) to periodically scrape these endpoints. This can be then used as a data source for a [Grafana](https://github.com/grafana/grafana) dashboard for visualizations and alerts.

End-to-end examples with [Kubernetes](https://kubernetes.io/) and [Docker Compose](https://docs.docker.com/compose/) are provided in the [example/](example/tfserving) subdirectory.

## Configuration

### Metric Definition

The configuration allows specifying metrics based on the JSON input features and the output predictions.

Additionally, input features can be parsed to create `labels` that are tagged to input and output metric instances.

The config format is defined at [api/proto/metricrule_metric_configuration.proto](api/proto/metricrule_metric_configuration.proto).

See an example configuration used for the demo at [configs/example_sidecar_config.textproto](configs/example_sidecar_config.textproto).

The configuration is supplied as a config file, with location defined by the `SIDECAR_CONFIG_PATH` environment variable.

### Other Config

Configuration is through environment variables. Some options of interest are:

- Set `REVERSE_PROXY_PORT` to the port the agent is running on, e.g `"9551"`
- Set `APPLICATION_HOST` to the host the serving endpoint is running on, defaults to `"127.0.0.1"`
- Set `APPLICATION_PORT` to the port the serving endpoint is running on, e.g `"8501"`
- Set `METRICS_PATH` to the path where a HTTP endpoint for metrics scraping should be exposed at, defaults to `"/metrics"`

## Contribute

We ❤️ contributions. Please see [CONTRIBUTING.md](CONTRIBUTING.md).

Please feel free to use the [Issue Tracker](https://github.com/MetricRule/metricrule-agent-go/issues) and [GitHub Discussions](https://github.com/MetricRule/metricrule-agent-go/discussions) to reach out to the maintainers.

## For more information

Please refer to [metricrule.com](https://metricrule.com) for more information.
