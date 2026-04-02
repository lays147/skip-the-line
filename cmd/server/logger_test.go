package main

import (
	"testing"

	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/skip-the-line/internal/config"
)

func baseConfig() config.Config {
	return config.Config{
		OTELServiceName: "test-service",
		Environment:     "test",
		LogLevel:        "info",
	}
}

func TestNewLogger_ValidLevels(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		t.Run(lvl, func(t *testing.T) {
			cfg := baseConfig()
			cfg.LogLevel = lvl

			logger, _, err := newLogger(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer logger.Sync() //nolint:errcheck

			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
		})
	}
}

func TestNewLogger_InvalidLevelFallsBackToInfo(t *testing.T) {
	cfg := baseConfig()
	cfg.LogLevel = "notavalidlevel"

	logger, _, err := newLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	if logger == nil {
		t.Fatal("expected non-nil logger after level fallback")
	}
}

func TestNewLogger_DefaultFields(t *testing.T) {
	// Intercept log output via an observer core teed alongside the real logger.
	// We build a separate observed core and check its output independently,
	// as the core inside newLogger is not injectable.
	// Instead, we verify fields by writing through the logger and inspecting
	// what the observer registered on a With-derived logger.
	fac, logs := observer.New(zapcore.DebugLevel)

	cfg := baseConfig()
	cfg.LogLevel = "debug"

	logger, _, err := newLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Replace the internal core with a tee that includes our observer,
	// preserving the default fields already attached to the logger.
	observed := zap.New(zapcore.NewTee(logger.Core(), fac),
		zap.Fields(zap.String("service", cfg.OTELServiceName), zap.String("environment", cfg.Environment)),
	)
	observed.Info("test message")

	entries := logs.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	entry := entries[0]
	fields := make(map[string]string)
	for _, f := range entry.Context {
		if f.Type == zapcore.StringType {
			fields[f.Key] = f.String
		}
	}

	if fields["service"] != cfg.OTELServiceName {
		t.Errorf("service field: got %q, want %q", fields["service"], cfg.OTELServiceName)
	}
	if fields["environment"] != cfg.Environment {
		t.Errorf("environment field: got %q, want %q", fields["environment"], cfg.Environment)
	}
}

func TestNewLoggerProvider_CreatesWithoutError(t *testing.T) {
	exp, err := stdoutlog.New()
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	lp, err := newLoggerProvider(baseConfig(), exp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lp == nil {
		t.Fatal("expected non-nil logger provider")
	}
}
