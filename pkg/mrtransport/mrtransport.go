package mrtransport

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
	"github.com/metricrule-sidecar-tfserving/pkg/mrotel"
	"github.com/metricrule-sidecar-tfserving/pkg/tfmetric"
)

// MetricRecorder is an interface that is able to record a measurement with labels.
// The expected instance of this is metric.Meter.
type MetricRecorder interface {
	RecordBatch(ctx context.Context, labels []attribute.KeyValue, measurement ...metric.Measurement)
}

// Transport is an HTTP transport that logs request and response metrics.
// Should be initialized by a backing RoundTripper (e.g http.DefaultTransport),
// a sidecar config, lists of input and output instruments, an opentelemetry
// meter (or an equivalent recorder).
type Transport struct {
	http.RoundTripper
	*configpb.SidecarConfig
	InInstrs  map[tfmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	OutInstrs map[tfmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	Meter     MetricRecorder
}

// RoundTrip does the following in order:
// - Logs request metrics.
// - Uses the backing RoundTripper to send the request and get a response.
// - Logs response metrics.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctxChan := make(chan []attribute.KeyValue, 1)
	if strings.Contains(req.Header.Get("Content-Type"), "json") {
		dump, err := httputil.DumpRequest(req, true)
		if err != nil {
			return nil, err;
		}

		reqdata := requestLogData{dump, t.SidecarConfig, t.InInstrs, t.Meter}
		go logRequestData(reqdata, ctxChan)
	} else {
		ctxChan <- []attribute.KeyValue{}
	}

	res, err := t.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if strings.Contains(res.Header.Get("Content-Type"), "json") {
		dump, err := httputil.DumpResponse(res, true);
		if err != nil {
			return res, err
		}

		resdata := responseLogData{dump, t.SidecarConfig, t.OutInstrs, t.Meter}
		go logResponseData(resdata, ctxChan)
	}

	return res, nil
}

type requestLogData struct {
	dump []byte
	config *configpb.SidecarConfig
	instrs map[tfmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	m MetricRecorder
}

// logRequestData logs metrics based on the input metrics,
// and communicates context labels to the parameter channel.
func logRequestData(d requestLogData, ctxChan chan<- []attribute.KeyValue) {
	defer close(ctxChan)
	
	strdump := string(d.dump)
	i := strings.Index(strdump, "{")
	j := strdump[i:]
	ctxLabels := tfmetric.GetContextLabels(d.config, j, tfmetric.InputContext)
	metrics := tfmetric.GetMetricInstances(d.config, j, tfmetric.InputContext)
	ctx := context.Background()
	for spec, m := range metrics {
		instr := d.instrs[spec]
		v, err := instr.Record(m.MetricValue)
		if err == nil {
			d.m.RecordBatch(ctx, append(ctxLabels, m.Labels...), v)
		}
	}
	ctxChan <- ctxLabels
}

type responseLogData struct {
	dump []byte
	config *configpb.SidecarConfig
	instrs map[tfmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	m MetricRecorder
}

// logResponse logs metrics based on output data and labels in context.
func logResponseData(d responseLogData, ctxChan <-chan []attribute.KeyValue) {
	ctxLabels := <-ctxChan
	strdump := string(d.dump)
	j := strdump[strings.Index(strdump, "{"):]
	metrics := tfmetric.GetMetricInstances(d.config, j, tfmetric.OutputContext)
	ctx := context.Background()
	for spec, m := range metrics {
		instr := d.instrs[spec]
		v, err := instr.Record(m.MetricValue)
		if err == nil {
			d.m.RecordBatch(ctx, append(ctxLabels, m.Labels...), v)
		}
	}
}