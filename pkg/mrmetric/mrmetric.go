package mrmetric

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"reflect"
	"strconv"

	"github.com/golang/glog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/client-go/util/jsonpath"

	configpb "github.com/metricrule-agent-go/api/proto/metricconfigpb"
)

// MetricInstrumentSpec specifies an instrument to record a metric.
// Multiple instances of a metric, over time, can be associated with
// this specification.
// Specifies the kind of instrument (counter, value recorder), the
// kind of value (int64, float64), and a name of the metric.
type MetricInstrumentSpec struct {
	// The kind of instrument to use.
	InstrumentKind metric.InstrumentKind
	// The type of value to be measured in the metric.
	MetricValueKind reflect.Kind
	// Identifying name of this metric
	Name string
}

// A MetricInstance is a single instance of a metric to be recorded.
// Consists of a list of values (of the type specified by MetricValueKind)
// and a list of key-value labels to be associated with the recording
// event.
type MetricInstance struct {
	// The metric values (all must be of the same type).
	MetricValues []interface{}
	// Sets of key-value pairs to use as labels.
	Labels []attribute.KeyValue
}

// MetricContext is the context to record the metric for.
// This is either InputContext for recording input features or
// OutputContext for recording output values.
type MetricContext int

const (
	// UnknownContext - Do not use.
	UnknownContext MetricContext = iota
	// InputContext creates metrics for input features and data.
	InputContext
	// OutputContext creates metrics for model outputs.
	OutputContext
)

// GetInstrumentSpecs returns the specifications of metric instruments to create
// for recording metrics, per each input context, for the provided config.
func GetInstrumentSpecs(config *configpb.SidecarConfig) map[MetricContext][]MetricInstrumentSpec {
	specs := make(map[MetricContext][]MetricInstrumentSpec)

	if len(config.InputMetrics) > 0 {
		specs[InputContext] = []MetricInstrumentSpec{}
		for _, c := range config.InputMetrics {
			specs[InputContext] = append(specs[InputContext], getInstrumentSpec(c))
		}
	}

	if len(config.OutputMetrics) > 0 {
		specs[OutputContext] = []MetricInstrumentSpec{}
		for _, c := range config.OutputMetrics {
			specs[OutputContext] = append(specs[OutputContext], getInstrumentSpec(c))
		}
	}

	return specs
}

// GetMetricInstances returns a map of metric specifications to the instances to record.
func GetMetricInstances(config *configpb.SidecarConfig, payload string, context MetricContext) map[MetricInstrumentSpec]MetricInstance {
	configs := []*configpb.MetricConfig{}
	if context == InputContext {
		configs = config.InputMetrics
	}
	if context == OutputContext {
		configs = config.OutputMetrics
	}

	var jsonObj interface{}
	err := json.Unmarshal([]byte(payload), &jsonObj)
	if err != nil {
		log.Fatal("Error when umarshaling payload json", err)
	}

	output := make(map[MetricInstrumentSpec]MetricInstance)
	for _, config := range configs {
		v := getMetricValues(config, jsonObj)
		l := getMetricLabels(config, jsonObj)
		s := getInstrumentSpec(config)
		output[s] = MetricInstance{v, l}
	}

	return output
}

// GetContextLabels returns a list of labels extracted for the supplied context.
// This is currently only supported for InputContext.
func GetContextLabels(config *configpb.SidecarConfig, payload string, context MetricContext) []attribute.KeyValue {
	configs := []*configpb.LabelConfig{}
	if context == InputContext {
		configs = config.ContextLabelsFromInput
	}
	if context == OutputContext {
		return []attribute.KeyValue{}
	}

	var jsonObj interface{}
	err := json.Unmarshal([]byte(payload), &jsonObj)
	if err != nil {
		log.Fatal("Error when umarshaling payload json", err)
	}

	var ls []attribute.KeyValue
	for _, lconf := range configs {
		lls := getLabelsForLabelConf(lconf, jsonObj)
		for _, l := range lls {
			if l.Valid() {
				ls = append(ls, l)
			}
		}
	}
	return ls
}

func getInstrumentSpec(config *configpb.MetricConfig) MetricInstrumentSpec {
	i := getInstrumentKind(config)
	m := getMetricKind(config)
	return MetricInstrumentSpec{i, m, config.Name}
}

func getInstrumentKind(config *configpb.MetricConfig) metric.InstrumentKind {
	if config.GetSimpleCounter() != nil {
		return metric.CounterInstrumentKind
	}
	if config.GetValue() != nil {
		return metric.ValueRecorderInstrumentKind
	}
	return -1
}

func getMetricValues(config *configpb.MetricConfig, jsonPayload interface{}) []interface{} {
	if config.GetSimpleCounter() != nil {
		return []interface{}{int64(1)}
	}
	if config.GetValue() != nil {
		v := config.GetValue().GetValue()
		return extractValues(v, jsonPayload)
	}
	// Default to a counter
	return []interface{}{int64(1)}
}

func getMetricLabels(config *configpb.MetricConfig, jsonObj interface{}) []attribute.KeyValue {
	var ls []attribute.KeyValue
	for _, lconf := range config.Labels {
		lls := getLabelsForLabelConf(lconf, jsonObj)
		for _, l := range lls {
			if l.Valid() {
				ls = append(ls, l)
			}
		}
	}
	return ls
}

