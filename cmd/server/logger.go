package main

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/skip-the-line/internal/config"
)

// newLogger builds a production zap.Logger that tees JSON output to stdout and
// an OTel log bridge. Log level and service metadata are sourced from cfg.
// The OTel core enriches log records with the active trace/span context when
// available. Callers are responsible for calling logger.Sync() and
// lp.Shutdown() on shutdown.
func newLogger(cfg config.Config) (*zap.Logger, *sdklog.LoggerProvider, error) {
	level, err := zapcore.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zapcore.InfoLevel
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "timestamp"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	baseCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.Lock(os.Stdout),
		zap.NewAtomicLevelAt(level),
	)

	stdoutExp, err := stdoutlog.New()
	if err != nil {
		return nil, nil, fmt.Errorf("init stdout log exporter: %w", err)
	}

	lp, err := newLoggerProvider(cfg, stdoutExp)
	if err != nil {
		return nil, nil, fmt.Errorf("init otel logger provider: %w", err)
	}

	otelCore := otelzap.NewCore(cfg.OTELServiceName, otelzap.WithLoggerProvider(lp))

	logger := zap.New(
		zapcore.NewTee(baseCore, otelCore),
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(
			zap.String("service", cfg.OTELServiceName),
			zap.String("environment", cfg.Environment),
		),
	)

	return logger, lp, nil
}

// newLoggerProvider initialises an OTel LoggerProvider with the given exporter,
// tagging the resource with cfg.OTELServiceName and cfg.Environment.
func newLoggerProvider(cfg config.Config, exp sdklog.Exporter) (*sdklog.LoggerProvider, error) {
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.OTELServiceName),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)),
	), nil
}
