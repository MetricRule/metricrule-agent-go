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
	"github.com/metricrule-sidecar-tfserving/pkg/mrrecorder"
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
			return nil, err
		}

		reqdata := mrrecorder.RequestLogData{
			Dump:   dump,
			Config: t.SidecarConfig,
			Instrs: t.InInstrs,
			Meter:  t.Meter,
		}
		go mrrecorder.LogRequestData(reqdata, ctxChan)
	} else {
		ctxChan <- []attribute.KeyValue{}
	}

	res, err := t.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if strings.Contains(res.Header.Get("Content-Type"), "json") {
		dump, err := httputil.DumpResponse(res, true)
		if err != nil {
			return res, err
		}

		resdata := mrrecorder.ResponseLogData{
			Dump:   dump,
			Config: t.SidecarConfig,
			Instrs: t.OutInstrs,
			Meter:  t.Meter,
		}
		go mrrecorder.LogResponseData(resdata, ctxChan)
	}

	return res, nil
}
