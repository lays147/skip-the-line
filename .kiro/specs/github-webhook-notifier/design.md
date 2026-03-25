# Design: github-webhook-notifier (skip-the-line)

## Overview

`skip-the-line` is a GitHub webhook receiver written in Go. It listens for pull request events from GitHub, resolves which Slack users should be notified based on team membership and subscription configuration, and delivers direct messages via the Slack API.

The service is designed to be deployed on Kubernetes and exposes health/readiness probes at `/healthz` and `/readyz`. All external dependencies (GitHub API, Slack API) are accessed through interfaces, enabling clean unit testing with generated mocks.

## Architecture

```
GitHub → POST /webhook
           │
           ▼
   Signature Validation
           │
           ▼
     Event Routing
    (PR opened/closed/
     review requested)
           │
           ▼
  Subscriber Resolution
  (subscriptions.yaml +
   GitHub team membership)
           │
           ▼
     Slack DM Delivery
```

The service is structured around a clean separation of concerns:

- **Transport layer**: HTTP server, webhook signature validation, request parsing
- **Business logic**: event routing, subscriber resolution, notification orchestration
- **External clients**: GitHub API client, Slack API client — both accessed only via interfaces

## Components and Interfaces

### Package Structure

```
skip-the-line/
├── README.md
├── Makefile
├── .gitignore
├── .editorconfig
├── cmd/
│   └── server/
│       └── main.go                # wires all components; chi router; signal-based graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go              # env var loading
│   ├── webhook/
│   │   ├── handler.go             # HTTP handler, signature validation
│   │   └── handler_test.go
│   ├── notification/
│   │   ├── recipients.go          # GitHubTeamResolver + SlackNotifier interfaces
│   │   ├── service.go             # NotificationServicer interface + implementation
│   │   └── service_test.go
│   ├── github/
│   │   └── client.go              # implements notification.GitHubTeamResolver
│   ├── slack/
│   │   └── client.go              # implements notification.SlackNotifier
│   ├── subscription/
│   │   └── loader.go              # loads subscriptions.yaml
│   └── mocks/
│       ├── mock_github_team_resolver.go   # GitHubTeamResolverMock
│       ├── mock_slack_notifier.go         # SlackNotifierMock
│       └── mock_notification_service.go   # NotificationServicerMock
├── subscriptions.yaml
├── docker-compose.yml
├── Dockerfile
└── k8s/
    └── deployment.yaml
```

### Interfaces (`internal/notification/recipients.go`)

All external calls go through interfaces. Business logic never imports concrete client types directly.

```go
// GitHubTeamResolver resolves GitHub team membership.
// GetTeamMembers returns all member usernames for a given org/team slug.
// The webhook payload already identifies individual and team reviewers;
// this interface is used only to expand team slugs into individual usernames.
type GitHubTeamResolver interface {
    GetTeamMembers(ctx context.Context, org, team string) ([]string, error)
}

// SlackNotifier sends direct messages via Slack.
type SlackNotifier interface {
    SendDM(ctx context.Context, email, message string) error
    LookupUserByEmail(ctx context.Context, email string) (string, error)
}
```

### NotificationServicer (`internal/notification/service.go`)

```go
// NotificationServicer orchestrates subscriber resolution and Slack delivery.
// It accepts typed structs from the google/go-github SDK directly.
type NotificationServicer interface {
    Notify(ctx context.Context, eventType string, event any) error
}
```

The `Notify` method uses a type switch on `event` to handle:
- `*github.PullRequestEvent` — for `pull_request` events
- `*github.PullRequestReviewEvent` — for `pull_request_review` events
- `*github.PullRequestReviewCommentEvent` — for `pull_request_review_comment` events

### Client Implementations

- `github.Client` (in `internal/github/client.go`) implements `notification.GitHubTeamResolver`
- `slack.Client` (in `internal/slack/client.go`) implements `notification.SlackNotifier`

Neither client is referenced directly in business logic — they are injected at startup via their interfaces.

### Webhook Handler (`internal/webhook/handler.go`)

Responsible for:

1. Validating the `X-Hub-Signature-256` HMAC header using `github.ValidatePayload`
2. Parsing the GitHub event type from `X-GitHub-Event`
3. Calling `github.ParseWebHook(eventType, body)` to obtain a typed SDK struct (`*github.PullRequestEvent`, `*github.PullRequestReviewEvent`, or `*github.PullRequestReviewCommentEvent`)
4. Dispatching to `NotificationServicer.Notify(ctx, eventType, typedEvent)`

The handler uses `github.com/go-chi/chi/v5` for route registration.

