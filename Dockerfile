# syntax=docker/dockerfile:1.2

FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.16.3-buster AS base

WORKDIR /src

# Install protobuf compilers
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive \
    apt-get install --no-install-recommends --assume-yes \
      protobuf-compiler
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Install dependencies
ENV CGO_ENABLED=0
COPY go.* .
RUN go mod download
COPY . .

RUN protoc -I=api/proto --go_out=api/proto \
      --go_opt=module=github.com/metricrule-agent-go/api/proto \
      api/proto/metricrule_metric_configuration.proto

FROM base AS build
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
  GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/proxy cmd/proxy/main.go
RUN --mount=type=cache,target=/root/.cache/go-build \
  GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o \
  /out/eventlistener cmd/eventlistener/main.go

FROM base AS unit-test
RUN --mount=type=cache,target=/root/.cache/go-build \
go test -v ./...

FROM golangci/golangci-lint:v1.27-alpine AS lint-base

FROM base AS lint
COPY --from=lint-base /usr/bin/golangci-lint /usr/bin/golangci-lint
RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/root/.cache/golangci-lint \
  golangci-lint run --timeout 10m0s ./...

FROM scratch AS bin-unix
COPY --from=build /out/proxy /
COPY --from=build /out/eventlistener /

FROM bin-unix AS bin-linux
FROM bin-unix AS bin-darwin

FROM scratch AS bin-windows
COPY --from=build /out/proxy /proxy.exe
COPY --from=build /out/eventlistener /eventlistener.exe

FROM bin-${TARGETOS} AS bin
