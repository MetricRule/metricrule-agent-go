package tfmetric

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/otel/metric"
	"google.golang.org/protobuf/encoding/prototext"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
)

func TestInputCounterInstrumentSpec(t *testing.T) {
	configTextProto := `
		input_metrics {
			name: "simple"
			simple_counter: {}
		}`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	specs := GetInstrumentSpecs(&config)

	gotInputLen := len(specs[InputContext])
	wantInputLen := 1
	if gotInputLen != wantInputLen {
		t.Errorf("Unexpected length of input context specs, got %v, wanted %v", gotInputLen, wantInputLen)
	}

	gotOutputLen := len(specs[OutputContext])
	wantOutputLen := 0
	if gotOutputLen != wantOutputLen {
		t.Errorf("Unexpected length of output context specs, got %v, wanted %v", gotOutputLen, wantOutputLen)
	}

	if gotInputLen < 1 {
		return
	}

	gotSpecInstance := specs[InputContext][0]
	wantInstrumentKind := metric.CounterInstrumentKind
	wantMetricKind := reflect.Int64
	wantMetricName := "simple"
	if gotSpecInstance.InstrumentKind != wantInstrumentKind {
		t.Errorf("Unexpected instrument kind in spec, got %v, wanted %v", gotSpecInstance.InstrumentKind, wantInstrumentKind)
	}
	if gotSpecInstance.MetricValueKind != wantMetricKind {
		t.Errorf("Unexpected metric kind in spec, got %v, wanted %v", gotSpecInstance.MetricValueKind, wantMetricKind)
	}
	if gotSpecInstance.Name != wantMetricName {
		t.Errorf("Unexpected metric name in spec, got %v, wanted %v", gotSpecInstance.Name, wantMetricName)
	}
}

func TestInputCounterMetrics(t *testing.T) {
	configTextProto := `
		input_metrics {
			simple_counter: {}
		}`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	metrics := GetMetricInstances(&config, "{}", InputContext)

	gotLen := len(metrics)
	wantLen := 1
	if gotLen != wantLen {
		t.Errorf("Unexpected length of metrics, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	counter := 0
	for spec, instance := range metrics {
		if counter >= wantLen {
			t.Errorf("Exceeded expected iteration length: %v", wantLen)
		}

		gotInstrumentKind := spec.InstrumentKind
		wantInstrumentKind := metric.CounterInstrumentKind
		if gotInstrumentKind != wantInstrumentKind {
			t.Errorf("Unexpected instrument kind, got %v, wanted %v", gotInstrumentKind, wantInstrumentKind)
		}

		gotMetricKind := spec.MetricValueKind
		wantMetricKind := reflect.Int64
		if gotMetricKind != wantMetricKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotMetricKind, wantMetricKind)
		}

		gotValue := instance.MetricValue
		wantValue := int64(1)
		if gotValue != wantValue {
			t.Errorf("Unexpected metric value, got %v, wanted %v", gotValue, wantValue)
		}

		gotLabelsLen := len(instance.Labels)
		wantLabelsLen := 0
		if gotLabelsLen != wantLabelsLen {
			t.Errorf("Unexpected labels length, got %v, wanted %v", gotLabelsLen, wantLabelsLen)
		}
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
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	metrics := GetMetricInstances(&config, "{}", InputContext)

	gotLen := len(metrics)
	wantLen := 1
	if gotLen != wantLen {
		t.Errorf("Unexpected length of metrics, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	counter := 0
	for spec, instance := range metrics {
		if counter >= wantLen {
			t.Errorf("Exceeded expected iteration length: %v", wantLen)
		}

		gotInstrumentKind := spec.InstrumentKind
		wantInstrumentKind := metric.CounterInstrumentKind
		if gotInstrumentKind != wantInstrumentKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotInstrumentKind, wantInstrumentKind)
		}

		gotMetricKind := spec.MetricValueKind
		wantMetricKind := reflect.Int64
		if gotMetricKind != wantMetricKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotMetricKind, wantMetricKind)
		}

		gotValue := instance.MetricValue
		wantValue := int64(1)
		if gotValue != wantValue {
			t.Errorf("Unexpected metric value, got %v, wanted %v", gotValue, wantValue)
		}

		gotLabelsLen := len(instance.Labels)
		wantLabelsLen := 1
		if gotLabelsLen != wantLabelsLen {
			t.Errorf("Unexpected labels length, got %v, wanted %v", gotLabelsLen, wantLabelsLen)
		}

		if gotLabelsLen == 0 {
			return
		}

		gotLabel := instance.Labels[0]
		wantLabelKey := "Application"
		wantLabelValue := "MetricRule"
		if string(gotLabel.Key) != wantLabelKey {
			t.Errorf("Unexpected label key, got %v, wanted %v", gotLabel.Key, wantLabelKey)
		}
		if gotLabel.Value.AsString() != wantLabelValue {
			t.Errorf("Unexpected label key, got %v, wanted %v", gotLabel.Value.AsString(), wantLabelValue)
		}
	}
}