### Subscription Loader (`internal/subscription/loader.go`)

Loads `subscriptions.yaml` at startup. Schema:

```yaml
subscriptions:
  - github_username: octocat
    email: octocat@example.com
```

## Dependencies / Tech Stack

| Package | Purpose |
|---|---|
| `google/go-github/v62` | GitHub API client + webhook payload types and parsing |
| `slack-go/slack` | Slack API client |
| `go-chi/chi/v5` | HTTP router |
| `caarlos0/env` | Environment variable loading |
| `matryer/moq` | Mock generation |
| `go.uber.org/zap` | Structured logging |
| `go.opentelemetry.io/otel` | Distributed tracing and metrics |
| `gopkg.in/yaml.v3` | YAML parsing for subscriptions |

## Mocking Strategy

All interfaces are mocked using `github.com/matryer/moq`. Mocks are generated into `internal/mocks/` and committed to the repository so CI does not require a `go generate` step.

Install: `go install github.com/matryer/moq@latest`

moq generates simple, idiomatic Go mock structs — no controller, no `EXPECT()` chains. Instead, you set function fields on the mock struct directly.

### `go:generate` Directives

Add these directives to the interface source files so mocks can be regenerated with `go generate ./...`:

```go
// In internal/notification/recipients.go
//go:generate moq -out ../mocks/mock_github_team_resolver.go -pkg mocks . GitHubTeamResolver
//go:generate moq -out ../mocks/mock_slack_notifier.go -pkg mocks . SlackNotifier

// In internal/notification/service.go
//go:generate moq -out ../mocks/mock_notification_service.go -pkg mocks . NotificationServicer
```

### Using Mocks in Tests

Mocks are injected via their interfaces. Behaviour is configured by setting function fields on the generated mock struct.

```go
func TestNotificationService_Notify(t *testing.T) {
    resolverCalls := 0
    mockResolver := &mocks.GitHubTeamResolverMock{
        GetTeamMembersFunc: func(ctx context.Context, org, team string) ([]string, error) {
            resolverCalls++
            return []string{"octocat"}, nil
        },
    }

    dmCalls := 0
    mockNotifier := &mocks.SlackNotifierMock{
        SendDMFunc: func(ctx context.Context, email, message string) error {
            dmCalls++
            return nil
        },
        LookupUserByEmailFunc: func(ctx context.Context, email string) (string, error) {
            return "U12345", nil
        },
    }

    svc := notification.NewService(mockResolver, mockNotifier, subscriptions)
    prEvent := &github.PullRequestEvent{ /* ... */ }
    err := svc.Notify(context.Background(), "pull_request", prEvent)
    require.NoError(t, err)
    assert.Equal(t, 1, resolverCalls)
    assert.Equal(t, 1, dmCalls)
}
```

> Note: Run `make generate` (or `go generate ./...`) if mocks are out of date before running tests.

## Data Models

The service uses the `google/go-github` SDK structs directly for all webhook event payloads. No internal event struct is defined — `github.ParseWebHook` returns the appropriate typed pointer which is passed through to the notification service.

Key SDK types used:
- `*github.PullRequestEvent` — PR opened, closed, review_requested
- `*github.PullRequestReviewEvent` — review submitted
- `*github.PullRequestReviewCommentEvent` — review comment posted

### Subscription

```go
type Subscription struct {
    GitHubUsername string `yaml:"github_username"`
    Email          string `yaml:"email"`
}
```

### Config

```go
type Config struct {
    GitHubWebhookSecret      string
    SlackBotToken            string
    GitHubToken              string
    Port                     string
    LogEnv                   string // "dev" or "prod"
    OTELExporterOTLPEndpoint string
    OTELServiceName          string
}
```

## Error Handling

| Scenario | Behavior |
|---|---|
| Invalid HMAC signature | Return HTTP 401, log warning |
| Unsupported event type | Return HTTP 200 (no-op), log debug |
| GitHub API error during team resolution | Return HTTP 500, log error with trace |
| Slack API error during DM delivery | Log error, continue to next subscriber (best-effort delivery) |
| Malformed JSON payload | Return HTTP 400, log warning |
| Missing required env vars at startup | Fatal log, exit 1 |
| subscriptions.yaml not found | Fatal log, exit 1 |

Errors are wrapped with context using `fmt.Errorf("...: %w", err)` throughout. Structured logging uses `zap` or `slog`. Distributed tracing spans wrap all external calls.

## Main Entry Point (`cmd/server/main.go`)

