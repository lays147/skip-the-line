# zap-stdout-logging Bugfix Design

## Overview

The application uses `otelzap` to bridge zap logs into the OTel SDK log pipeline for
trace/span correlation enrichment. `metrics.NewLoggerProvider` wires the OTel logger provider
with an OTLP/gRPC exporter, so log records are exported to an OTel collector (or silently
dropped when the endpoint is unset) — never written to stdout.

The fix is to replace the OTLP/gRPC log exporter in `NewLoggerProvider` with
`go.opentelemetry.io/otel/exporters/stdout/stdoutlog`, so the full pipeline becomes:

```
zap → otelzap bridge → OTel SDK logger provider → stdout log exporter
```

The otelzap bridge remains the sole zap core. No `zapcore.NewTee` is needed. Only
`internal/metrics/metrics.go` changes; `cmd/server/main.go` wiring is unchanged.

## Glossary

- **Bug_Condition (C)**: The condition that triggers the bug — `NewLoggerProvider` configures
  the OTel logger provider with an OTLP/gRPC exporter (or no processor), so log records never
  reach stdout.
- **Property (P)**: The desired behavior — every log record emitted by the application is
  written to stdout via the pipeline: zap → otelzap bridge → OTel SDK logger provider →
  stdout log exporter.
- **Preservation**: The trace/span correlation enrichment via otelzap, the service name
  resource attribute, and the OTel metrics pipeline that must remain unchanged after the fix.
- **NewLoggerProvider**: The function in `internal/metrics/metrics.go` that initialises an OTel
  `sdk/log.LoggerProvider`; the only function that changes in this fix.
- **otelzap bridge**: `otelzap.NewCore(...)` in `cmd/server/main.go` — the sole zap core,
  forwarding all log records into the OTel SDK log pipeline.
- **stdoutlog exporter**: `go.opentelemetry.io/otel/exporters/stdout/stdoutlog` — the
  replacement exporter that writes OTel log records to stdout.
- **NewMeterProvider**: The function in `internal/metrics/metrics.go` that initialises the OTel
  `MeterProvider` with OTLP/gRPC for metrics — completely untouched by this fix.

## Bug Details

### Bug Condition

The bug manifests inside `metrics.NewLoggerProvider`. When `OTELExporterOTLPEndpoint` is set,
the function attaches an OTLP/gRPC batch processor, routing all log records to the collector.
When the endpoint is unset, no processor is attached and all log records are silently dropped.
In neither case does the OTel logger provider write to stdout.

**Formal Specification:**
```
FUNCTION isBugCondition(cfg)
  INPUT: cfg of type config.Config
  OUTPUT: boolean

  loggerProvider := NewLoggerProvider(cfg)

  RETURN loggerProvider has NO stdout log exporter attached
         (i.e. log records are routed to OTLP/gRPC collector OR silently dropped,
          never written to stdout)
END FUNCTION
```

### Examples

- `OTELExporterOTLPEndpoint` is set → `logger.Info("server starting")` is exported to the
  OTel collector; expected: JSON log record written to stdout.
- `OTELExporterOTLPEndpoint` is empty → `logger.Info("server starting")` is silently dropped;
  expected: JSON log record written to stdout.
- Any log level, any message → no stdout output under either configuration; expected: stdout
  receives the record enriched with trace/span correlation fields.

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**
- Log records MUST continue to be enriched with trace and span correlation data via the
  otelzap bridge (`otelzap.NewCore` in `cmd/server/main.go`).
- `otelzap.NewCore` MUST remain the sole zap core — no `zapcore.NewTee` is introduced.
- `metrics.NewLoggerProvider` MUST continue to initialise the OTel logger provider with the
  service name resource attribute from `cfg.OTELServiceName`.
- OTel metrics (`NewMeterProvider`, OTLP/gRPC metric export, `WebhookEventsCounter`) MUST
  remain completely untouched.
- `cmd/server/main.go` wiring MUST remain unchanged — only `internal/metrics/metrics.go` is
  modified.
