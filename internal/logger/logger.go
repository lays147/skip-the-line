package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// FromContext enriches base with trace_id and span_id fields when ctx carries
// an active recording span. Use this at the start of any function that has
// both a context and something worth logging, so that stdout JSON log lines
// are correlated with traces without requiring a context-aware logger type.
func FromContext(ctx context.Context, base *zap.Logger) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return base
	}
	sc := span.SpanContext()
	return base.With(
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
	)
}
