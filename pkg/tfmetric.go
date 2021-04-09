package tfmetric

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
)

type MetricInstance struct {
	// The kind of metric to use.
	metricKind metric.InstrumentKind
	// The value of the metric.
	metricValue interface{}
	// Sets of key-value pairs to use as labels.
	labels []attribute.KeyValue
}

type MetricContext int

const (
	UnknownContext MetricContext = iota
	InputContext
	OutputContext
)

func CreateMetrics(config *configpb.SidecarConfig, payload string, context MetricContext) []MetricInstance {
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

	var output []MetricInstance
	for _, config := range metricConfigs {
		kind := getMetricKind(config)
		value := getMetricValue(config, jsonPayload)
		labels := getMetricLabels(config, jsonPayload)
		output = append(output, MetricInstance{kind, value, labels})
	}

	return output
}

func getMetricKind(config *configpb.MetricConfig) metric.InstrumentKind {
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
		return 1
	}
	if config.GetValue() != nil {
		value := config.GetValue().GetValue()
		return extractValue(value, jsonPayload)
	}
	// Default to a counter
	return 1
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
