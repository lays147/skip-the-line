# Implementation Plan: github-webhook-notifier (skip-the-line)

## Overview

Implement a Go webhook receiver that listens for GitHub pull request events and delivers targeted Slack DMs. Tasks are ordered by dependency: scaffolding → core packages → handler → clients → service → observability → tests → containerization → wiring.

## Tasks

- [x] 1. Project scaffolding
  - Create `go.mod` with module name `github.com/skip-the-line` and Go 1.26.1
  - Add dependencies: `google/go-github/v62`, `slack-go/slack`, `go-chi/chi/v5`, `caarlos0/env`, `matryer/moq`, `go.uber.org/zap`, `go.opentelemetry.io/otel`, `gopkg.in/yaml.v3`
  - Create directory tree: `cmd/server/`, `internal/webhook/`, `internal/notification/`, `internal/github/`, `internal/slack/`, `internal/subscription/`, `internal/health/`, `internal/metrics/`, `internal/mocks/`
  - Create `Makefile` with targets: `up`, `down`, `logs`, `test`, `test-cover`, `generate`, `build`, `lint`
  - Create `.gitignore` covering Go binaries, `dist/`, `coverage.out`, `coverage.html`, IDE files, `.env`
  - Create `.editorconfig` with project-wide formatting rules (indent_style, indent_size, end_of_line, charset, trim_trailing_whitespace, insert_final_newline)
  - Create `README.md` covering prerequisites, quickstart, environment variables table, subscription config, Makefile targets, and Kubernetes probe docs
  - Create `subscriptions.yaml` with example entries (`github_username`, `email` fields)
  - _Requirements: 3.1, 3.2, 8.1, 15.1_

- [x] 2. Config package
  - [x] 2.1 Implement `internal/config/config.go` with `Config` struct using `caarlos0/env` tags
    - Fields: `GitHubWebhookSecret` (required), `GitHubToken` (required), `SlackBotToken` (required), `Port` (default `8080`), `LogEnv` (default `prod`), `OTELExporterOTLPEndpoint`, `OTELServiceName` (default `github-webhook-notifier`)
    - Export a `Load() (Config, error)` function that calls `env.Parse`
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 12.6_

  - [x] 2.2 Write unit tests for `Config` loading
    - Test that missing required variables return an error
    - Test that optional variables use their defaults
    - _Requirements: 8.3, 8.4_

- [x] 3. Subscription loader
  - [x] 3.1 Implement `internal/subscription/loader.go`
    - Define `Subscription` struct with `GitHubUsername string \`yaml:"github_username"\`` and `Email string \`yaml:"email"\``
    - Embed `subscriptions.yaml` via `//go:embed subscriptions.yaml` and expose `Load() ([]Subscription, error)`
    - Return a fatal-worthy error if the file is missing or unparseable
    - _Requirements: 3.1, 3.2, 3.3, 3.4_

  - [x] 3.2 Write unit tests for subscription loader
    - Test successful YAML parse returning correct slice
    - Test malformed YAML returns error
    - _Requirements: 3.1, 3.3_

- [x] 4. Notification interfaces and mock directives
  - [x] 4.1 Implement `internal/notification/recipients.go`
    - Define `GitHubTeamResolver` interface with only `GetTeamMembers(ctx context.Context, org, team string) ([]string, error)`
    - Define `SlackNotifier` interface (`SendDM`, `LookupUserByEmail`)
    - Add `//go:generate moq` directives for both interfaces targeting `../mocks/`
    - _Requirements: 4.1, 5.1_

  - [x] 4.2 Implement `internal/notification/service.go`
    - Define `NotificationServicer` interface with `Notify(ctx context.Context, eventType string, event any) error`
    - The `event` parameter accepts typed SDK structs: `*github.PullRequestEvent`, `*github.PullRequestReviewEvent`, `*github.PullRequestReviewCommentEvent`
    - Add `//go:generate moq` directive for `NotificationServicer` targeting `../mocks/`
    - _Requirements: 6.1, 6.2, 6.3_

- [x] 5. GitHub client
  - [x] 5.1 Implement `internal/github/client.go`
    - Define `Client` struct wrapping `go-github` SDK, authenticated with `GitHubToken`
    - Implement `GetTeamMembers(ctx, org, team string) ([]string, error)` — list all members of a GitHub team
    - Wrap errors with `fmt.Errorf("...: %w", err)`
    - _Requirements: 4.1, 4.2, 4.3_

