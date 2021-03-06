package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/golang/glog"
	"go.opentelemetry.io/otel/attribute"
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
	"github.com/metricrule-agent-go/pkg/mrrecorder"
)

// AgentPortKey is the key for the port where metrics will be exposed.
const AgentPortKey = "AGENT_PORT"

// AgentPortDefault is the default port where metrics will be exposed.
const AgentPortDefault = "8551"

// MetricsPathKey is the key for the env variable to set the path that metrics will be served on.
const MetricsPathKey = "METRICS_PATH"

// DefaultMetricsPath is the default path where metrics will be made available.
const DefaultMetricsPath = "/metrics"

// ConfigPathKey is the key for the env variable to set the path for the config
// to create metrics.
const ConfigPathKey = "SIDECAR_CONFIG_PATH"

// KfServingRequestType is the cloud event type for a KFServing Request.
const KfServingRequestType = "org.kubeflow.serving.inference.request"

// KfServingResponseType is the cloud event type for a KFServing Response.
const KfServingResponseType = "org.kubeflow.serving.inference.response"

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
	return global.Meter("metricrule.agent.eventlistener"), exp, agg
}

type recordConfig struct {
	config    *configpb.AgentConfig
	inInstrs  map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	outInstrs map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
}

func getRecordConfig(meter metric.Meter, aggregator mrotel.AggregatorProvider) recordConfig {
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

	return recordConfig{config, inputInstr, outputInstr}
}

type recordArgs struct {
	event    cloudevents.Event
	config   recordConfig
	meter    metric.Meter
	ctxChans map[string](chan []attribute.KeyValue)
}

func record(args recordArgs) {
	if args.event.Type() == KfServingRequestType {
		reqData := mrrecorder.RequestLogData{
			Dump:   args.event.Data(),
			Config: args.config.config,
			Instrs: args.config.inInstrs,
			Meter:  args.meter,
		}
		ctxChan := make(chan []attribute.KeyValue, 1)
		args.ctxChans[args.event.ID()] = ctxChan
		go mrrecorder.LogRequestData(reqData, ctxChan)
	} else if args.event.Type() == KfServingResponseType {
		resData := mrrecorder.ResponseLogData{
			Dump:   args.event.Data(),
			Config: args.config.config,
			Instrs: args.config.outInstrs,
			Meter:  args.meter,
		}
		if reqChan, ok := args.ctxChans[args.event.ID()]; ok {
			go mrrecorder.LogResponseData(resData, reqChan)
		} else {
			dummyChan := make(chan []attribute.KeyValue, 1)
			dummyChan <- []attribute.KeyValue{}
			go mrrecorder.LogResponseData(resData, dummyChan)
		}
	}
}

func main() {
	// glog requires flag.Parse to be invoked before usage.
	// See https://github.com/golang/glog/commit/65d674618f712aa808a7d0104131b9206fc3d5ad.
	flag.Parse()
	glog.Info("Metricrule agent initialized")

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		log.Fatal("Failed to create client, ", err)
	}

	meter, exporter, aggregator := initOtel()
	config := getRecordConfig(meter, aggregator)
	ctxChans := make(map[string](chan []attribute.KeyValue))

	agentPort := getEnv(AgentPortKey, AgentPortDefault)
	path := getEnv(MetricsPathKey, DefaultMetricsPath)
	glog.Infof("Metrics will be served on :%v%v", agentPort, path)
	metricsMux := http.NewServeMux()
	metricsMux.Handle(path, exporter)

	go func() {
		if err := http.ListenAndServe(":"+agentPort, metricsMux); err != nil {
			glog.Error(err)
		}
	}()

	go func() {
		log.Fatal(c.StartReceiver(context.Background(), func(e cloudevents.Event) {
			record(recordArgs{e, config, meter, ctxChans})
		}))
	}()

	// When exiting from your process, call Stop for last collection cycle.
	defer func() {
		err := exporter.Controller().Stop(context.TODO())
		if err != nil {
			panic(err)
		}
	}()

	select {}
}
