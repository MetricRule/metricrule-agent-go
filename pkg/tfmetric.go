package tfmetric

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	configpb "github.com/metricrule-sidecar-tfserving/api/proto/metricconfigpb"
)

type MetricInstance struct {
	// The kind of metric to use.
	metric_kind metric.InstrumentKind
	// The value of the metric.
	metric_value interface{}
	// Sets of key-value pairs to use as labels.
	labels []attribute.KeyValue
}

func CreateMetrics(config *configpb.MetricConfig, payload string) []MetricInstance {
	return []MetricInstance{}
}
