package aws

import (
	"log"

	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

var (
	metricProvider *metric.MeterProvider
)

func init() {
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal(err)
	}
	metricProvider = metric.NewMeterProvider(metric.WithReader(exporter))
}