- Caller information (`zap.AddCaller()`) and error-level stack traces
  (`zap.AddStacktrace(zapcore.ErrorLevel)`) MUST continue to appear in log output.

**Scope:**
All behaviors unrelated to the log exporter in `NewLoggerProvider` are unaffected. This
includes HTTP routing, webhook signature validation, Slack DM delivery, OTel metric
instrumentation, and graceful shutdown.

## Hypothesized Root Cause

The root cause is in `metrics.NewLoggerProvider` (`internal/metrics/metrics.go`):

1. **OTLP/gRPC exporter used for logs**: When `OTELExporterOTLPEndpoint` is set, the function
   creates an `otlploggrpc` exporter and attaches it as a batch processor. Log records go to
   the collector, not stdout.

2. **No processor when endpoint is unset**: When `OTELExporterOTLPEndpoint` is empty, no
   processor is added to the logger provider. The OTel SDK discards all log records silently.

3. **stdout exporter never wired**: The `go.opentelemetry.io/otel/exporters/stdout/stdoutlog`
   package is not used anywhere. There is no code path that writes log records to stdout via
   the OTel pipeline.

The fix is straightforward: replace the conditional OTLP/gRPC exporter logic with an
unconditional `stdoutlog` exporter, always attached as a simple (or batch) processor.

## Correctness Properties

Property 1: Bug Condition - OTel Logger Provider Writes to Stdout

_For any_ configuration (whether `OTELExporterOTLPEndpoint` is set or unset), the fixed
`NewLoggerProvider` SHALL configure the OTel logger provider with a stdout log exporter so
that all log records emitted via the otelzap bridge are written to stdout.

**Validates: Requirements 2.1, 2.2, 2.3**

Property 2: Preservation - Trace Correlation and Metrics Pipeline Unchanged

_For any_ log record emitted where the bug condition does NOT hold (i.e. the logger provider
already writes to stdout), the fixed code SHALL continue to enrich records with trace/span
correlation via the otelzap bridge, retain the service name resource attribute, and leave the
OTel metrics pipeline (`NewMeterProvider`, OTLP/gRPC) completely unchanged.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5**

## Fix Implementation

### Changes Required

**File**: `internal/metrics/metrics.go`

**Function**: `NewLoggerProvider`

**Specific Changes**:

1. **Remove OTLP/gRPC log exporter import and logic**: Delete the
   `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` import and the conditional
   block that creates and attaches the `otlploggrpc` exporter.

2. **Add stdout log exporter**: Import
   `go.opentelemetry.io/otel/exporters/stdout/stdoutlog` and unconditionally create a
   `stdoutlog.New()` exporter.

3. **Attach stdout exporter as processor**: Add the stdout exporter to the logger provider
   options via `sdklog.WithProcessor(sdklog.NewSimpleProcessor(stdoutExp))` (or
   `NewBatchProcessor` — simple is sufficient for stdout).

4. **No other changes**: The resource construction, `cfg.OTELServiceName` attribute, and
   return value are unchanged.

**Sketch of the fixed `NewLoggerProvider`:**
```go
func NewLoggerProvider(cfg config.Config) (*sdklog.LoggerProvider, error) {
    res, err := resource.New(
        context.Background(),
        resource.WithAttributes(attribute.String("service.name", cfg.OTELServiceName)),
    )
    if err != nil {
        return nil, fmt.Errorf("create otel resource: %w", err)
    }

    stdoutExp, err := stdoutlog.New()
    if err != nil {
        return nil, fmt.Errorf("create stdout log exporter: %w", err)
    }

    return sdklog.NewLoggerProvider(
        sdklog.WithResource(res),
        sdklog.WithProcessor(sdklog.NewSimpleProcessor(stdoutExp)),
    ), nil
}
```

**File**: `cmd/server/main.go`

No changes required. The `zapcore.NewTee(baseCore, otelCore)` wiring and `otelzap.NewCore`
call remain exactly as they are today.

## Testing Strategy

### Validation Approach

Two-phase approach: first run exploratory tests against the unfixed code to surface
counterexamples and confirm the root cause, then verify the fix and preservation properties.

