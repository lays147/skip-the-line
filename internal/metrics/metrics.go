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

// PRMergeHistogram returns a Float64Histogram instrument named "pr_merge_duration_seconds"
// scoped to the given MeterProvider.
func PRMergeHistogram(mp *sdkmetric.MeterProvider) (metric.Float64Histogram, error) {
	meter := mp.Meter("github.com/skip-the-line")
	h, err := meter.Float64Histogram(
		"pr_merge_duration_seconds",
		metric.WithDescription("Time from PR opened to merged, in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create pr_merge_duration_seconds histogram: %w", err)
	}
	return h, nil
}

// RecordPRMergeDuration records a single observation on the PR merge histogram.
// openedAt and mergedAt are the PR creation and merge timestamps.
// authorSubscribed indicates whether the PR author is registered in the subscription registry.
func RecordPRMergeDuration(ctx context.Context, h metric.Float64Histogram, openedAt, mergedAt time.Time, authorSubscribed bool) {
	h.Record(ctx, mergedAt.Sub(openedAt).Seconds(), metric.WithAttributes(
		attribute.Bool("subscribed", authorSubscribed),
	))
}
