# Project Structure

```
cmd/server/main.go              # Entrypoint: wires config, clients, services, router, graceful shutdown

internal/
  config/                       # Env var config via caarlos0/env; Config struct + Load()
  github/                       # GitHub API client (team membership resolution)
  slack/                        # Slack API client (LookupUserByEmail, SendDM)
  webhook/                      # HTTP handler: signature validation, event parsing, dispatch
  notification/                 # Core business logic: event routing, recipient resolution, Slack DM delivery
    service.go                  # NotificationService + NotificationServicer interface
    recipients.go               # GitHubTeamResolver + SlackNotifier interfaces (with go:generate directives)
  subscription/                 # Subscription registry; subscriptions.yaml embedded via //go:embed
  health/                       # /healthz and /readyz handlers
  metrics/                      # OTel meter provider + webhook_events_total counter
  mocks/                        # moq-generated mocks (DO NOT edit manually)

scripts/send-webhook.sh         # Helper to send sample webhooks locally
```

## Conventions

- Interfaces are defined in the package that uses them (consumer-side), not the package that implements them
- `//go:generate moq ...` directives live in `recipients.go` alongside the interface definitions
- Mocks go in `internal/mocks/` with package name `mocks`; regenerate with `make generate`
- Config is loaded once at startup and passed down via constructor injection — no global state
- Clients (`github`, `slack`) implement the interfaces defined in `notification/recipients.go`
- Slack messages use Block Kit JSON built with `[]any` maps and marshalled to string
- Errors from Slack delivery are best-effort: logged and skipped, never returned to the caller
- `subscriptions.yaml` is embedded at compile time; changes require a rebuild
- Tests use table-driven style with `t.Run`; test files use `_test` package suffix for black-box testing (e.g. `package notification_test`)
- Internal handler tests (e.g. `webhook`) use the same package (white-box) when access to unexported helpers is needed
