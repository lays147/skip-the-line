package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/skip-the-line/internal/config"
)

// newTracerProvider initialises an OTel TracerProvider with an OTLP gRPC
// batch exporter when cfg.OTELExporterOTLPEndpoint is non-empty, or a
// NeverSample no-op otherwise (zero overhead, no data exported).
// Sets the global TracerProvider and W3C TraceContext propagator so that
// the otelzap bridge can correlate log records to active spans.
// Callers are responsible for calling tp.Shutdown() on shutdown.
func newTracerProvider(cfg config.Config) (*sdktrace.TracerProvider, error) {
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.OTEL.ServiceName),
			attribute.String("service.version", cfg.OTEL.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}

	if cfg.OTEL.ExporterEndpoint != "" {
		exp, err := otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithEndpoint(cfg.OTEL.ExporterEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("create otlp trace exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	} else {
		opts = append(opts, sdktrace.WithSampler(sdktrace.NeverSample()))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp, nil
}
