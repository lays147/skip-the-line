package metrics

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/skip-the-line/internal/config"
)

// NewMeterProvider initialises an OTel MeterProvider with a Prometheus exporter
// and, when cfg.OTELExporterOTLPEndpoint is non-empty, an OTLP gRPC exporter.
// It returns the provider, a Prometheus-compatible HTTP handler, and any error.
func NewMeterProvider(cfg config.Config) (*sdkmetric.MeterProvider, http.Handler, error) {
	promExp, err := promexporter.New()
	if err != nil {
		return nil, nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(attribute.String("service.name", cfg.OTELServiceName)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create otel resource: %w", err)
	}

	opts := []sdkmetric.Option{
		sdkmetric.WithReader(promExp),
		sdkmetric.WithResource(res),
	}

	if cfg.OTELExporterOTLPEndpoint != "" {
		otlpExp, err := otlpmetricgrpc.New(
			context.Background(),
			otlpmetricgrpc.WithEndpoint(cfg.OTELExporterOTLPEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("create otlp grpc exporter: %w", err)
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpExp)))
	}

	mp := sdkmetric.NewMeterProvider(opts...)
	return mp, promhttp.Handler(), nil
}

// WebhookEventsCounter returns an Int64Counter instrument named "webhook_events_total"
// scoped to the given MeterProvider. Use the "event_type" attribute when recording.
func WebhookEventsCounter(mp *sdkmetric.MeterProvider) (metric.Int64Counter, error) {
	meter := mp.Meter("github.com/skip-the-line")
	counter, err := meter.Int64Counter(
		"webhook_events_total",
		metric.WithDescription("Total number of webhook events received, labelled by event_type"),
	)
	if err != nil {
		return nil, fmt.Errorf("create webhook_events_total counter: %w", err)
	}
	return counter, nil
}
