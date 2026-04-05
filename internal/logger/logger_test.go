package logger_test

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/skip-the-line/internal/logger"
)

func observedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return zap.New(core), logs
}

func TestFromContext_NoSpan(t *testing.T) {
	// Empty context carries no span — base logger returned unchanged.
	base, logs := observedLogger()

	l := logger.FromContext(context.Background(), base)
	l.Info("msg")

	for _, f := range logs.All()[0].Context {
		if f.Key == "trace_id" || f.Key == "span_id" {
			t.Errorf("unexpected field %q when no span in context", f.Key)
		}
	}
}

func TestFromContext_NonRecordingSpan(t *testing.T) {
	// A noop span is valid but not recording — base logger returned unchanged.
	base, logs := observedLogger()

	noopSpan := trace.SpanFromContext(context.Background()) // always a noop span
	ctx := trace.ContextWithSpan(context.Background(), noopSpan)

	l := logger.FromContext(ctx, base)
	l.Info("msg")

	for _, f := range logs.All()[0].Context {
		if f.Key == "trace_id" || f.Key == "span_id" {
			t.Errorf("unexpected field %q for non-recording span", f.Key)
		}
	}
}

func TestFromContext_RecordingSpan(t *testing.T) {
	// A real SDK span is recording — trace_id and span_id must be injected.
	base, logs := observedLogger()

	exp := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(exp))
	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	l := logger.FromContext(ctx, base)
	l.Info("msg")

	fields := make(map[string]string)
	for _, f := range logs.All()[0].Context {
		if f.Type == zapcore.StringType {
			fields[f.Key] = f.String
		}
	}

	sc := span.SpanContext()
	if fields["trace_id"] != sc.TraceID().String() {
		t.Errorf("trace_id: got %q, want %q", fields["trace_id"], sc.TraceID().String())
	}
	if fields["span_id"] != sc.SpanID().String() {
		t.Errorf("span_id: got %q, want %q", fields["span_id"], sc.SpanID().String())
	}
}
