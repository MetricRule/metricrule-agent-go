package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/golang/glog"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"google.golang.org/protobuf/encoding/prototext"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
)

// MetricsPathKey is the key for the env variable to set the path that metrics will be served on.
const MetricsPathKey = "METRICS_PATH"

// DefaultMetricsPath is the default path where metrics will be made available.
const DefaultMetricsPath = "/metrics"

// ConfigPathKey is the key for the env variable to set the path for the config
// to create metrics.
const ConfigPathKey = "SIDECAR_CONFIG_PATH"

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func loadSidecarConfig() *configpb.SidecarConfig {
	configPath := getEnv(ConfigPathKey, "")
	if len(configPath) == 0 {
		return &configpb.SidecarConfig{}
	}
	contents, err := os.ReadFile(configPath)

	if err != nil {
		glog.Warningf("Error reading file at config path %v: %v", configPath, err)
		return &configpb.SidecarConfig{}
	}

	var config configpb.SidecarConfig
	err = prototext.Unmarshal(contents, &config)
	if err != nil {
		glog.Warningf("Error unmarshaling textproto: %v", err)
	}
	return &config
}

func initOtel() (metric.Meter, *prometheus.Exporter) {
	exporter, err := prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		log.Panicf("failed to initialize prometheus exporter %v", err)
	}
	path := getEnv(MetricsPathKey, DefaultMetricsPath)
	http.Handle(path, exporter)
	return global.Meter("metricrule.sidecar.tfserving"), exporter
}

func record(event cloudevents.Event, meter metric.Meter) {
	log.Printf("☁️  cloudevents.Event\n%s", event.String())
}

func main() {
	// glog requires flag.Parse to be invoked before usage.
	// See https://github.com/golang/glog/commit/65d674618f712aa808a7d0104131b9206fc3d5ad.
	flag.Parse()

	meter, exporter := initOtel()
	c, err := cloudevents.NewDefaultClient()
	if err != nil {
		log.Fatal("Failed to create client, ", err)
	}
	c.StartReceiver(context.Background(), func(e cloudevents.Event) {
		record(e, meter)
	})

	// When exiting from your process, call Stop for last collection cycle.
	defer func() {
		err := exporter.Controller().Stop(context.TODO())
		if err != nil {
			panic(err)
		}
	}()
}
