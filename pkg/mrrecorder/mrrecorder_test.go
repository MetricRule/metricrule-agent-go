package mrrecorder

import (
	"context"
	"log"
	"reflect"
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

func TestRecordInputCounterNoLabels(t *testing.T) {
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

	requestData := []byte("{\"feature\": 1234.567}")
	logData := RequestLogData{
		Dump:   requestData,
		Config: &config,
		Instrs: map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper{spec: fakeCounter},
		Meter:  fakeMeter,
	}
	ctxChan := make(chan []attribute.KeyValue, 1)

	LogRequestData(logData, ctxChan)
}

func TestOutputRecorderNoLabels(t *testing.T) {
	fakeMeter := fakeMeterImpl{}
	fakeRecorder := fakeInstrWrapper{}
	spec := mrmetric.MetricInstrumentSpec{
		InstrumentKind:  metric.ValueRecorderInstrumentKind,
		MetricValueKind: reflect.Float64,
		Name:            "prediction_recorder",
	}
	configTextProto := `
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
		}`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	fakeRecorder.expectedCalls = []instrumentRecordCall{{float64(1)}}
	fakeMeter.expectedCalls = []meterRecordCall{{[]attribute.KeyValue{}}}

	responseData := []byte("{\"prediction\": 1}")
	logData := ResponseLogData{
		Dump:   responseData,
		Config: &config,
		Instrs: map[mrmetric.MetricInstrumentSpec]mrotel.InstrumentWrapper{spec: fakeRecorder},
		Meter:  fakeMeter,
	}
	ctxChan := make(chan []attribute.KeyValue, 1)
	ctxChan <- []attribute.KeyValue{}

	LogResponseData(logData, ctxChan)
}
