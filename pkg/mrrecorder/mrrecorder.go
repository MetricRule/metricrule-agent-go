package mrrecorder

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
	"github.com/metricrule-sidecar-tfserving/pkg/mrmetric"
	"github.com/metricrule-sidecar-tfserving/pkg/mrotel"
)

type MetricRecorder interface {
	RecordBatch(
		ctx context.Context,
		labels []attribute.KeyValue,
		measurement ...metric.Measurement,
	)
}

// RequestLogData contains the request data and relevant configuration
// for logging.
type RequestLogData struct {
	Dump   []byte
	Config *configpb.SidecarConfig
	Instrs map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	Meter  MetricRecorder
}

// LogRequestData logs metrics based on the input metrics,
// and communicates context labels to the parameter channel.
func LogRequestData(d RequestLogData, ctxChan chan<- []attribute.KeyValue) {
	defer close(ctxChan)

	strdump := string(d.Dump)
	i := strings.Index(strdump, "{")
	j := strdump[i:]
	ctxLabels := mrmetric.GetContextLabels(d.Config, j, mrmetric.InputContext)
	metrics := mrmetric.GetMetricInstances(d.Config, j, mrmetric.InputContext)
	ctx := context.Background()
	for spec, m := range metrics {
		instr := d.Instrs[spec]
		v, err := instr.Record(m.MetricValue)
		if err == nil {
			d.Meter.RecordBatch(ctx, append(ctxLabels, m.Labels...), v)
		}
	}
	ctxChan <- ctxLabels
}

// ResponseLogData contains response data and relevant configuration
// for logging.
type ResponseLogData struct {
	Dump   []byte
	Config *configpb.SidecarConfig
	Instrs map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	Meter  MetricRecorder
}

// LogResponseData logs metrics based on output data and labels in context.
func LogResponseData(d ResponseLogData, ctxChan <-chan []attribute.KeyValue) {
	ctxLabels := <-ctxChan
	strdump := string(d.Dump)
	j := strdump[strings.Index(strdump, "{"):]
	metrics := mrmetric.GetMetricInstances(d.Config, j, mrmetric.OutputContext)
	ctx := context.Background()
	for spec, m := range metrics {
		instr := d.Instrs[spec]
		v, err := instr.Record(m.MetricValue)
		if err == nil {
			d.Meter.RecordBatch(ctx, append(ctxLabels, m.Labels...), v)
		}
	}
}