func getLabelsForLabelConf(config *configpb.LabelConfig, jsonObj interface{}) []attribute.KeyValue {
	ks := extractValues(config.LabelKey, jsonObj)
	vs := extractValues(config.LabelValue, jsonObj)

	klen := len(ks)
	vlen := len(vs)
	iterlen := int(math.Max(float64(klen), float64(vlen)))

	if klen == 0 || vlen == 0 {
		return []attribute.KeyValue{}
	}

	labels := []attribute.KeyValue{}
	for i := 0; i < iterlen; i++ {
		key := ks[i%klen]
		value := vs[i%vlen]

		var l attribute.KeyValue
		// The key must be a string.
		if s, ok := key.(string); ok {
			k := attribute.Key(s)
			// We expect floats, ints, and strings only.
			switch value := value.(type) {
			case string:
				l = k.String(value)
			case int64:
				l = k.Int64(value)
			case float64:
				l = k.Float64(value)
			}
		}
		labels = append(labels, l)
	}
	return labels
}

func getMetricKind(config *configpb.MetricConfig) reflect.Kind {
	if config.GetSimpleCounter() != nil {
		return reflect.Int64
	}
	if config.GetValue() != nil {
		return getValueMetricKind(config.GetValue().GetValue())
	}
	return reflect.Invalid
}

// Gets the kind of metric values to record.
// Only ints and floats are supported. Notably, strings are not
// supported - they should perhaps be modeled as counters with the 'value'
// being a label.
func getValueMetricKind(config *configpb.ValueConfig) reflect.Kind {
	if config.GetParsedValue() != nil {
		switch config.GetParsedValue().ParsedType {
		case configpb.ParsedValue_INTEGER:
			return reflect.Int64
		case configpb.ParsedValue_FLOAT:
			return reflect.Float64
		}
	}

	if config.GetStaticValue() != nil {
		if _, ok := config.GetStaticValue().(*configpb.ValueConfig_IntegerValue); ok {
			return reflect.Int64
		}
		if _, ok := config.GetStaticValue().(*configpb.ValueConfig_FloatValue); ok {
			return reflect.Float64
		}
	}

	return reflect.Invalid
}

func extractValues(config *configpb.ValueConfig, jsonPayload interface{}) []interface{} {
	if config.GetParsedValue() != nil {
		vType := config.GetParsedValue().ParsedType
		j := jsonpath.New("value_extractor")
		j.AllowMissingKeys(true)
		j.EnableJSONOutput(true)

		err := j.Parse(fmt.Sprintf("{ %v }", config.ParsedValue.FieldPath))
		if err != nil {
			glog.Errorf("Unable to parse jsonPath %v\n.Err: %v", config.ParsedValue.FieldPath, err)
			return []interface{}{defaultValueForType(vType)}
		}

		buf := new(bytes.Buffer)
		err = j.Execute(buf, jsonPayload)
		if err != nil {
			glog.Error("Unable to find results in JSON payload\n", err)
			return []interface{}{defaultValueForType(vType)}
		}
		var extracted []interface{}
		err = json.Unmarshal(buf.Bytes(), &extracted)
		if err != nil {
			glog.Error("Unable to unmarshal jsonpath output\n", err)
			return []interface{}{defaultValueForType(vType)}
		}

		vals := []interface{}{}
		for _, e := range extracted {
			vals = append(vals, getTypedValue(e, vType))
		}
		return vals
	}

	if config.GetStaticValue() != nil {
		if config, ok := config.GetStaticValue().(*configpb.ValueConfig_StringValue); ok {
			return []interface{}{config.StringValue}
		}
		if config, ok := config.GetStaticValue().(*configpb.ValueConfig_IntegerValue); ok {
			return []interface{}{config.IntegerValue}
		}
		if config, ok := config.GetStaticValue().(*configpb.ValueConfig_FloatValue); ok {
			return []interface{}{config.FloatValue}
		}
	}

	return []interface{}{}
}

func getTypedValue(v interface{}, t configpb.ParsedValue_ParsedType) interface{} {
	switch valueP := v.(type) {
	case string:
		switch t {
		case configpb.ParsedValue_STRING:
			return valueP
		case configpb.ParsedValue_FLOAT:
			f, err := strconv.ParseFloat(valueP, 64)
			if err != nil {
				log.Fatal("Error parsing float", err)
			}
			return f
		case configpb.ParsedValue_INTEGER:
			n, err := strconv.ParseInt(valueP, 10, 64)
			if err != nil {
				log.Fatal("Error parsing integer", err)
			}
			return n
		}
	case float64:
		switch t {
		case configpb.ParsedValue_FLOAT:
			return valueP
		case configpb.ParsedValue_INTEGER:
			return int64(valueP)
		case configpb.ParsedValue_STRING:
			return fmt.Sprintf("%f", valueP)
		}
	default:
		log.Printf("Unexpected field when parsing JSON payload. Value %v", v)
		return defaultValueForType(t)
	}
	return nil
}

func defaultValueForType(t configpb.ParsedValue_ParsedType) interface{} {
	switch t {
	case configpb.ParsedValue_FLOAT:
		return float64(0)
	case configpb.ParsedValue_STRING:
		return ""
	case configpb.ParsedValue_INTEGER:
		return int64(0)
	}
	return nil
}
