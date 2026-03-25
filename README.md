# skip-the-line

GitHub webhook receiver that sends Slack DMs to subscribed users for pull request activity (opened, closed, review requested).

## How it works

```
webhook → signature validation → event routing → subscriber resolution → Slack DM
```

## Prerequisites

- Go 1.26.1+
- Docker & Docker Compose
- A GitHub App or webhook configured with a secret
- A Slack bot token (`xoxb-...`) with `users:read.email` and `chat:write` scopes

## Quickstart

```bash
# Copy and fill in your environment variables
cp .env.example .env

# Start the full stack locally
make up
```

The app will be reachable at `http://localhost:8080`.

## Environment Variables

| Variable | Description | Required |
|---|---|---|
| `GITHUB_WEBHOOK_SECRET` | Secret used to validate webhook HMAC signatures | Yes |
| `SLACK_BOT_TOKEN` | Slack bot OAuth token (`xoxb-...`) | Yes |
| `GITHUB_TOKEN` | GitHub personal access token for team API calls | Yes |
| `PORT` | HTTP listen port (default: `8080`) | No |
| `LOG_ENV` | Logging mode: `dev` or `prod` (default: `prod`) | No |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry collector endpoint | No |
| `OTEL_SERVICE_NAME` | Service name reported to tracing backend | No |

## Subscription Config

Add users to `subscriptions.yaml` to opt them in to notifications:

```yaml
subscriptions:
  - github_username: octocat
    email: octocat@example.com
```

The `email` field is used to look up the corresponding Slack user ID.

## Makefile Targets

| Target | Description |
|---|---|
| `make up` | Start the full stack with Docker Compose |
| `make down` | Stop and remove the Docker Compose stack |
| `make logs` | Tail logs from the app service |
| `make test` | Run all unit tests |
| `make test-cover` | Run tests and generate HTML coverage report |
| `make generate` | Regenerate all moq mocks |
| `make build` | Build the binary locally |
| `make lint` | Run golangci-lint |

## Kubernetes Probes

The service exposes health endpoints for Kubernetes pod lifecycle management:

| Endpoint | Type | Success Response |
|---|---|---|
| `GET /healthz` | Liveness | `200 {"status":"ok"}` |
| `GET /readyz` | Readiness | `200 {"status":"ready"}` |

The readiness probe returns `503 {"status":"not ready"}` until all dependencies (config, subscriptions) are fully initialised.

Example Kubernetes probe configuration:

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```
