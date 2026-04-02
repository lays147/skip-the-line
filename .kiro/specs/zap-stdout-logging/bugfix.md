# Bugfix Requirements Document

## Introduction

The application uses `otelzap` to bridge zap logs into the OTel SDK log pipeline for trace/span
correlation enrichment. However, `metrics.NewLoggerProvider` wires the OTel logger provider with
an OTLP/gRPC exporter, so all log records are exported to an OTel collector instead of stdout.
The fix is to replace the OTLP/gRPC log exporter with a stdout log exporter in
`NewLoggerProvider`, so the full pipeline is: zap → otelzap bridge → OTel SDK logger provider →
stdout log exporter. No `zapcore.NewTee` is needed; the otelzap bridge is the sole zap core.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN `metrics.NewLoggerProvider` is called and `OTELExporterOTLPEndpoint` is set THEN the
system exports log records to the OTel collector via OTLP/gRPC instead of writing them to stdout
1.2 WHEN `metrics.NewLoggerProvider` is called and `OTELExporterOTLPEndpoint` is empty THEN the
system creates a logger provider with no processor, silently dropping all log records instead of
writing them to stdout
1.3 WHEN the application emits any log record THEN the system routes it through the otelzap
bridge to the OTel logger provider, which does not write to stdout under either configuration

### Expected Behavior (Correct)

2.1 WHEN `metrics.NewLoggerProvider` is called THEN the system SHALL configure the OTel logger
provider with a stdout log exporter so that all log records are written to stdout
2.2 WHEN the application emits any log record THEN the system SHALL write it to stdout via the
pipeline: zap → otelzap bridge → OTel SDK logger provider → stdout log exporter
2.3 WHEN `OTELExporterOTLPEndpoint` is set or unset THEN the system SHALL write log records to
stdout regardless, as the OTel logger provider always uses the stdout exporter for logs

### Unchanged Behavior (Regression Prevention)

3.1 WHEN the application emits any log record THEN the system SHALL CONTINUE TO enrich it with
trace and span correlation data via the otelzap bridge
3.2 WHEN the logger is initialised THEN the system SHALL CONTINUE TO use `otelzap.NewCore` as
the zap core, forwarding all records into the OTel SDK log pipeline
3.3 WHEN `metrics.NewLoggerProvider` is called THEN the system SHALL CONTINUE TO initialise the
OTel logger provider with the service name resource attribute from `cfg.OTELServiceName`
3.4 WHEN `OTELExporterOTLPEndpoint` is set THEN the system SHALL CONTINUE TO export metrics
(not logs) to the OTel collector via OTLP/gRPC through `NewMeterProvider` — this is unchanged
3.5 WHEN the application emits any log record THEN the system SHALL CONTINUE TO include caller
information and stack traces on error-level logs via `zap.AddCaller()` and
`zap.AddStacktrace(zapcore.ErrorLevel)`