The main function wires all components and manages the server lifecycle:

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := server.Shutdown(ctx); err != nil {
    log.Warn("shutdown timeout exceeded, forcing exit", zap.Error(err))
}
```

Routes are registered using `chi.NewRouter()`:

```go
r := chi.NewRouter()
r.Post("/webhook", webhookHandler.ServeHTTP)
r.Get("/healthz", healthHandler.Healthz)
r.Get("/readyz", healthHandler.Readyz)
r.Get("/metrics", metricsHandler)
```

## Makefile

The project root `Makefile` provides common development tasks:

```makefile
.PHONY: up down logs test test-cover generate build lint

## up: start the full stack locally with Docker Compose
up:
	docker compose up --build

## down: stop and remove local Docker Compose stack
down:
	docker compose down

## logs: tail logs from the app service
logs:
	docker compose logs -f app

## test: run all unit tests
test:
	go test ./...

## test-cover: run tests with coverage report
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## generate: regenerate all moq mocks
generate:
	go generate ./...

## build: build the binary locally
build:
	CGO_ENABLED=0 go build -o skip-the-line ./cmd/server

## lint: run golangci-lint
lint:
	golangci-lint run ./...
```

## README.md

The project root `README.md` should cover the following:

**Project name**: `skip-the-line`

**What it does**: GitHub webhook receiver that sends Slack DMs to subscribed users for pull request activity (opened, closed, review requested).

**How it works**:

```
webhook → signature validation → event routing → subscriber resolution → Slack DM
```

**Configuration** — environment variables:

| Variable | Description | Required |
|---|---|---|
| `GITHUB_WEBHOOK_SECRET` | Secret used to validate webhook HMAC signatures | Yes |
| `SLACK_BOT_TOKEN` | Slack bot OAuth token (`xoxb-...`) | Yes |
| `GITHUB_TOKEN` | GitHub personal access token for team API calls | Yes |
| `PORT` | HTTP listen port (default: `8080`) | No |
| `LOG_ENV` | Logging mode: `dev` or `prod` (default: `prod`) | No |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry collector endpoint | No |
| `OTEL_SERVICE_NAME` | Service name reported to tracing backend | No |

**Subscription config** — add users to `subscriptions.yaml`:

```yaml
subscriptions:
  - github_username: octocat
    email: octocat@example.com
```

**Running locally**:

```bash
make up
```

**Stopping**:

```bash
make down
```

**Running tests**:

```bash
make test
```

**Running tests with coverage**:

```bash
make test-cover
```

**Generating mocks**:

```bash
make generate
```

**Building binary**:

```bash
make build
```

**Kubernetes deployment**: The service exposes `/healthz` (liveness) and `/readyz` (readiness) probes for use in Kubernetes deployments.

## .gitignore

The project root `.gitignore` should cover:

```gitignore
# Go binaries and build output
skip-the-line
*.exe
dist/

# Go test cache and coverage
*.test
coverage.out
coverage.html

# IDE files
.idea/
.vscode/
*.swp

# Environment files
.env
.env.local

# OS files
.DS_Store
Thumbs.db
```

Note: `Makefile` is a project file and should NOT be ignored.

## .editorconfig

The project root `.editorconfig` should define:

```ini
root = true

[*]
indent_style = tab
indent_size = 4
end_of_line = lf
charset = utf-8
trim_trailing_whitespace = true
insert_final_newline = true

[*.go]
indent_style = tab

[*.yaml]
indent_style = space
indent_size = 2

[*.yml]
indent_style = space
indent_size = 2

[*.md]
trim_trailing_whitespace = false

[Makefile]
indent_style = tab
```

## Testing Strategy

Tests use the standard `testing` package with `github.com/matryer/moq` for mocking external dependencies.

Use table-driven tests for any case with multiple input/output combinations — this keeps test files concise and makes it easy to add new cases.

### Mocking External Dependencies

Mocks are generated into `internal/mocks/` via `make generate` (or `go generate ./...`) and should be committed to the repository.

> Run `make generate` before running tests if mocks are out of date.

Mock injection pattern: construct the system under test with mock implementations of `GitHubTeamResolver`, `SlackNotifier`, and `NotificationServicer` interfaces. Configure behaviour by setting the corresponding `*Func` fields on the generated mock struct.

### Test Coverage Targets

- Webhook handler: signature validation, event routing, error responses
- Notification service: subscriber resolution logic, DM dispatch, error propagation
- Subscription loader: YAML parsing, missing file handling
- GitHub client: team membership resolution (integration test, skipped in CI without token)
- Slack client: DM delivery (integration test, skipped in CI without token)
