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

// Transport is an HTTP transport that logs request and response metrics.
// Should be initialized by a backing RoundTripper (e.g http.DefaultTransport),
// a sidecar config, lists of input and output instruments, an opentelemetry
// meter.
type Transport struct {
	http.RoundTripper
	*configpb.SidecarConfig
	InInstrs  map[tfmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	OutInstrs map[tfmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper
	metric.Meter
}

// RoundTrip does the following in order:
// - Logs request metrics.
// - Uses the backing RoundTripper to send the request and get a response.
// - Logs response metrics.
func (t *Transport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	ctxLabels := []attribute.KeyValue{}
	if strings.Contains(req.Header.Get("Content-Type"), "json") {
		reqDump, err := httputil.DumpRequest(req, true)
		if err != nil {
			return nil, err
		}

		reqDumpS := string(reqDump)
		i := strings.Index(reqDumpS, "{")
		j := reqDumpS[i:]
		ctxLabels = tfmetric.GetContextLabels(t.SidecarConfig, j, tfmetric.InputContext)
		metrics := tfmetric.GetMetricInstances(t.SidecarConfig, j, tfmetric.InputContext)
		ctx := context.Background()
		for spec, m := range metrics {
			instr := t.InInstrs[spec]
			v, err := instr.Record(m.MetricValue)
			if err == nil {
				t.Meter.RecordBatch(ctx, append(ctxLabels, m.Labels...), v)
			}
		}
	}

	res, err = t.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if strings.Contains(res.Header.Get("Content-Type"), "json") {
		resDump, err := httputil.DumpResponse(res, true)
		if err != nil {
			return nil, err
		}

		resDumpS := string(resDump)
		j := resDumpS[strings.Index(resDumpS, "{"):]
		metrics := tfmetric.GetMetricInstances(t.SidecarConfig, j, tfmetric.OutputContext)
		ctx := context.Background()
		for spec, m := range metrics {
			instr := t.OutInstrs[spec]
			v, err := instr.Record(m.MetricValue)
			if err == nil {
				t.Meter.RecordBatch(ctx, append(ctxLabels, m.Labels...), v)
			}
		}
	}

	return res, nil
}
