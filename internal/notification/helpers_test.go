package notification_test

import (
	"github.com/skip-the-line/internal/metrics"
	"github.com/skip-the-line/internal/subscription"
	"go.opentelemetry.io/otel/metric/noop"
)

func noopMetrics() *metrics.Metrics {
	m, _ := metrics.New(noop.NewMeterProvider())
	return m
}

func strPtr(s string) *string { return &s }

var testSubs = subscription.Registry{
	"octocat":     "octocat@example.com",
	"reviewer1":   "reviewer1@example.com",
	"reviewer2":   "reviewer2@example.com",
	"teamMember1": "tm1@example.com",
	"teamMember2": "tm2@example.com",
	"author":      "author@example.com",
}