### Exploratory Bug Condition Checking

**Goal**: Surface counterexamples that demonstrate the bug on the UNFIXED code. Confirm that
`NewLoggerProvider` never writes to stdout under either endpoint configuration.

**Test Plan**: Construct the logger exactly as `main.go` does today — call
`metrics.NewLoggerProvider(cfg)`, build `otelzap.NewCore` with the returned provider, and
construct `zap.New(otelCore, ...)`. Redirect stdout to a buffer, emit a log record, and assert
the buffer is non-empty. Run on unfixed code — expect the assertion to fail, confirming the bug.

**Test Cases**:
1. **Endpoint set — logs go to collector**: Construct logger with `OTELExporterOTLPEndpoint`
   set, emit `logger.Info("test")`, assert stdout buffer is non-empty. (will fail on unfixed
   code — records go to collector)
2. **Endpoint unset — logs dropped**: Construct logger with `OTELExporterOTLPEndpoint` empty,
   emit `logger.Info("test")`, assert stdout buffer is non-empty. (will fail on unfixed code —
   records are silently dropped)

**Expected Counterexamples**:
- Both tests fail: stdout buffer is empty regardless of endpoint configuration.
- This confirms the root cause is in `NewLoggerProvider`, not in the `main.go` wiring.

### Fix Checking

**Goal**: Verify that for all inputs where the bug condition holds, the fixed `NewLoggerProvider`
causes log records to appear on stdout.

**Pseudocode:**
```
FOR ALL cfg WHERE isBugCondition(cfg) DO
  lp := NewLoggerProvider_fixed(cfg)
  logger := zap.New(otelzap.NewCore("...", otelzap.WithLoggerProvider(lp)), ...)
  logger.Info("test message")
  ASSERT stdout contains "test message"
END FOR
```

### Preservation Checking

**Goal**: Verify that for all inputs where the bug condition does NOT hold, the fixed code
continues to enrich records with trace/span correlation and leaves the metrics pipeline intact.

**Pseudocode:**
```
FOR ALL logRecord WHERE NOT isBugCondition(cfg) DO
  ASSERT logRecord contains trace_id and span_id fields (otelzap enrichment preserved)
  ASSERT NewMeterProvider behavior is identical before and after fix
END FOR
```

**Testing Approach**: Property-based testing is well-suited here because it generates many log
records across levels, messages, and field combinations automatically, catching edge cases that
manual tests miss and providing strong guarantees that trace correlation is preserved.

**Test Cases**:
1. **Trace correlation preservation**: For any log record emitted inside an active OTel span,
   assert the stdout output contains `trace_id` and `span_id` fields.
2. **Service name resource preservation**: Assert the logger provider resource still carries
   `service.name = cfg.OTELServiceName` after the fix.
3. **Metrics pipeline preservation**: Assert `NewMeterProvider` is unmodified and the
   `webhook_events_total` counter still increments correctly.
4. **Caller and stack trace preservation**: For any log record, assert `caller` is present;
   for error-level records, assert `stacktrace` is present.

### Unit Tests

- Test that the fixed `NewLoggerProvider` returns a provider whose stdout exporter receives
  log records (endpoint set and unset).
- Test that `NewMeterProvider` is unaffected by the change.
- Test that caller information and error-level stack traces are present in stdout output.

### Property-Based Tests

- Generate random log messages and field combinations; verify stdout output is non-empty for
  every record after the fix (Property 1).
- Generate random log records inside active OTel spans; verify `trace_id` and `span_id` appear
  in stdout output, confirming otelzap enrichment is preserved (Property 2).
- Generate random `cfg.OTELServiceName` values; verify the resource attribute is present in
  the logger provider after the fix.

### Integration Tests

- Construct the full logger as `main.go` does (call `NewLoggerProvider`, build `otelzap.NewCore`,
  construct `zap.New`), emit a log record, and assert stdout contains the record — confirming
  the end-to-end pipeline works after the fix.
- Verify `NewMeterProvider` and `WebhookEventsCounter` still function correctly after the fix,
  confirming the metrics pipeline is untouched.