func TestOutputValuesMetrics(t *testing.T) {
	configTextProto := `
		output_metrics {
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

	metrics := GetMetricInstances(&config, "{ \"prediction\": 0.495 }", OutputContext)

	gotLen := len(metrics)
	wantLen := 1
	if gotLen != wantLen {
		t.Errorf("Unexpected length of metrics, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	counter := 0
	for spec, instance := range metrics {
		if counter >= wantLen {
			t.Errorf("Exceeded expected iteration length: %v", wantLen)
		}

		gotInstrumentKind := spec.InstrumentKind
		wantInstrumentKind := metric.ValueRecorderInstrumentKind
		if gotInstrumentKind != wantInstrumentKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotInstrumentKind, wantInstrumentKind)
		}

		gotMetricKind := spec.MetricValueKind
		wantMetricKind := reflect.Float64
		if gotMetricKind != wantMetricKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotMetricKind, wantMetricKind)
		}

		gotValue := instance.MetricValue
		wantValue := 0.495
		if gotValue != wantValue {
			t.Errorf("Unexpected metric value, got %v, wanted %v", gotValue, wantValue)
		}

		gotLabelsLen := len(instance.Labels)
		wantLabelsLen := 0
		if gotLabelsLen != wantLabelsLen {
			t.Errorf("Unexpected labels length, got %v, wanted %v", gotLabelsLen, wantLabelsLen)
		}
	}
}

func TestOutputNestedValuesMetrics(t *testing.T) {
	configTextProto := `
		output_metrics {
			value {
				value {
					parsed_value {
						field_path: "prediction.0.0"
						parsed_type: FLOAT
					}
				}
			}
		}`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	metrics := GetMetricInstances(&config, "{ \"prediction\": [[0.495]] }", OutputContext)

	gotLen := len(metrics)
	wantLen := 1
	if gotLen != wantLen {
		t.Errorf("Unexpected length of metrics, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	counter := 0
	for spec, instance := range metrics {
		if counter >= wantLen {
			t.Errorf("Exceeded expected iteration length: %v", wantLen)
		}

		gotInstrumentKind := spec.InstrumentKind
		wantInstrumentKind := metric.ValueRecorderInstrumentKind
		if gotInstrumentKind != wantInstrumentKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotInstrumentKind, wantInstrumentKind)
		}

		gotMetricKind := spec.MetricValueKind
		wantMetricKind := reflect.Float64
		if gotMetricKind != wantMetricKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotMetricKind, wantMetricKind)
		}

		gotValue := instance.MetricValue
		wantValue := 0.495
		if gotValue != wantValue {
			t.Errorf("Unexpected metric value, got %v, wanted %v", gotValue, wantValue)
		}

		gotLabelsLen := len(instance.Labels)
		wantLabelsLen := 0
		if gotLabelsLen != wantLabelsLen {
			t.Errorf("Unexpected labels length, got %v, wanted %v", gotLabelsLen, wantLabelsLen)
		}
	}
}

func TestMultipleInputsNestedMetrics(t *testing.T) {
	configTextProto := `
		input_metrics {
			name: "input_distribution_counts"
			simple_counter {}
			labels {
				label_key { string_value: "PetType" }
				label_value {
					parsed_value {
						field_path: "instances.0.Type.0"
						parsed_type: STRING
					}
				}
			}
			labels {
				label_key { string_value: "Breed" }
				label_value {
					parsed_value {
						field_path: "instances.0.Breed1.0"
						parsed_type: STRING
					}
				}
			}
		}`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	response := `{
		"instances": [
			{
				"Type": [
					"Cat"
				],
				"Age": [
					4
				],
				"Breed1": [
					"Turkish"
				]
			}
		]
	}`
	metrics := GetMetricInstances(&config, response, InputContext)

	gotLen := len(metrics)
	wantLen := 1
	if gotLen != wantLen {
		t.Errorf("Unexpected length of metrics, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	counter := 0
	for spec, instance := range metrics {
		if counter >= wantLen {
			t.Errorf("Exceeded expected iteration length: %v", wantLen)
		}

		gotInstrumentKind := spec.InstrumentKind
		wantInstrumentKind := metric.CounterInstrumentKind
		if gotInstrumentKind != wantInstrumentKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotInstrumentKind, wantInstrumentKind)
		}

		gotMetricKind := spec.MetricValueKind
		wantMetricKind := reflect.Int64
		if gotMetricKind != wantMetricKind {
			t.Errorf("Unexpected metric kind, got %v, wanted %v", gotMetricKind, wantMetricKind)
		}

		gotValue := instance.MetricValue
		wantValue := int64(1)
		if gotValue != wantValue {
			t.Errorf("Unexpected metric value, got %v, wanted %v", gotValue, wantValue)
		}

		gotLabelsLen := len(instance.Labels)
		wantLabelsLen := 2
		if gotLabelsLen != wantLabelsLen {
			t.Errorf("Unexpected labels length, got %v, wanted %v", gotLabelsLen, wantLabelsLen)
		}

		if gotLabelsLen == 0 {
			return
		}

		got1Label := instance.Labels[0]
		want1LabelKey := "PetType"
		want1LabelValue := "Cat"
		if string(got1Label.Key) != want1LabelKey {
			t.Errorf("Unexpected label key, got %v, wanted %v", got1Label.Key, want1LabelKey)
		}
		if got1Label.Value.AsString() != want1LabelValue {
			t.Errorf("Unexpected label key, got %v, wanted %v", got1Label.Value.AsString(), want1LabelValue)
		}

		got2Label := instance.Labels[1]
		want2LabelKey := "Breed"
		want2LabelValue := "Turkish"
		if string(got2Label.Key) != want2LabelKey {
			t.Errorf("Unexpected label key, got %v, wanted %v", got2Label.Key, want2LabelKey)
		}
		if got2Label.Value.AsString() != want2LabelValue {
			t.Errorf("Unexpected label key, got %v, wanted %v", got2Label.Value.AsString(), want2LabelValue)
		}
	}
}

func TestGetInputContextLabels(t *testing.T) {
	configTextProto := `
		context_labels_from_input {
			label_key { string_value: "PetType" }
			label_value {
				parsed_value {
					field_path: "Type"
					parsed_type: STRING
				}
			}
		}
		context_labels_from_input {
			label_key { string_value: "Breed" }
			label_value {
				parsed_value {
					field_path: "Breed"
					parsed_type: STRING
				}
			}
		}
		`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	response := `{
		"Type": "Cat",
		"Breed": "Turkish"
	}`
	labels := GetContextLabels(&config, response, InputContext)

	gotLen := len(labels)
	wantLen := 2
	if gotLen != wantLen {
		t.Errorf("Unexpected length of labels, got %v, wanted %v", gotLen, wantLen)
	}

	if gotLen == 0 {
		return
	}

	got1Label := labels[0]
	want1LabelKey := "PetType"
	want1LabelValue := "Cat"
	if string(got1Label.Key) != want1LabelKey {
		t.Errorf("Unexpected label key, got %v, wanted %v", got1Label.Key, want1LabelKey)
	}
	if got1Label.Value.AsString() != want1LabelValue {
		t.Errorf("Unexpected label key, got %v, wanted %v", got1Label.Value.AsString(), want1LabelValue)
	}

	got2Label := labels[1]
	want2LabelKey := "Breed"
	want2LabelValue := "Turkish"
	if string(got2Label.Key) != want2LabelKey {
		t.Errorf("Unexpected label key, got %v, wanted %v", got2Label.Key, want2LabelKey)
	}
	if got2Label.Value.AsString() != want2LabelValue {
		t.Errorf("Unexpected label key, got %v, wanted %v", got2Label.Value.AsString(), want2LabelValue)
	}
}
