package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/golang/glog"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"google.golang.org/protobuf/encoding/prototext"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
	"github.com/metricrule-sidecar-tfserving/pkg/tfmetric"
)

// ApplicationPortKey is the key for the env variable for the port of
// the main TF Serving application.
const ApplicationPortKey = "APPLICATION_PORT"

// ReverseProxyPortKey is the key for the env variable for the port
// this sidecar will run on.
const ReverseProxyPortKey = "REVERSE_PROXY_PORT"

// ApplicationPortDefault is the default port for the HTTP server of TF serving.
const ApplicationPortDefault = "8501"

// ReverseProxyPortDefault is the default port for the sidecar to run a reverse proxy on.
const ReverseProxyPortDefault = "8551"

// MetricsPathKey is the key for the env variable to set the path that metrics will be served on.
const MetricsPathKey = "METRICS_PATH"

// DefaultMetricsPath is the default path where metrics will be made available.
const DefaultMetricsPath = "/metrics"

// ConfigPathKey is the key for the env variable to set the path for the config
// to create metrics.
const ConfigPathKey = "SIDECAR_CONFIG_PATH"

type instrumentWrapper interface {
	record(value interface{}) (metric.Measurement, error)
}

type int64CounterWrapper struct {
	c metric.Int64Counter
}

func (c int64CounterWrapper) record(value interface{}) (metric.Measurement, error) {
	number, ok := value.(int64)
	if !ok {
		glog.Error("Unexpected value %v in int 64 counter", value)
		return c.c.Measurement(0), errors.New("Unexpected value")
	}
	return c.c.Measurement(number), nil
}

type int64ValueRecorderWrapper struct {
	c metric.Int64ValueRecorder
}

func (c int64ValueRecorderWrapper) record(value interface{}) (metric.Measurement, error) {
	number, ok := value.(int64)
	if !ok {
		glog.Error("Unexpected value %v in int 64 value recorder", value)
		return c.c.Measurement(0), errors.New("Unexpected value")
	}
	return c.c.Measurement(number), nil
}

type float64ValueRecorderWrapper struct {
	c metric.Float64ValueRecorder
}

func (c float64ValueRecorderWrapper) record(value interface{}) (metric.Measurement, error) {
	number, ok := value.(float64)
	if !ok {
		glog.Error("Unexpected value %v in float 64 value recorder", value)
		return c.c.Measurement(0), errors.New("Unexpected value")
	}
	return c.c.Measurement(number), nil
}

type noOpWrapper struct{}

func (c noOpWrapper) record(value interface{}) (metric.Measurement, error) {
	return metric.Measurement{}, errors.New("No op wrapper used")
}

// Returns an env variable if it exists, else uses the provided fallback.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func loadSidecarConfig() *configpb.SidecarConfig {
	configPath := getEnv(ConfigPathKey, "")
	if len(configPath) == 0 {
		return &configpb.SidecarConfig{}
	}
	// read from file.
	contents, err := os.ReadFile(configPath)
	if err != nil {
		glog.Warning("Error reading file at config path %v: %v", configPath, err)
		return &configpb.SidecarConfig{}
	}
	var config configpb.SidecarConfig
	prototext.Unmarshal(contents, &config)
	return &config
}

