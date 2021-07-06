package mrotel

import (
	"errors"
	"reflect"

	"github.com/golang/glog"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/number"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"

	configpb "github.com/metricrule-agent-go/api/proto/metricconfigpb"
	"github.com/metricrule-agent-go/pkg/mrmetric"
)

// InstrumentWrapper wraps around various types of opentelemetry instruments.
// It provides a single Record method.
type InstrumentWrapper interface {
	// Record returns either a single measurement object or an error.
	Record(value interface{}) (metric.Measurement, error)

	// Describe returns a descriptor object for the instrument.
	Describe() metric.Descriptor
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

func (c int64CounterWrapper) Describe() metric.Descriptor {
	return c.c.SyncImpl().Descriptor()
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

func (c int64ValueRecorderWrapper) Describe() metric.Descriptor {
	return c.c.SyncImpl().Descriptor()
}

type float64ValueRecorderWrapper struct {
	c metric.Float64ValueRecorder
}

func (c float64ValueRecorderWrapper) Record(value interface{}) (metric.Measurement, error) {
	number, ok := value.(float64)
	if !ok {
		glog.Errorf("Unexpected value %v in float 64 value recorder", value)
		return c.c.Measurement(0), errors.New("unexpected value")
	}
	return c.c.Measurement(number), nil
}

func (c float64ValueRecorderWrapper) Describe() metric.Descriptor {
	return c.c.SyncImpl().Descriptor()
}

type noOpWrapper struct{}

func (c noOpWrapper) Record(value interface{}) (metric.Measurement, error) {
	return metric.Measurement{}, errors.New("no op wrapper used")
}

func (c noOpWrapper) Describe() metric.Descriptor {
	return metric.NewDescriptor("No-op", metric.CounterInstrumentKind, number.Int64Kind)
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

// AggregatorProvider returns an appropriate aggregator for given metric descriptor.
type AggregatorProvider struct {
	c map[metric.Descriptor]mrmetric.AggregatorSpec
}

// AggregatorFor sets the provided pointer/s to aggregators initialized for the given
// descriptor.
func (a AggregatorProvider) AggregatorFor(descriptor *metric.Descriptor, aggPtrs ...*export.Aggregator) {
	if spec, ok := a.c[*descriptor]; ok {
		if spec.HistogramBins != nil && len(spec.HistogramBins) > 0 {
			aggs := histogram.New(len(aggPtrs), descriptor,
				histogram.WithExplicitBoundaries(spec.HistogramBins))
			for i := range aggPtrs {
				*aggPtrs[i] = &aggs[i]
			}
			return
		}
	}
	defaultAgg := simple.NewWithHistogramDistribution()
	defaultAgg.AggregatorFor(descriptor, aggPtrs...)
}

// Update adds aggregator configurations in the provider for the given list of instruments.
func (a AggregatorProvider) Update(instrs map[mrmetric.MetricInstrumentSpec]InstrumentWrapper,
	config *configpb.AgentConfig) {
	aggSpecs := mrmetric.GetAggregatorSpecs(config)
	for spec, instr := range instrs {
		if aggSpec, ok := aggSpecs[spec]; ok {
			d := instr.Describe()
			a.c[d] = aggSpec
		}
	}
}

// NewAggregatorProvider returns a new empty instance of AggregatorProvider.
func NewAggregatorProvider() AggregatorProvider {
	c := make(map[metric.Descriptor]mrmetric.AggregatorSpec)
	return AggregatorProvider{c}
}
