# skip-the-line

Skip The Line has the purpose to reduce the cognitive toil of pinging coworkers to review a PR and for the author to keep checking GitHub for reviews on it's own PR's. 

With this automation we use GitHub Webhooks to send notifications via slack for reviewers to review and for an author to check out request changes or approves.

By using this automation your organization might have the opportunity to reduce the Mean Time to Merge (MTTM) to **about 40%**.

## Prompted by me, built with AI

> Project 98% built with [AWS Kiro](kiro.dev), 1% by Claude and 1% by me.

This project was built as a study case of AWS Kiro. Further updates and refactorings were made using Claude.

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
make up
```

The app will be reachable at `http://localhost:8080`.

## Environment Variables

| Variable | Description | Required | Default |
|---|---|---|---|
| `GITHUB_WEBHOOK_SECRET` | Secret used to validate webhook HMAC signatures | Yes | — |
| `SLACK_BOT_TOKEN` | Slack bot OAuth token (`xoxb-...`) | Yes | — |
| `GITHUB_TOKEN` | GitHub personal access token for team API calls | Yes | — |
| `PORT` | HTTP listen port | No | `8080` |
| `LOG_LEVEL` | Log verbosity: `debug`, `info`, `warn`, `error` | No | `info` |
| `ENVIRONMENT` | Deployment environment tag attached to every log and metric | No | `dev` |
| `OTEL_SERVICE_NAME` | Service name reported to the OTel backend | No | `github-webhook-notifier` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP gRPC collector endpoint (e.g. `otel-collector:4317`); metrics are dropped when unset | No | — |
| `SLACK_API_URL` | Override Slack API base URL (local dev / testing) | No | — |
| `GITHUB_API_URL` | Override GitHub API base URL (local dev / testing) | No | — |

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