// Creates a reverse proxy listening at the specified port.
// Uses the provided meter for metrics.
// The port should be provided as a string, without the ':', e.g "8080".
func createReverseProxy(port string, meter metric.Meter) *httputil.ReverseProxy {
	// parse the url
	url, _ := url.Parse("http://127.0.0.1:" + port)

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)
	config := loadSidecarConfig()
	instrumentSpecs := tfmetric.GetInstrumentSpecs(config)
	inputInstrumentSpecs := instrumentSpecs[tfmetric.InputContext]
	outputInstrumentSpecs := instrumentSpecs[tfmetric.OutputContext]

	inputInstruments := make(map[tfmetric.MetricInstrumentSpec]instrumentWrapper)
	for _, spec := range inputInstrumentSpecs {
		inputInstruments[spec] = initializeInstrument(meter, spec)
	}

	outputInstruments := make(map[tfmetric.MetricInstrumentSpec]instrumentWrapper)
	for _, spec := range outputInstrumentSpecs {
		outputInstruments[spec] = initializeInstrument(meter, spec)
	}

	proxy.ModifyResponse = func(r *http.Response) error {
		dump, err := httputil.DumpResponse(r, true)
		if err != nil {
			log.Fatal(err)
		}
		res := string(dump)
		jsonBlob := res[strings.Index(res, "{"):]
		metricsToRecord := tfmetric.GetMetricInstances(config, jsonBlob, tfmetric.OutputContext)
		ctx := context.Background()
		for spec, instance := range metricsToRecord {
			instrument := outputInstruments[spec]
			m, err := instrument.record(instance.MetricValue)
			if err != nil {
				meter.RecordBatch(ctx, instance.Labels, m)
			}
		}
		return nil
	}

	singleHostDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		dump, _ := httputil.DumpRequest(r, true)
		req := string(dump)
		index := strings.Index(req, "{")
		if index < 0 {
			log.Printf("No request body found: %s", r.RequestURI)
			singleHostDirector(r)
			return
		}
		jsonBlob := req[index:]
		metricsToRecord := tfmetric.GetMetricInstances(config, jsonBlob, tfmetric.InputContext)
		ctx := context.Background()
		for spec, instance := range metricsToRecord {
			instrument := inputInstruments[spec]
			m, err := instrument.record(instance.MetricValue)
			if err != nil {
				meter.RecordBatch(ctx, instance.Labels, m)
			}
		}
		singleHostDirector(r)
	}

	return proxy
}

func initializeInstrument(meter metric.Meter, spec tfmetric.MetricInstrumentSpec) instrumentWrapper {
	switch spec.InstrumentKind {
	case metric.CounterInstrumentKind:
		switch spec.MetricValueKind {
		case reflect.Int64:
			c, err := meter.NewInt64Counter(spec.Name)
			if err != nil {
				return int64CounterWrapper{c}
			}
		}
	case metric.ValueRecorderInstrumentKind:
		switch spec.MetricValueKind {
		case reflect.Int64:
			c, err := meter.NewInt64ValueRecorder(spec.Name)
			if err != nil {
				return int64ValueRecorderWrapper{c}
			}
		case reflect.Float64:
			c, err := meter.NewFloat64ValueRecorder(spec.Name)
			if err != nil {
				return float64ValueRecorderWrapper{c}
			}
		}
	}
	glog.Error("No instrument could be created for spec %v", spec.Name)
	return noOpWrapper{}
}

func initOtel() metric.Meter {
	exporter, err := prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		log.Panicf("failed to initialize prometheus exporter %v", err)
	}
	metricsPath := getEnv(MetricsPathKey, DefaultMetricsPath)
	http.HandleFunc(metricsPath, exporter.ServeHTTP)
	return global.Meter("metricrule.sidecar.tfserving")
}

func serveReverseProxy(proxy *httputil.ReverseProxy, port string, res http.ResponseWriter, req *http.Request) {
	// Update the headers to allow for SSL redirection
	url, _ := url.Parse("http://127.0.0.1:" + port)
	req.URL.Host = url.Host
	req.URL.Scheme = url.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = url.Host
	// Note that ServeHttp is non-blocking and uses a goroutine under the hood.
	proxy.ServeHTTP(res, req)
}

func main() {
	applicationPort := getEnv(ApplicationPortKey, ApplicationPortDefault)
	reverseProxyPort := getEnv(ReverseProxyPortKey, ReverseProxyPortDefault)
	// TODO(jishnu): Investigate error for log before flag parse.
	// glog.Info("Proxy server running on :%v will redirect to application on :%v", reverseProxyPort, applicationPort)

	meter := initOtel()
	proxy := createReverseProxy(applicationPort, meter)
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		serveReverseProxy(proxy, applicationPort, res, req)
	})
	http.HandleFunc("/favicon.ico", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusNoContent)
	})
	if err := http.ListenAndServe(":"+reverseProxyPort, nil); err != nil {
		glog.Error(err)
	}
}
