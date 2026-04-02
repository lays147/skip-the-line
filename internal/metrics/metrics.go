package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/skip-the-line/internal/config"
)

// NewMeterProvider initialises an OTel MeterProvider with an OTLP gRPC exporter
// when cfg.OTELExporterOTLPEndpoint is non-empty, or a no-op reader otherwise.
func NewMeterProvider(cfg config.Config) (*sdkmetric.MeterProvider, error) {
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(attribute.String("service.name", cfg.OTELServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}

	if cfg.OTELExporterOTLPEndpoint != "" {
		otlpExp, err := otlpmetricgrpc.New(
			context.Background(),
			otlpmetricgrpc.WithEndpoint(cfg.OTELExporterOTLPEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("create otlp grpc exporter: %w", err)
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpExp)))
	}

	return sdkmetric.NewMeterProvider(opts...), nil
}

// NewLoggerProvider initialises an OTel LoggerProvider with an OTLP gRPC exporter
// when cfg.OTELExporterOTLPEndpoint is non-empty, or a no-op batch processor otherwise.
func NewLoggerProvider(cfg config.Config) (*sdklog.LoggerProvider, error) {
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(attribute.String("service.name", cfg.OTELServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	opts := []sdklog.LoggerProviderOption{
		sdklog.WithResource(res),
	}

	if cfg.OTELExporterOTLPEndpoint != "" {
		otlpExp, err := otlploggrpc.New(
			context.Background(),
			otlploggrpc.WithEndpoint(cfg.OTELExporterOTLPEndpoint),
			otlploggrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("create otlp grpc log exporter: %w", err)
		}
		opts = append(opts, sdklog.WithProcessor(sdklog.NewBatchProcessor(otlpExp)))
	}

	return sdklog.NewLoggerProvider(opts...), nil
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
