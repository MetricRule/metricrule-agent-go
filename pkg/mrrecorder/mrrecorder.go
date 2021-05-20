package mrrecorder

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/golang/glog"
	configpb "github.com/metricrule-agent-go/api/proto/metricconfigpb"
	"github.com/metricrule-agent-go/pkg/mrmetric"
	"github.com/metricrule-agent-go/pkg/mrotel"
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
		vs := []metric.Measurement{}
		for _, val := range m.MetricValues {
			v, err := instr.Record(val)
			if err != nil {
				glog.Errorf("Error recording metric for spec %v:\n%v", spec.Name, err)
			} else {
				vs = append(vs, v)
			}
		}
		d.Meter.RecordBatch(ctx, append(ctxLabels, m.Labels...), vs...)
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
		vs := []metric.Measurement{}
		for _, val := range m.MetricValues {
			v, err := instr.Record(val)
			if err != nil {
				glog.Errorf("Error recording metric for spec %v:\n%v", spec.Name, err)
			} else {
				vs = append(vs, v)
			}
		}
		d.Meter.RecordBatch(ctx, append(ctxLabels, m.Labels...), vs...)
	}
}
