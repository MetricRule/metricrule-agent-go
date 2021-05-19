package mrtransport

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/protobuf/encoding/prototext"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
	"github.com/metricrule-sidecar-tfserving/pkg/mrmetric"
	"github.com/metricrule-sidecar-tfserving/pkg/mrotel"
)

type meterRecordCall struct {
	labels []attribute.KeyValue
	// Only labels are checked as measurements are
	// checked in instrument wrapper.
}

type fakeMeterImpl struct {
	expectedCalls []meterRecordCall
}

type instrumentRecordCall struct {
	value interface{}
}

type fakeInstrWrapper struct {
	expectedCalls []instrumentRecordCall
}

func (f fakeInstrWrapper) Record(value interface{}) (metric.Measurement, error) {
	if len(f.expectedCalls) == 0 {
		log.Panicf("Instrument: Unexpected call with %v", value)
	}
	exp := f.expectedCalls[0]
	f.expectedCalls = f.expectedCalls[1:]

	if value != exp.value {
		log.Panicf("Instrument: Unexpected call, got %v want %v", value, exp.value)
	}

	return metric.Measurement{}, nil
}

func (f fakeMeterImpl) RecordBatch(ctx context.Context, labels []attribute.KeyValue, measurement ...metric.Measurement) {
	if len(f.expectedCalls) == 0 {
		log.Panic("Meter: Unexpected call to RecordBatch")
	}
	exp := f.expectedCalls[0]
	f.expectedCalls = f.expectedCalls[1:]

	if len(labels) != len(exp.labels) {
		log.Panicf("Meter: Unexpected count of labels, got %v want %v", len(labels), len(exp.labels))
	}

	for i, l := range labels {
		if l != exp.labels[i] {
			log.Panicf("Meter: Unexpected label at index %v, got %v want %v", i, l, exp.labels[i])
		}
	}
}

type stubRoundTrip struct {
	stubRes string
}

func (s stubRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	h := make(http.Header)
	h.Add("Content-Type", "application/json")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     h,
		Body:       ioutil.NopCloser(strings.NewReader(s.stubRes)),
	}, nil
}

func makeHTTPRequest(s string) *http.Request {
	req, err := http.NewRequest("POST", "https://test.metricrule.com/predict", strings.NewReader(s))
	if err != nil {
		log.Panicf("Unable to make request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	return req
}

func TestRecordCounterNoLabels(t *testing.T) {
	stubTransport := stubRoundTrip{}
	fakeMeter := fakeMeterImpl{}
	fakeCounter := fakeInstrWrapper{}
	spec := mrmetric.MetricInstrumentSpec{
		InstrumentKind:  metric.CounterInstrumentKind,
		MetricValueKind: reflect.Int64,
		Name:            "simple",
	}
	configTextProto := `
		input_metrics {
			name: "simple"
			simple_counter: {}
		}`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	fakeCounter.expectedCalls = []instrumentRecordCall{{int64(1)}}
	fakeMeter.expectedCalls = []meterRecordCall{{[]attribute.KeyValue{}}}
	stubTransport.stubRes = "{\"class\": \"ok\"}"

	transport := Transport{
		RoundTripper:  stubTransport,
		SidecarConfig: &config,
		InInstrs:      map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper{spec: fakeCounter},
		OutInstrs:     map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper{},
		Meter:         fakeMeter,
	}

	req := makeHTTPRequest("{\"feature\": 1234.567}")
	res, err := transport.RoundTrip(req)
	if res == nil {
		t.Error("Received nil response")
	}
	if err != nil {
		t.Errorf("Roundtrip failed with: %v", err)
	}
}

func TestRecordContextLabelsAppliedForInputAndOutput(t *testing.T) {
	stubTransport := stubRoundTrip{}
	fakeMeter := fakeMeterImpl{}
	fakeCounter := fakeInstrWrapper{}
	fakeRecorder := fakeInstrWrapper{}
	inputSpec := mrmetric.MetricInstrumentSpec{
		InstrumentKind:  metric.CounterInstrumentKind,
		MetricValueKind: reflect.Int64,
		Name:            "simple",
	}
	outputSpec := mrmetric.MetricInstrumentSpec{
		InstrumentKind:  metric.ValueRecorderInstrumentKind,
		MetricValueKind: reflect.Float64,
		Name:            "prediction_recorder",
	}
	configTextProto := `
		input_metrics {
			name: "simple"
			simple_counter: {}
		}
		output_metrics {
			name: "prediction_recorder"
			value {
				value {
					parsed_value {
						field_path: "prediction"
						parsed_type: FLOAT
					}
				}
			}
		}
		context_labels_from_input {
			label_key { string_value: "Country" }
			label_value {
				parsed_value {
					field_path: "country"
					parsed_type: STRING
				}
			}
		}
		`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	// Input counter
	fakeCounter.expectedCalls = []instrumentRecordCall{{int64(1)}}
	// Output recorder
	fakeRecorder.expectedCalls = []instrumentRecordCall{{float64(5)}}

	label := attribute.Key("Country").String("IN")
	// Should be recorded twice, once each for input and output.
	wantCall := meterRecordCall{[]attribute.KeyValue{label}}
	fakeMeter.expectedCalls = []meterRecordCall{wantCall, wantCall}

	stubTransport.stubRes = "{\"prediction\": 5}"

	transport := Transport{
		RoundTripper:  stubTransport,
		SidecarConfig: &config,
		InInstrs:      map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper{inputSpec: fakeCounter},
		OutInstrs:     map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper{outputSpec: fakeRecorder},
		Meter:         fakeMeter,
	}

	req := makeHTTPRequest("{\"feature\": 1234.567, \"country\": \"IN\"}")
	res, err := transport.RoundTrip(req)
	if res == nil {
		t.Error("Received nil response")
	}
	if err != nil {
		t.Errorf("Roundtrip failed with: %v", err)
	}
}
