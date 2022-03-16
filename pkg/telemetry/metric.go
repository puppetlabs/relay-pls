package telemetry

import (
	"github.com/google/wire"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

var ServerSet = wire.NewSet(
	NewTelemetryServer,
)

var ProviderSet = wire.NewSet(
	ProvidePrometheusConfig,
	ProvidePrometheusExporter,
	ProvideMeter,
)

func ProvidePrometheusConfig() prometheus.Config {
	return prometheus.Config{
		DefaultHistogramBoundaries: []float64{
			0, 5, 10, 20, 30, 45, 60, 120, 300, 600, 1800, 3600,
		},
	}
}

func ProvidePrometheusExporter(config prometheus.Config) (*prometheus.Exporter, error) {
	ctrl := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries),
			),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
	)
	return prometheus.New(config, ctrl)
}

func ProvideMeter(exporter *prometheus.Exporter) *metric.Meter {
	meter := exporter.MeterProvider().Meter("relay-pls")

	global.SetMeterProvider(exporter.MeterProvider())

	return &meter
}
