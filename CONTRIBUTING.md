# Contributing

Thank you for being interested in contributing!

Suggestions, bug reports, and code and doc contributions are very appreciated.

## Requirements

[Docker](https://www.docker.com/get-started) is needed for development.

Installations of [Go](https://golang.org/dl/), [protobuf-compiler](https://grpc.io/docs/protoc-installation/), and [protoc-gen-go](https://pkg.go.dev/github.com/golang/protobuf/protoc-gen-go) might be helpful for ease of development.

## Workflow

Please send a pull request for review with any contributions.

For code contributions, you are strongly encouraged to add unit tests.

Before sending for review, please run the following from the
repository root and verify that they all pass:

`make`

to verify a successful build.

`make lint`

to lint the source code.

`make test`

to run all unit tests.

### Verifying changes locally

This [Docker Compose template](example/docker-compose/docker-compose.yml) runs
the agent with a toy model and Prometheus-Grafana locally.

To use this for testing local modifications, build a Docker image locally with
some tag.

```bash
DOCKER_BUILDKIT=1 docker build . --target bin -t metricrule-tfserving-local-feature-xyz
```

Change the Docker Compose template's metricrule image to `metricrule-tfserving-local-feature-xyz`

Run with `docker-compose up`.
