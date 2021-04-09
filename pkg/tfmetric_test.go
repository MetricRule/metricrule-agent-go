package tfmetric

import (
	"testing"

	"go.opentelemetry.io/otel/metric"
	"google.golang.org/protobuf/encoding/prototext"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
)

func TestInputCounter(t *testing.T) {
	configTextProto := `
		input_metrics {
			simple_counter: {}
		}`
	var config configpb.SidecarConfig
	prototext.Unmarshal([]byte(configTextProto), &config)

	metrics := CreateMetrics(&config, "{}", InputContext)

	gotLen := len(metrics)
	wantLen := 1
	if gotLen != wantLen {
		t.Errorf("Unexpected length of metrics, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	gotFirstInstance := metrics[0]
	gotKind := gotFirstInstance.metricKind
	wantKind := metric.CounterInstrumentKind
	if gotKind != wantKind {
		t.Errorf("Unexpected metric kind, got %v, wanted %v", gotKind, wantKind)
	}

	gotValue := gotFirstInstance.metricValue
	wantValue := 1
	if gotValue != wantValue {
		t.Errorf("Unexpected metric value, got %v, wanted %v", gotValue, wantValue)
	}

	gotLabelsLen := len(gotFirstInstance.labels)
	wantLabelsLen := 0
	if gotLabelsLen != wantLabelsLen {
		t.Errorf("Unexpected labels length, got %v, wanted %v", gotLabelsLen, wantLabelsLen)
	}
}

func TestInputCounterWithLabels(t *testing.T) {
	configTextProto := `
		input_metrics {
			simple_counter: {}
			labels: {
				label_key: { string_value: "Application" }
				label_value: { string_value: "MetricRule" }
			}
		}`
	var config configpb.SidecarConfig
	prototext.Unmarshal([]byte(configTextProto), &config)

	metrics := CreateMetrics(&config, "{}", InputContext)

	gotLen := len(metrics)
	wantLen := 1
	if gotLen != wantLen {
		t.Errorf("Unexpected length of metrics, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	gotFirstInstance := metrics[0]
	gotKind := gotFirstInstance.metricKind
	wantKind := metric.CounterInstrumentKind
	if gotKind != wantKind {
		t.Errorf("Unexpected metric kind, got %v, wanted %v", gotKind, wantKind)
	}

	gotValue := gotFirstInstance.metricValue
	wantValue := 1
	if gotValue != wantValue {
		t.Errorf("Unexpected metric value, got %v, wanted %v", gotValue, wantValue)
	}

	gotLabelsLen := len(gotFirstInstance.labels)
	wantLabelsLen := 1
	if gotLabelsLen != wantLabelsLen {
		t.Errorf("Unexpected labels length, got %v, wanted %v", gotLabelsLen, wantLabelsLen)
	}

	if gotLabelsLen == 0 {
		return
	}

	gotLabel := gotFirstInstance.labels[0]
	wantLabelKey := "Application"
	wantLabelValue := "MetricRule"
	if string(gotLabel.Key) != wantLabelKey {
		t.Errorf("Unexpected label key, got %v, wanted %v", gotLabel.Key, wantLabelKey)
	}
	if gotLabel.Value.AsString() != wantLabelValue {
		t.Errorf("Unexpected label key, got %v, wanted %v", gotLabel.Value.AsString(), wantLabelValue)
	}
}

func TestOutputValues(t *testing.T) {
	configTextProto := `
		output_metrics {
			value {
				value {
					parsed_value {
						field_path: {
							paths: "prediction"
						}
						parsed_type: FLOAT
					}
				}
			}
		}`
	var config configpb.SidecarConfig
	prototext.Unmarshal([]byte(configTextProto), &config)

	metrics := CreateMetrics(&config, "{ \"prediction\": 0.495 }", OutputContext)

	gotLen := len(metrics)
	wantLen := 1
	if gotLen != wantLen {
		t.Errorf("Unexpected length of metrics, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	gotFirstInstance := metrics[0]
	gotKind := gotFirstInstance.metricKind
	wantKind := metric.ValueRecorderInstrumentKind
	if gotKind != wantKind {
		t.Errorf("Unexpected metric kind, got %v, wanted %v", gotKind, wantKind)
	}

	gotValue := gotFirstInstance.metricValue
	wantValue := 0.495
	if gotValue != wantValue {
		t.Errorf("Unexpected metric value, got %v, wanted %v", gotValue, wantValue)
	}

	gotLabelsLen := len(gotFirstInstance.labels)
	wantLabelsLen := 0
	if gotLabelsLen != wantLabelsLen {
		t.Errorf("Unexpected labels length, got %v, wanted %v", gotLabelsLen, wantLabelsLen)
	}
}