- [x] 6. Slack client
  - [x] 6.1 Implement `internal/slack/client.go`
    - Define `Client` struct wrapping `slack-go/slack`, authenticated with `SlackBotToken`
    - Implement `LookupUserByEmail(ctx, email string) (string, error)` — returns Slack user ID
    - Implement `SendDM(ctx, email, message string) error` — opens a DM channel and posts a message
    - Wrap errors with `fmt.Errorf("...: %w", err)`
    - _Requirements: 5.1, 5.2, 5.3, 6.1_

- [x] 7. Mock generation
  - Run `go generate ./...` to produce mocks in `internal/mocks/` for `GitHubTeamResolver`, `SlackNotifier`, and `NotificationServicer`
  - Commit generated mock files so CI does not require a `go generate` step
  - _Requirements: (supports all unit test tasks)_

- [ ] 8. Notification service implementation
  - [x] 8.1 Implement `NotificationService` struct in `internal/notification/service.go`
    - Constructor `NewNotificationService(resolver GitHubTeamResolver, notifier SlackNotifier, subs []subscription.Subscription) *NotificationService`
    - Implement `Notify(ctx, eventType string, event any) error` using a type switch on `event`:
      - `*github.PullRequestEvent` with action `review_requested`: expand team reviewers via `GetTeamMembers`, collect individual reviewer usernames, deduplicate GitHub usernames before Slack lookup, exclude PR author, look up each subscriber's email in `subs`, call `SlackNotifier.SendDM`
      - `*github.PullRequestReviewEvent` with action `submitted`: notify PR author (exclude reviewer themselves)
      - `*github.PullRequestReviewCommentEvent`: notify PR author and any subscribers mentioned in the comment
    - Return HTTP 200 no-op for unrecognised event type/action combinations
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 7.1, 7.2_

  - [x] 8.2 Write unit tests for `NotificationService.Notify` — `pull_request review_requested`
    - Use `mocks.GitHubTeamResolverMock` and `mocks.SlackNotifierMock`
    - Table-driven: single reviewer, team reviewer expanded to members, author excluded, duplicate recipients deduplicated
    - Pass `*github.PullRequestEvent` directly to `Notify`
    - _Requirements: 6.2, 7.1, 7.2_

  - [x] 8.3 Write unit tests for `NotificationService.Notify` — `pull_request_review submitted`
    - Table-driven: reviewer is not notified, author is notified, Slack error is logged and skipped
    - Pass `*github.PullRequestReviewEvent` directly to `Notify`
    - _Requirements: 6.5, 5.3_

  - [x] 8.4 Write unit tests for `NotificationService.Notify` — `pull_request_review_comment`
    - Table-driven: author notified, mentioned subscribers notified, duplicates deduplicated
    - Pass `*github.PullRequestReviewCommentEvent` directly to `Notify`
    - _Requirements: 6.6, 7.2_

- [ ] 9. Webhook handler
  - [x] 9.1 Implement `internal/webhook/handler.go`
    - Define `Handler` struct with `NotificationServicer`, `webhookSecret`, and OTel counter fields
    - Implement `ServeHTTP` for `POST /webhook`:
      - Validate `X-Hub-Signature-256` using `github.ValidatePayload(r, []byte(secret))`; return HTTP 401 on failure
      - Read `X-GitHub-Event` header for the event type
      - Call `github.ParseWebHook(eventType, body)` to obtain a typed SDK struct; return HTTP 400 if the event type is unrecognised or body is unparseable
      - Dispatch to `NotificationServicer.Notify(ctx, eventType, typedEvent)`; return HTTP 500 on error
      - Return HTTP 200 no-op for unsupported event types (log at debug)
    - Return JSON error responses with `Content-Type: application/json` on all error paths
    - Increment `webhook_events_total` OTel counter with `event_type` label after successful validation
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.3, 10.1, 10.2, 12.1, 12.2, 12.3_

  - [x] 9.2 Write unit tests for webhook handler
    - Table-driven: valid signature dispatches to service, invalid signature returns 401, bad body returns 400, service error returns 500, unsupported event returns 200
    - Use `mocks.NotificationServicerMock`
    - _Requirements: 1.2, 1.3, 1.4, 2.1, 2.2, 10.1, 10.2_

