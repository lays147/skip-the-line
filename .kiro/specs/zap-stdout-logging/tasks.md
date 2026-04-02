# Implementation Plan

- [x] 1. Write bug condition exploration test
  - **Property 1: Bug Condition** - OTel Logger Provider Never Writes to Stdout
  - **CRITICAL**: This test MUST FAIL on unfixed code — failure confirms the bug exists
  - **DO NOT attempt to fix the test or the code when it fails**
  - **NOTE**: This test encodes the expected behavior — it will validate the fix when it passes after implementation
  - **GOAL**: Surface counterexamples that demonstrate the bug exists
  - **Scoped PBT Approach**: Scope the property to the two concrete failing cases: endpoint set and endpoint unset
  - Create `internal/metrics/metrics_test.go` (package `metrics_test`)
  - For each cfg variant (OTELExporterOTLPEndpoint set, OTELExporterOTLPEndpoint empty):
    - Call `metrics.NewLoggerProvider(cfg)` to get the logger provider
    - Build `otelzap.NewCore("github.com/skip-the-line", otelzap.WithLoggerProvider(lp))`
    - Construct `zap.New(otelCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))`
    - Redirect os.Stdout to a pipe/buffer, emit `logger.Info("test message")`, call `logger.Sync()`
    - Assert the captured buffer is non-empty (contains "test message")
  - Run test on UNFIXED code: `make test`
  - **EXPECTED OUTCOME**: Test FAILS — stdout buffer is empty under both configurations (confirms bug)
  - Document counterexamples found (e.g. "endpoint set: buffer empty — records go to collector; endpoint unset: buffer empty — records silently dropped")
  - Mark task complete when test is written, run, and failure is documented
  - _Requirements: 1.1, 1.2, 1.3_

- [x] 2. Write preservation property tests (BEFORE implementing fix)
  - **Property 2: Preservation** - Trace Correlation, Service Name, and Metrics Pipeline Unchanged
  - **IMPORTANT**: Follow observation-first methodology — run UNFIXED code with non-buggy inputs first
  - Create preservation tests in `internal/metrics/metrics_test.go`
  - **Observe on UNFIXED code** (inputs where isBugCondition does NOT hold — i.e. the logger provider structure itself, not stdout output):
    - Observe: `NewLoggerProvider(cfg)` returns a provider whose resource carries `service.name = cfg.OTELServiceName`
    - Observe: `NewMeterProvider(cfg)` returns a provider and `WebhookEventsCounter` increments correctly
    - Observe: `zap.New(otelCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))` includes caller field in output
  - Write property-based tests capturing these observed behaviors:
    - For any `cfg.OTELServiceName` value, assert the logger provider resource attribute `service.name` equals the configured value
    - For any log record emitted inside an active OTel span, assert stdout output (post-fix) contains `trace_id` and `span_id` fields
    - Assert `NewMeterProvider` is unaffected: `WebhookEventsCounter` increments and `NewMeterProvider` returns no error
    - Assert caller info is present in log output; assert stacktrace is present for error-level records
  - Run tests on UNFIXED code: `make test`
  - **EXPECTED OUTCOME**: Tests PASS — confirms baseline preservation behavior before the fix
  - Mark task complete when tests are written, run, and passing on unfixed code
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 3. Fix NewLoggerProvider to use stdout log exporter

  - [x] 3.1 Implement the fix in `internal/metrics/metrics.go`
    - Remove the `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` import
    - Remove the conditional block that creates and attaches the `otlploggrpc` exporter
    - Add import `go.opentelemetry.io/otel/exporters/stdout/stdoutlog`
    - Unconditionally create `stdoutExp, err := stdoutlog.New()` after the resource construction
    - Attach via `sdklog.WithProcessor(sdklog.NewSimpleProcessor(stdoutExp))` — no conditional on endpoint
    - Keep resource construction, `cfg.OTELServiceName` attribute, and return value unchanged
    - `cmd/server/main.go` is NOT modified
    - _Bug_Condition: isBugCondition(cfg) — NewLoggerProvider has no stdout exporter attached (records go to OTLP/gRPC or are dropped)_
    - _Expected_Behavior: for any cfg, NewLoggerProvider returns a provider with a stdout SimpleProcessor so all log records are written to stdout_
    - _Preservation: service name resource attribute retained; NewMeterProvider/OTLP metrics pipeline untouched; cmd/server/main.go wiring unchanged_
    - _Requirements: 2.1, 2.2, 2.3, 3.3, 3.4_

  - [x] 3.2 Verify bug condition exploration test now passes
    - **Property 1: Expected Behavior** - OTel Logger Provider Writes to Stdout
    - **IMPORTANT**: Re-run the SAME test from task 1 — do NOT write a new test
    - Run `make test` — the test from task 1 now asserts stdout is non-empty
    - **EXPECTED OUTCOME**: Test PASSES — confirms the stdout exporter is wired and log records reach stdout
    - _Requirements: 2.1, 2.2, 2.3_

  - [x] 3.3 Verify preservation tests still pass
    - **Property 2: Preservation** - Trace Correlation, Service Name, and Metrics Pipeline Unchanged
    - **IMPORTANT**: Re-run the SAME tests from task 2 — do NOT write new tests
    - Run `make test`
    - **EXPECTED OUTCOME**: Tests PASS — confirms no regressions in trace correlation, service name resource, metrics pipeline, caller info, and stack traces
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 4. Checkpoint — Ensure all tests pass
  - Run `make test` and confirm all tests pass with no failures
  - Ensure `make build` succeeds (the `stdoutlog` package may need to be added to `go.mod` — run `go get go.opentelemetry.io/otel/exporters/stdout/stdoutlog` if needed)
  - Ask the user if any questions arise
