package mrotel

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/number"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/sum"
	"google.golang.org/protobuf/encoding/prototext"

	configpb "github.com/metricrule-agent-go/api/proto/metricconfigpb"
	"github.com/metricrule-agent-go/pkg/mrmetric"
)

type fakeInstrWrapper struct {
	d *metric.Descriptor
}

func (f fakeInstrWrapper) Record(value interface{}) (metric.Measurement, error) {
	return metric.Measurement{}, nil
}

func (f fakeInstrWrapper) Describe() metric.Descriptor {
	return *f.d
}

func TestInitializeIntCounterAndRecord(t *testing.T) {
	spec := mrmetric.MetricInstrumentSpec{
		InstrumentKind:  metric.CounterInstrumentKind,
		MetricValueKind: reflect.Int64,
		Name:            "TestIntCounter",
	}
	meter := metric.Meter{}

	instr := InitializeInstrument(meter, spec)
	m, err := instr.Record(int64(1))

	if err != nil {
		t.Errorf("Error on record: %v", err)
	}
	if m.Number() != 1 {
		t.Errorf("Unexpected measurement, got %v want 1", m.Number())
	}
}

func TestInitializeFloatValueRecorderAndRecord(t *testing.T) {
	spec := mrmetric.MetricInstrumentSpec{
		InstrumentKind:  metric.ValueRecorderInstrumentKind,
		MetricValueKind: reflect.Float64,
		Name:            "TestFloatCounter",
	}
	meter := metric.Meter{}

	instr := InitializeInstrument(meter, spec)
	m, err := instr.Record(float64(1.1))

	if err != nil {
		t.Errorf("Error on record: %v", err)
	}
	if m.Number() != number.NewFloat64Number(1.1) {
		t.Errorf("Unexpected measurement, got %v want 1.0", m.Number())
	}
}

func TestIntRecorderFailsToRecordFloat(t *testing.T) {
	spec := mrmetric.MetricInstrumentSpec{
		InstrumentKind:  metric.ValueRecorderInstrumentKind,
		MetricValueKind: reflect.Int64,
		Name:            "TestErrorIntCounter",
	}
	meter := metric.Meter{}

	instr := InitializeInstrument(meter, spec)
	m, err := instr.Record(float64(1.0))

	if err == nil {
		t.Errorf("No error received on incorrect type.")
	}
	if m.Number() != 0 {
		t.Errorf("Unexpected measurement, got %v want 0", m.Number())
	}
}

func TestNewAggregatorProvider(t *testing.T) {
	a := NewAggregatorProvider()

	_, ok := interface{}(a).(export.AggregatorSelector)
	if !ok {
		t.Errorf("Expected %v to conform to AggregatorSelector", a)
	}
	if len(a.c) != 0 {
		t.Error("Expected map to be initialized empty")
	}
}

func TestAggregatorForDescriptor(t *testing.T) {
	d := metric.NewDescriptor("Fake", metric.ValueRecorderInstrumentKind, number.Float64Kind)
	spec := mrmetric.AggregatorSpec{
		HistogramBins: []float64{0.25, 0.5, 1},
	}
	m := map[metric.Descriptor]mrmetric.AggregatorSpec{d: spec}
	p := AggregatorProvider{m}

	var agg export.Aggregator
	p.AggregatorFor(&d, &agg)

	if agg == nil {
		t.Error("Expected created aggregator to not be nil.")
	}

	h, ok := agg.(*histogram.Aggregator)
	if !ok {
		t.Error("Expected created aggregator to be histogram")
	} else {
		h, err := h.Histogram()
		if err != nil {
			t.Errorf("Aggregator threw error when creating histogram: %v", err)
		}

		if !reflect.DeepEqual(h.Boundaries, spec.HistogramBins) {
			t.Error("Expected histogram to have bins as specified")
		}
	}
}

func TestFallsbacktoDefaultIfUnknownDescriptor(t *testing.T) {
	d := metric.NewDescriptor("Test", metric.CounterInstrumentKind, number.Int64Kind)
	m := make(map[metric.Descriptor]mrmetric.AggregatorSpec)
	p := AggregatorProvider{m}

	var agg export.Aggregator
	p.AggregatorFor(&d, &agg)

	if agg == nil {
		t.Error("Expected created aggregator to not be nil.")
	}

	// Default behavior is to create a sum aggregator to maintain int counter.
	_, ok := agg.(*sum.Aggregator)
	if !ok {
		t.Error("Expected created aggregator to be sum")
	}
}

func TestUpdateProvider(t *testing.T) {
	d := metric.NewDescriptor("Test", metric.ValueRecorderInstrumentKind, number.Float64Kind)
	m := make(map[metric.Descriptor]mrmetric.AggregatorSpec)
	p := AggregatorProvider{m}

	configTextProto := `
		output_metrics {
			value {
				value {
					parsed_value {
						field_path: "$.prediction"
						parsed_type: FLOAT
					}
				}
				bins: 0.0,
				bins: 0.25,
				bins: 0.5,
				bins: 0.75,
				bins: 1.0,
			}
		}`
	var config configpb.SidecarConfig
	_ = prototext.Unmarshal([]byte(configTextProto), &config)

	spec := mrmetric.MetricInstrumentSpec{
		Name:            "",
		InstrumentKind:  metric.ValueRecorderInstrumentKind,
		MetricValueKind: reflect.Float64,
	}
	instr := fakeInstrWrapper{&d}
	instrs := map[mrmetric.MetricInstrumentSpec]InstrumentWrapper{spec: instr}

	p.Update(instrs, &config)

	if val, ok := m[d]; ok {
		gotBins := val.HistogramBins
		wantBins := []float64{0.0, 0.25, 0.5, 0.75, 1.0}
		if !reflect.DeepEqual(gotBins, wantBins) {
			t.Errorf("Expected %v bins for descriptor, got %v bins", wantBins, gotBins)
		}
	} else {
		t.Errorf("Expected aggregator for descriptor to be present")
	}
}
