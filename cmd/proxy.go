package main

import (
	"context"
	"errors"
	"flag"
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
		glog.Errorf("Unexpected value %v in int 64 counter", value)
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
		glog.Errorf("Unexpected value %v in int 64 value recorder", value)
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
		glog.Errorf("Unexpected value %v in float 64 value recorder", value)
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
	// read from file.
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

// Creates a reverse proxy listening at the specified port.
// Uses the provided meter for metrics.
// The port should be provided as a string, without the ':', e.g "8080".
func createReverseProxy(port string, meter metric.Meter) *httputil.ReverseProxy {
	// parse the url
	url, _ := url.Parse("http://127.0.0.1:" + port)

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)
	config := loadSidecarConfig()
	specs := tfmetric.GetInstrumentSpecs(config)
	inputSpecs := specs[tfmetric.InputContext]
	outputSpecs := specs[tfmetric.OutputContext]

	inputInstr := make(map[tfmetric.MetricInstrumentSpec]instrumentWrapper)
	for _, spec := range inputSpecs {
		inputInstr[spec] = initializeInstrument(meter, spec)
	}

	outputInstr := make(map[tfmetric.MetricInstrumentSpec]instrumentWrapper)
	for _, spec := range outputSpecs {
		outputInstr[spec] = initializeInstrument(meter, spec)
	}

	proxy.ModifyResponse = func(r *http.Response) error {
		dump, err := httputil.DumpResponse(r, true)
		if err != nil {
			log.Fatal(err)
		}
		res := string(dump)
		j := res[strings.Index(res, "{"):]
		metrics := tfmetric.GetMetricInstances(config, j, tfmetric.OutputContext)
		ctx := context.Background()
		for spec, m := range metrics {
			instr := outputInstr[spec]
			v, err := instr.record(m.MetricValue)
			if err != nil {
				meter.RecordBatch(ctx, m.Labels, v)
			}
		}
		return nil
	}

	singleHostDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		dump, _ := httputil.DumpRequest(r, true)
		req := string(dump)
		i := strings.Index(req, "{")
		if i < 0 {
			log.Printf("No request body found: %s", r.RequestURI)
			singleHostDirector(r)
			return
		}
		j := req[i:]
		metrics := tfmetric.GetMetricInstances(config, j, tfmetric.InputContext)
		ctx := context.Background()
		for spec, m := range metrics {
			instr := inputInstr[spec]
			v, err := instr.record(m.MetricValue)
			if err != nil {
				meter.RecordBatch(ctx, m.Labels, v)
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
	glog.Errorf("No instrument could be created for spec %v", spec.Name)
	return noOpWrapper{}
}

func initOtel() metric.Meter {
	exporter, err := prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		log.Panicf("failed to initialize prometheus exporter %v", err)
	}
	path := getEnv(MetricsPathKey, DefaultMetricsPath)
	http.HandleFunc(path, exporter.ServeHTTP)
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
	// glog requires flag.Parse to be invoked before usage.
	// See https://github.com/golang/glog/commit/65d674618f712aa808a7d0104131b9206fc3d5ad.
	flag.Parse()

	appPort := getEnv(ApplicationPortKey, ApplicationPortDefault)
	proxyPort := getEnv(ReverseProxyPortKey, ReverseProxyPortDefault)
	glog.Infof("Proxy server running on :%v will redirect to application on :%v", proxyPort, appPort)

	meter := initOtel()
	proxy := createReverseProxy(appPort, meter)
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		serveReverseProxy(proxy, appPort, res, req)
	})
	http.HandleFunc("/favicon.ico", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusNoContent)
	})
	if err := http.ListenAndServe(":"+proxyPort, nil); err != nil {
		glog.Error(err)
	}
}
