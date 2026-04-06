package main

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/skip-the-line/internal/config"
)

// newLogger builds a production zap.Logger that writes structured JSON to
// stdout. Log level and service metadata are sourced from cfg.
// Callers are responsible for calling logger.Sync() on shutdown.
func newLogger(cfg config.Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zapcore.InfoLevel
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "timestamp"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.Lock(os.Stdout),
		zap.NewAtomicLevelAt(level),
	)

	logger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(
			zap.String("service", cfg.OTEL.ServiceName),
			zap.String("environment", cfg.Environment),
			zap.String("version", cfg.OTEL.ServiceVersion),
		),
	)

	return logger, nil
}
