package mrotel

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/number"

	"github.com/metricrule-sidecar-tfserving/pkg/mrmetric"
)

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
