package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/golang/glog"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"google.golang.org/protobuf/encoding/prototext"

	configpb "github.com/metricrule-agent-go/api/proto/metricconfigpb"
	"github.com/metricrule-agent-go/pkg/mrmetric"
	"github.com/metricrule-agent-go/pkg/mrotel"
	"github.com/metricrule-agent-go/pkg/mrtransport"
)

// ApplicationHostKey is the address where the
// TF Serving application is hosted.
const ApplicationHostKey = "APPLICATION_HOST"

// ApplicationHostDefault is the default host for
// the application (127.0.0.1).
const ApplicationHostDefault = "127.0.0.1"

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

// Returns an env variable if it exists, else uses the provided fallback.
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func loadAgentConfig() *configpb.AgentConfig {
	configPath := getEnv(ConfigPathKey, "")
	if len(configPath) == 0 {
		return &configpb.AgentConfig{}
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		glog.Warningf("Error reading file at config path %v: %v", configPath, err)
		return &configpb.AgentConfig{}
	}

	var config configpb.AgentConfig
	err = prototext.Unmarshal(contents, &config)
	if err != nil {
		glog.Warningf("Error unmarshaling textproto: %v", err)
	}
	return &config
}

// Creates a reverse proxy listening at the specified port.
// Uses the provided meter for metrics.
// The host should be provided as a string, without a protocol,  e.g "127.0.0.1".
// The port should be provided as a string, without the ':', e.g "8080".
// Configures the provided aggregator.
func createReverseProxy(host string, port string, meter metric.Meter, aggregator mrotel.AggregatorProvider) *httputil.ReverseProxy {
	// parse the url
	url, _ := url.Parse("http://" + host + ":" + port)

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)
	config := loadAgentConfig()
	specs := mrmetric.GetInstrumentSpecs(config)
	inputSpecs := specs[mrmetric.InputContext]
	outputSpecs := specs[mrmetric.OutputContext]

	allInstr := make(map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper)
	inputInstr := make(map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper)
	for _, spec := range inputSpecs {
		inputInstr[spec] = mrotel.InitializeInstrument(meter, spec)
		allInstr[spec] = inputInstr[spec]
	}

	outputInstr := make(map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper)
	for _, spec := range outputSpecs {
		outputInstr[spec] = mrotel.InitializeInstrument(meter, spec)
		allInstr[spec] = outputInstr[spec]
	}

	aggregator.Update(allInstr, config)

	proxy.Transport = &mrtransport.Transport{
		RoundTripper:  http.DefaultTransport,
		AgentConfig: config,
		InInstrs:      inputInstr,
		OutInstrs:     outputInstr,
		Meter:         meter,
	}

	return proxy
}

func initOtel() (metric.Meter, *prometheus.Exporter, mrotel.AggregatorProvider) {
	agg := mrotel.NewAggregatorProvider()
	ctlr := controller.New(
		processor.New(
			agg,
			export.CumulativeExportKindSelector(),
			processor.WithMemory(true),
		))
	exp, err := prometheus.NewExporter(prometheus.Config{}, ctlr)
	if err != nil {
		log.Panicf("failed to initialize prometheus exporter %v", err)
	}
	global.SetMeterProvider(exp.MeterProvider())
	path := getEnv(MetricsPathKey, DefaultMetricsPath)
	http.Handle(path, exp)
	return global.Meter("metricrule.agent.proxy"), exp, agg
}

func serveReverseProxy(proxy *httputil.ReverseProxy, host string, port string, res http.ResponseWriter, req *http.Request) {
	// Update the headers to allow for SSL redirection
	url, _ := url.Parse("http://" + host + ":" + port)
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

	appHost := getEnv(ApplicationHostKey, ApplicationHostDefault)
	appPort := getEnv(ApplicationPortKey, ApplicationPortDefault)
	proxyPort := getEnv(ReverseProxyPortKey, ReverseProxyPortDefault)
	glog.Infof("Proxy server running on :%v will redirect to application on %v:%v", proxyPort, appHost, appPort)

	meter, exporter, aggregator := initOtel()
	proxy := createReverseProxy(appHost, appPort, meter, aggregator)
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		serveReverseProxy(proxy, appHost, appPort, res, req)
	})
	http.HandleFunc("/favicon.ico", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusNoContent)
	})
	if err := http.ListenAndServe(":"+proxyPort, nil); err != nil {
		glog.Error(err)
	}

	// When exiting from your process, call Stop for last collection cycle.
	defer func() {
		err := exporter.Controller().Stop(context.TODO())
		if err != nil {
			panic(err)
		}
	}()
}
