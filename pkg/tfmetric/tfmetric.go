package tfmetric

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
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
// Consists of a value (of the type specified by MetricValueKind)
// and a list of key-value labels to be associated with the recording
// event.
type MetricInstance struct {
	// The value of the metric.
	MetricValue interface{}
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
		for _, config := range config.InputMetrics {
			specs[InputContext] = append(specs[InputContext], getInstrumentSpec(config))
		}
	}

	if len(config.OutputMetrics) > 0 {
		specs[OutputContext] = []MetricInstrumentSpec{}
		for _, config := range config.OutputMetrics {
			specs[OutputContext] = append(specs[OutputContext], getInstrumentSpec(config))
		}
	}

	return specs
}

// GetMetricInstances returns a map of metric specifications to the instances to record.
func GetMetricInstances(config *configpb.SidecarConfig, payload string, context MetricContext) map[MetricInstrumentSpec]MetricInstance {
	metricConfigs := []*configpb.MetricConfig{}
	if context == InputContext {
		metricConfigs = config.InputMetrics
	}
	if context == OutputContext {
		metricConfigs = config.OutputMetrics
	}

	var jsonPayload interface{}
	err := json.Unmarshal([]byte(payload), &jsonPayload)
	if err != nil {
		log.Fatal("Error when umarshaling payload json", err)
	}

	output := make(map[MetricInstrumentSpec]MetricInstance)
	for _, config := range metricConfigs {
		value := getMetricValue(config, jsonPayload)
		labels := getMetricLabels(config, jsonPayload)
		spec := getInstrumentSpec(config)
		output[spec] = MetricInstance{value, labels}
	}

	return output
}

func getInstrumentSpec(config *configpb.MetricConfig) MetricInstrumentSpec {
	instrumentKind := getInstrumentKind(config)
	metricKind := getMetricKind(config)
	spec := MetricInstrumentSpec{instrumentKind, metricKind, config.Name}
	return spec
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

func getMetricValue(config *configpb.MetricConfig, jsonPayload interface{}) interface{} {
	if config.GetSimpleCounter() != nil {
		return int64(1)
	}
	if config.GetValue() != nil {
		value := config.GetValue().GetValue()
		return extractValue(value, jsonPayload)
	}
	// Default to a counter
	return int64(1)
}

func getMetricLabels(config *configpb.MetricConfig, jsonPayload interface{}) []attribute.KeyValue {
	var labels []attribute.KeyValue
	for _, labelConfig := range config.Labels {
		key := extractValue(labelConfig.LabelKey, jsonPayload)
		value := extractValue(labelConfig.LabelValue, jsonPayload)

		// The key must be a string.
		if keyString, ok := key.(string); ok {
			attributeKey := attribute.Key(keyString)
			// We expect floats, ints, and strings only.
			var keyValue attribute.KeyValue
			switch value := value.(type) {
			case string:
				keyValue = attributeKey.String(value)
			case int64:
				keyValue = attributeKey.Int64(value)
			case float64:
				keyValue = attributeKey.Float64(value)
			}
			labels = append(labels, keyValue)
		}
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

func extractValue(config *configpb.ValueConfig, jsonPayload interface{}) interface{} {
	if config.GetParsedValue() != nil {
		path := config.GetParsedValue().FieldPath.Paths[0]
		pathSegments := strings.Split(path, ".")
		valueType := config.GetParsedValue().ParsedType

		payload := jsonPayload
		for _, segment := range pathSegments {
			if mapPayload, ok := payload.(map[string]interface{}); ok {
				payload = mapPayload[segment]
			}
		}
		switch valuePayload := payload.(type) {
		case string:
			switch valueType {
			case configpb.ParsedValue_STRING:
				return valuePayload
			case configpb.ParsedValue_FLOAT:
				val, err := strconv.ParseFloat(valuePayload, 64)
				if err != nil {
					log.Fatal("Error parsing float", err)
				}
				return val
			case configpb.ParsedValue_INTEGER:
				val, err := strconv.ParseInt(valuePayload, 10, 64)
				if err != nil {
					log.Fatal("Error parsing integer", err)
				}
				return val
			}
		case float64:
			switch valueType {
			case configpb.ParsedValue_FLOAT:
				return valuePayload
			case configpb.ParsedValue_INTEGER:
				return int64(valuePayload)
			case configpb.ParsedValue_STRING:
				return fmt.Sprintf("%f", valuePayload)
			}
		default:
			log.Fatal("Unexpected field when parsing JSON payload")
		}
	}

	if config.GetStaticValue() != nil {
		if config, ok := config.GetStaticValue().(*configpb.ValueConfig_StringValue); ok {
			return config.StringValue
		}
		if config, ok := config.GetStaticValue().(*configpb.ValueConfig_IntegerValue); ok {
			return config.IntegerValue
		}
		if config, ok := config.GetStaticValue().(*configpb.ValueConfig_FloatValue); ok {
			return config.FloatValue
		}
	}

	return nil
}