- [ ] 10. Health handler
  - [x] 10.1 Implement `internal/health/handler.go`
    - Define `Handler` struct with an `atomic.Bool` ready flag
    - `GET /healthz` always returns HTTP 200 `{"status":"ok"}`
    - `GET /readyz` returns HTTP 200 `{"status":"ready"}` when ready flag is set, HTTP 503 `{"status":"not ready"}` otherwise
    - Export `SetReady(bool)` to flip the flag after all dependencies are initialised
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5_

  - [x] 10.2 Write unit tests for health handler
    - Test `/healthz` always 200
    - Test `/readyz` 503 before `SetReady(true)`, 200 after
    - _Requirements: 13.2, 13.4, 13.5_

- [ ] 11. OpenTelemetry metrics setup
  - [x] 11.1 Implement `internal/metrics/metrics.go`
    - Initialize OTel SDK with service name from `Config.OTELServiceName`
    - Register a `webhook_events_total` counter instrument with `event_type` attribute
    - Expose a Prometheus-compatible `/metrics` HTTP handler using `go.opentelemetry.io/otel/exporters/prometheus`
    - When `Config.OTELExporterOTLPEndpoint` is non-empty, additionally configure an OTLP gRPC exporter
    - Export `NewMeterProvider(cfg Config) (*sdkmetric.MeterProvider, http.Handler, error)` and `WebhookEventsCounter(mp *sdkmetric.MeterProvider) metric.Int64Counter`
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6_

- [x] 12. Checkpoint — core packages complete
  - Ensure `go build ./...` succeeds and all existing tests pass with `go test ./...`
  - Ask the user if any questions arise before proceeding to containerization and wiring.

- [x] 13. Dockerfile
  - Create `Dockerfile` with a multi-stage build
    - Builder stage: `golang:1.26.1-alpine`, copies source, runs `CGO_ENABLED=0 go build -o /skip-the-line ./cmd/server`
    - Final stage: `gcr.io/distroless/static:nonroot`, copies binary only
    - `EXPOSE 8080`
    - `USER nonroot:nonroot` (distroless nonroot UID 65532)
    - `ENTRYPOINT ["/skip-the-line"]`
  - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.5_

- [x] 14. Docker Compose
  - Create `docker-compose.yml` defining four services on a shared `skip-the-line` network:
    - `app`: built from local `Dockerfile`, port `8080:8080`, env vars wired to mock services
    - `mock-slack`: lightweight HTTP server (e.g., `mockserver` or a minimal Go binary in `tools/mock-slack/`) that accepts Slack API calls and logs payloads
    - `mock-github`: not a live server — a one-shot `webhook-sender` container that replays sample payloads from `testdata/` to `app:8080/webhook` using `curl` or a small Go script
    - `webhook-sender`: sends sample webhook payloads with correct HMAC signatures to the app
  - Set environment variable defaults sufficient to run in the mock environment
  - _Requirements: 15.1, 15.2, 15.3, 15.4, 15.5, 15.6_

- [ ] 15. Main wiring (`cmd/server/main.go`)
  - [x] 15.1 Implement `cmd/server/main.go`
    - Call `config.Load()`, fatal-log on error
    - Initialise `zap` logger (dev or prod mode based on `Config.LogEnv`)
    - Call `subscription.Load()`, fatal-log on error
    - Initialise OTel meter provider and `webhook_events_total` counter
    - Construct `github.Client`, `slack.Client`, `notification.NotificationService`
    - Construct `webhook.Handler` and `health.Handler`
    - Register routes using `chi.NewRouter()`: `r.Post("/webhook", ...)`, `r.Get("/healthz", ...)`, `r.Get("/readyz", ...)`, `r.Get("/metrics", ...)`
    - Call `health.SetReady(true)` after all dependencies are initialised
    - Listen for OS signals using `signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)`
    - On signal received, call `server.Shutdown` with a `context.WithTimeout` of 30 seconds
    - Force-exit with a warning log if the shutdown timeout is exceeded
    - _Requirements: 1.1, 8.1, 8.4, 9.1, 9.2, 9.3, 11.1, 11.2, 11.3, 11.4, 11.5, 13.4_

- [x] 16. Final checkpoint — Ensure all tests pass
  - Run `go test ./...` and confirm all tests pass.
  - Run `go build ./...` and confirm the binary compiles.
  - Ask the user if any questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for a faster MVP
- Each task references specific requirements for traceability
- Mocks must be generated (task 7) before writing any unit tests
- Property tests are not applicable here as the design document does not define correctness properties; unit tests with table-driven cases cover the equivalent ground
