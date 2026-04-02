package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/skip-the-line/internal/config"
)

const meterName = "github.com/skip-the-line"

// Metrics holds all application metric instruments.
type Metrics struct {
	eventsCounter  metric.Int64Counter
	mergeHistogram metric.Float64Histogram
}

// New registers all application metric instruments against mp.
// Accepts the metric.MeterProvider interface so both the real SDK provider
// and noop providers (in tests) can be passed.
func New(mp metric.MeterProvider) (*Metrics, error) {
	meter := mp.Meter(meterName)

	counter, err := meter.Int64Counter(
		"webhook_events_total",
		metric.WithDescription("Total number of webhook events received, labelled by event_type"),
	)
	if err != nil {
		return nil, fmt.Errorf("create webhook_events_total counter: %w", err)
	}

	histogram, err := meter.Float64Histogram(
		"pr_merge_duration_seconds",
		metric.WithDescription("Time from PR opened to merged, in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create pr_merge_duration_seconds histogram: %w", err)
	}

	return &Metrics{eventsCounter: counter, mergeHistogram: histogram}, nil
}

// RecordWebhookEvent increments the webhook_events_total counter for the given event type.
func (m *Metrics) RecordWebhookEvent(ctx context.Context, eventType string) {
	m.eventsCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("event_type", eventType),
	))
}

// RecordPRMergeDuration records how long a PR took from open to merge.
// authorSubscribed indicates whether the PR author is in the subscription registry.
func (m *Metrics) RecordPRMergeDuration(ctx context.Context, openedAt, mergedAt time.Time, authorSubscribed bool) {
	m.mergeHistogram.Record(ctx, mergedAt.Sub(openedAt).Seconds(), metric.WithAttributes(
		attribute.Bool("subscribed", authorSubscribed),
	))
}

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
