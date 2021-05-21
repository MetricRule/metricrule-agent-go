package mrotel

import (
	"errors"
	"reflect"

	"github.com/golang/glog"
	"go.opentelemetry.io/otel/metric"

	"github.com/metricrule-agent-go/pkg/mrmetric"
)

// InstrumentWrapper wraps around various types of opentelemetry instruments.
// It provides a single Record method.
type InstrumentWrapper interface {
	Record(value interface{}) (metric.Measurement, error)
}

type int64CounterWrapper struct {
	c metric.Int64Counter
}

func (c int64CounterWrapper) Record(value interface{}) (metric.Measurement, error) {
	number, ok := value.(int64)
	if !ok {
		glog.Errorf("Unexpected value %v in int 64 counter", value)
		return c.c.Measurement(0), errors.New("Unexpected value")
	}
	return c.c.Measurement(number), nil
}

type int64ValueRecorderWrapper struct {
	c metric.Int64ValueRecorder
}

func (c int64ValueRecorderWrapper) Record(value interface{}) (metric.Measurement, error) {
	number, ok := value.(int64)
	if !ok {
		glog.Errorf("Unexpected value %v in int 64 value recorder", value)
		return c.c.Measurement(0), errors.New("Unexpected value")
	}
	return c.c.Measurement(number), nil
}

type float64ValueRecorderWrapper struct {
	c metric.Float64ValueRecorder
}

func (c float64ValueRecorderWrapper) Record(value interface{}) (metric.Measurement, error) {
	number, ok := value.(float64)
	if !ok {
		glog.Errorf("Unexpected value %v in float 64 value recorder", value)
		return c.c.Measurement(0), errors.New("Unexpected value")
	}
	return c.c.Measurement(number), nil
}

type noOpWrapper struct{}

func (c noOpWrapper) Record(value interface{}) (metric.Measurement, error) {
	return metric.Measurement{}, errors.New("No op wrapper used")
}

// InitializeInstrument creates an instrument with the meter for the given spec.
func InitializeInstrument(meter metric.Meter, spec mrmetric.MetricInstrumentSpec) InstrumentWrapper {
	switch spec.InstrumentKind {
	case metric.CounterInstrumentKind:
		switch spec.MetricValueKind {
		case reflect.Int64:
			c, err := meter.NewInt64Counter(spec.Name)
			if err == nil {
				return int64CounterWrapper{c}
			}
			glog.Warningf("Error initializing counter: %v", err)
		}
	case metric.ValueRecorderInstrumentKind:
		switch spec.MetricValueKind {
		case reflect.Int64:
			c, err := meter.NewInt64ValueRecorder(spec.Name)
			if err == nil {
				return int64ValueRecorderWrapper{c}
			}
			glog.Warningf("Error initializing recorder: %v", err)
		case reflect.Float64:
			c, err := meter.NewFloat64ValueRecorder(spec.Name)
			if err == nil {
				return float64ValueRecorderWrapper{c}
			}
			glog.Warningf("Error initializing recorder: %v", err)
		}
	}
	glog.Errorf("No instrument could be created for spec %v", spec.Name)
	return noOpWrapper{}
}
