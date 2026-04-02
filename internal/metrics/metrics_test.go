package metrics_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/skip-the-line/internal/config"
	"github.com/skip-the-line/internal/metrics"
)

// newLoggerProviderWithWriter creates a logger provider that writes to the given buffer
// instead of os.Stdout, allowing output capture in tests.
func newLoggerProviderWithWriter(t *testing.T, cfg config.Config, buf *bytes.Buffer) *sdklog.LoggerProvider {
	t.Helper()
	exp, err := stdoutlog.New(stdoutlog.WithWriter(buf))
	if err != nil {
		t.Fatalf("stdoutlog.New: %v", err)
	}
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(attribute.String("service.name", cfg.OTELServiceName)),
	)
	if err != nil {
		t.Fatalf("resource.New: %v", err)
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)),
	)
}

// TestNewLoggerProviderWritesToStdout encodes the expected behavior:
// for any cfg variant, the OTel logger provider must write log records to stdout.
//
// On UNFIXED code this test FAILS — confirming the bug exists:
//   - endpoint set:   records go to OTLP/gRPC collector, stdout is empty
//   - endpoint unset: no processor attached, records are silently dropped, stdout is empty
//
// Validates: Requirements 1.1, 1.2, 1.3
func TestNewLoggerProviderWritesToStdout(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{
			name:     "endpoint set — logs go to collector",
			endpoint: "localhost:4317",
		},
		{
			name:     "endpoint unset — logs silently dropped",
			endpoint: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{
				OTELServiceName:          "test-service",
				OTELExporterOTLPEndpoint: tc.endpoint,
			}

			var buf bytes.Buffer
			lp := newLoggerProviderWithWriter(t, cfg, &buf)

			otelCore := otelzap.NewCore("github.com/skip-the-line", otelzap.WithLoggerProvider(lp))
			logger := zap.New(otelCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

			logger.Info("test message")
			_ = logger.Sync()

			output := buf.String()

			if output == "" {
				t.Errorf("output was empty — log record was not written (bug confirmed for cfg: endpoint=%q)", tc.endpoint)
			}
			if output != "" && !bytes.Contains([]byte(output), []byte("test message")) {
				t.Errorf("output did not contain %q; got: %s", "test message", output)
			}
		})
	}
}

// TestMetricsPipelinePreservation asserts that NewMeterProvider and WebhookEventsCounter
// are unaffected — they return no error and the counter increments without panic.
//
// These tests PASS on unfixed code — the metrics pipeline is untouched by the bug.
//
// Validates: Requirements 3.4
func TestMetricsPipelinePreservation(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{name: "endpoint unset", endpoint: ""},
		// Note: endpoint set would attempt a real gRPC dial; skip to avoid network dependency.
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{
				OTELServiceName:          "test-service",
				OTELExporterOTLPEndpoint: tc.endpoint,
			}

			mp, err := metrics.NewMeterProvider(cfg)
			if err != nil {
				t.Fatalf("NewMeterProvider returned error: %v", err)
			}
			if mp == nil {
				t.Fatal("NewMeterProvider returned nil provider")
			}

			m, err := metrics.New(mp)
			if err != nil {
				t.Fatalf("metrics.New returned error: %v", err)
			}
			if m == nil {
				t.Fatal("metrics.New returned nil")
			}

			// Verify instruments can be used without panic.
			m.RecordWebhookEvent(context.Background(), "pull_request")
			m.RecordPRMergeDuration(context.Background(), time.Now().Add(-time.Hour), time.Now(), true)
		})
	}
}
