module github.com/metricrule-sidecar-tfserving

go 1.16

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	go.opentelemetry.io/otel v0.19.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.19.0
	go.opentelemetry.io/otel/metric v0.19.0
	google.golang.org/genproto v0.0.0-20210406143921-e86de6bf7a46 // indirect
	google.golang.org/protobuf v1.26.0
)
