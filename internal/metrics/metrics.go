package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/skip-the-line/internal/config"
)

const meterName = "github.com/skip-the-line"

// Metrics holds all application metric instruments.
type Metrics struct {
	eventsCounter        metric.Int64Counter
	mergeHistogram       metric.Float64Histogram
	deliveriesCounter    metric.Int64Counter
	slackLookupHistogram metric.Float64Histogram
	slackSendHistogram   metric.Float64Histogram
	teamMembersHistogram metric.Float64Histogram
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

	deliveries, err := meter.Int64Counter(
		"notification_deliveries_total",
		metric.WithDescription("Total Slack DM delivery attempts, labelled by event_type and outcome"),
	)
	if err != nil {
		return nil, fmt.Errorf("create notification_deliveries_total counter: %w", err)
	}

	slackLookup, err := meter.Float64Histogram(
		"slack_lookup_duration_seconds",
		metric.WithDescription("Latency of Slack LookupUserByEmail calls, labelled by outcome"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create slack_lookup_duration_seconds histogram: %w", err)
	}

	slackSend, err := meter.Float64Histogram(
		"slack_send_duration_seconds",
		metric.WithDescription("Latency of Slack SendDM calls, labelled by outcome"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create slack_send_duration_seconds histogram: %w", err)
	}

	teamMembers, err := meter.Float64Histogram(
		"github_team_members_duration_seconds",
		metric.WithDescription("Latency of GitHub GetTeamMembers calls, labelled by outcome"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create github_team_members_duration_seconds histogram: %w", err)
	}

	return &Metrics{
		eventsCounter:        counter,
		mergeHistogram:       histogram,
		deliveriesCounter:    deliveries,
		slackLookupHistogram: slackLookup,
		slackSendHistogram:   slackSend,
		teamMembersHistogram: teamMembers,
	}, nil
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

// Outcome values used as labels on latency and delivery metrics.
const (
	OutcomeOK                = "ok"
	OutcomeSlackLookupFailed = "slack_lookup_failed"
	OutcomeSlackSendFailed   = "slack_send_failed"
	OutcomeError             = "error"
)

// RecordNotificationDelivery increments the notification_deliveries_total counter.
// outcome must be one of the Outcome* constants.
func (m *Metrics) RecordNotificationDelivery(ctx context.Context, eventType, outcome string) {
	m.deliveriesCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("event_type", eventType),
		attribute.String("outcome", outcome),
	))
}

// RecordSlackLookupDuration records the latency of a LookupUserByEmail call.
func (m *Metrics) RecordSlackLookupDuration(ctx context.Context, d time.Duration, outcome string) {
	m.slackLookupHistogram.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("outcome", outcome),
	))
}

// RecordSlackSendDuration records the latency of a SendDM call.
func (m *Metrics) RecordSlackSendDuration(ctx context.Context, d time.Duration, outcome string) {
	m.slackSendHistogram.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("outcome", outcome),
	))
}

// RecordTeamMembersDuration records the latency of a GetTeamMembers call.
func (m *Metrics) RecordTeamMembersDuration(ctx context.Context, d time.Duration, outcome string) {
	m.teamMembersHistogram.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("outcome", outcome),
	))
}

// NewMeterProvider initialises an OTel MeterProvider with an OTLP gRPC exporter
// when cfg.OTELExporterOTLPEndpoint is non-empty, or a no-op reader otherwise.
func NewMeterProvider(cfg config.Config) (*sdkmetric.MeterProvider, error) {
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.OTEL.ServiceName),
			attribute.String("service.version", cfg.OTEL.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}

	if cfg.OTEL.ExporterEndpoint != "" {
		otlpExp, err := otlpmetricgrpc.New(
			context.Background(),
			otlpmetricgrpc.WithEndpoint(cfg.OTEL.ExporterEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("create otlp grpc exporter: %w", err)
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpExp)))
	}

	return sdkmetric.NewMeterProvider(opts...), nil
}
