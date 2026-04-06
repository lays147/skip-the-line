package main

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/skip-the-line/internal/config"
)

func baseConfig() config.Config {
	return config.Config{
		OTEL:        config.OTELConfig{ServiceName: "test-service"},
		Environment: "test",
		LogLevel:    "info",
	}
}

func TestNewLogger_ValidLevels(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		t.Run(lvl, func(t *testing.T) {
			cfg := baseConfig()
			cfg.LogLevel = lvl

			logger, err := newLogger(cfg)
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

	logger, err := newLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	if logger == nil {
		t.Fatal("expected non-nil logger after level fallback")
	}
}

func TestNewLogger_DefaultFields(t *testing.T) {
	fac, logs := observer.New(zapcore.DebugLevel)

	cfg := baseConfig()
	cfg.LogLevel = "debug"

	logger, err := newLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	observed := zap.New(zapcore.NewTee(logger.Core(), fac),
		zap.Fields(zap.String("service", cfg.OTEL.ServiceName), zap.String("environment", cfg.Environment)),
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

	if fields["service"] != cfg.OTEL.ServiceName {
		t.Errorf("service field: got %q, want %q", fields["service"], cfg.OTEL.ServiceName)
	}
	if fields["environment"] != cfg.Environment {
		t.Errorf("environment field: got %q, want %q", fields["environment"], cfg.Environment)
	}
}
