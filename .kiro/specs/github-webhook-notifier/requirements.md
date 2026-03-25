# Requirements Document

## Introduction

A Go application that receives GitHub webhook events for pull request activity and sends targeted Slack notifications to relevant users. The system translates GitHub users to Slack users via email lookup, uses an embedded subscription YAML file to determine who opts in to notifications, and avoids duplicate notifications. The application is designed for containerized deployment on Kubernetes, with structured logging, OpenTelemetry metrics, and health probe endpoints.

## Glossary

- **Webhook_Server**: The Go HTTP server that receives and processes GitHub webhook payloads.
- **Subscription_Config**: The YAML file embedded in the binary at build time that maps GitHub usernames to email addresses for Slack lookup.
- **Slack_Client**: The component responsible for looking up Slack user IDs by email and sending Slack messages.
- **GitHub_Client**: The component responsible for resolving GitHub team memberships via the GitHub API.
- **Notification_Service**: The component that orchestrates recipient resolution and dispatches Slack notifications.
- **Reviewer**: A GitHub user or team listed as a requested reviewer on a pull request.
- **Author**: The GitHub user who opened the pull request.
- **Subscriber**: A user present in the Subscription_Config who has opted in to notifications.
- **Logger**: The structured logging component backed by Uber's zap library, used throughout the system for all log output.
- **Metrics_Exporter**: The OpenTelemetry component responsible for recording webhook event counters and exposing metrics to an external collector or Prometheus scrape endpoint.
- **Health_Handler**: The HTTP handler that exposes liveness and readiness probe endpoints for Kubernetes.

## Requirements

### Requirement 1: Receive GitHub Webhook Events

**User Story:** As a platform engineer, I want the application to receive GitHub webhook events, so that pull request activity can trigger Slack notifications.

#### Acceptance Criteria

1. WHEN a POST request is received at the webhook endpoint, THE Webhook_Server SHALL parse the `X-GitHub-Event` header to determine the event type.
2. WHEN the event type is `pull_request`, `pull_request_review`, or `pull_request_review_comment`, THE Webhook_Server SHALL process the payload.
3. WHEN the event type is not one of the supported types, THE Webhook_Server SHALL return HTTP 200 and take no further action.
4. IF the request body cannot be parsed as a valid GitHub webhook payload, THEN THE Webhook_Server SHALL return HTTP 400 and log the error.

### Requirement 2: Validate Webhook Signatures

**User Story:** As a security-conscious engineer, I want webhook payloads to be validated using HMAC signatures, so that only legitimate GitHub events are processed.

#### Acceptance Criteria

1. WHEN a webhook request is received, THE Webhook_Server SHALL validate the `X-Hub-Signature-256` header against the payload using the configured secret.
2. IF the signature is missing or invalid, THEN THE Webhook_Server SHALL return HTTP 401 and reject the request.
3. THE Webhook_Server SHALL use the HMAC-SHA256 algorithm for signature validation.

### Requirement 3: Resolve Subscribers from Embedded Config

**User Story:** As a developer, I want the application to load subscriber mappings from an embedded YAML file, so that no external config service is needed at runtime.

#### Acceptance Criteria

1. THE Subscription_Config SHALL be embedded in the binary at build time using Go's `embed` package.
2. WHEN the application starts, THE Notification_Service SHALL load and parse the Subscription_Config into memory.
3. IF the Subscription_Config cannot be parsed at startup, THEN THE Webhook_Server SHALL fail to start and log a fatal error.
4. THE Subscription_Config SHALL map each GitHub username to an email address used for Slack user lookup.

### Requirement 4: Resolve GitHub Team Members

**User Story:** As a developer, I want the application to expand GitHub team reviewers into individual members, so that team-level review requests notify all relevant subscribers.

#### Acceptance Criteria

1. WHEN a pull request event contains a team as a requested reviewer, THE GitHub_Client SHALL resolve the team's members via the GitHub API.
2. IF the GitHub API returns an error, THEN THE Notification_Service SHALL log the error and skip notifications for that team.
3. THE GitHub_Client SHALL use a configured GitHub personal access token for API authentication.

### Requirement 5: Translate GitHub Users to Slack Users

**User Story:** As a developer, I want GitHub usernames to be mapped to Slack user IDs via email, so that Slack messages are sent to the correct users.

#### Acceptance Criteria

1. WHEN a recipient GitHub username is resolved, THE Slack_Client SHALL look up the corresponding Slack user ID using the email address from the Subscription_Config.
2. IF the Slack API returns no user for the given email, THEN THE Notification_Service SHALL log a warning and skip that recipient.
3. IF the Slack API returns an error, THEN THE Notification_Service SHALL log the error and skip that recipient.

### Requirement 6: Send Slack Notifications

**User Story:** As a subscriber, I want to receive Slack direct messages for pull request activity I care about, so that I can respond promptly.

#### Acceptance Criteria

1. WHEN a pull request event is processed and recipients are resolved, THE Notification_Service SHALL send a Slack direct message to each resolved Slack user.
2. THE Notification_Service SHALL NOT send a notification to the Author of the pull request.
3. THE Notification_Service SHALL only send notifications to users present in the Subscription_Config.
4. WHEN a `pull_request` event with action `review_requested` is received, THE Notification_Service SHALL notify the requested reviewers who are Subscribers.
5. WHEN a `pull_request_review` event with action `submitted` is received, THE Notification_Service SHALL notify the Author if the Author is a Subscriber.
6. WHEN a `pull_request_review_comment` event is received, THE Notification_Service SHALL notify the Author and any Subscribers mentioned in the comment.

### Requirement 7: Avoid Duplicate Notifications

**User Story:** As a subscriber, I want to receive each notification only once per event, so that I am not spammed by repeated messages.

#### Acceptance Criteria

1. THE Notification_Service SHALL deduplicate recipients within a single event processing cycle before sending notifications.
2. WHEN the same Slack user ID appears more than once in the resolved recipient list, THE Notification_Service SHALL send only one notification to that user for that event.

### Requirement 8: Configuration via Environment Variables

**User Story:** As a platform engineer, I want the application to be configured via environment variables, so that secrets and settings can be injected at runtime without modifying the binary.

#### Acceptance Criteria

1. THE Webhook_Server SHALL read the webhook secret from an environment variable named `GITHUB_WEBHOOK_SECRET`.
2. THE Slack_Client SHALL read the Slack bot token from an environment variable named `SLACK_BOT_TOKEN`.
3. THE GitHub_Client SHALL read the GitHub personal access token from an environment variable named `GITHUB_TOKEN`.
4. IF a required environment variable is not set at startup, THEN THE Webhook_Server SHALL fail to start and log a fatal error identifying the missing variable.

### Requirement 9: Graceful Shutdown

**User Story:** As a platform engineer, I want the application to shut down gracefully, so that in-flight requests are completed before the process exits.

#### Acceptance Criteria

1. WHEN the application receives a SIGTERM or SIGINT signal, THE Webhook_Server SHALL stop accepting new connections.
2. WHILE a graceful shutdown is in progress, THE Webhook_Server SHALL allow in-flight requests up to a configured timeout to complete before exiting.
3. IF the shutdown timeout is exceeded, THEN THE Webhook_Server SHALL force-exit and log a warning.

### Requirement 10: Structured Error Responses

**User Story:** As an integrator, I want the application to return consistent JSON error responses, so that webhook senders can programmatically handle failures.

#### Acceptance Criteria

1. WHEN THE Webhook_Server returns an HTTP 4xx or 5xx response, THE Webhook_Server SHALL include a JSON body with an `error` field containing a human-readable message.
2. THE Webhook_Server SHALL set the `Content-Type` header to `application/json` on all error responses.

### Requirement 11: Structured Logging with Zap

**User Story:** As a platform engineer, I want all application logs to be structured and machine-readable, so that log aggregation systems can parse and query them efficiently.

#### Acceptance Criteria

1. THE Logger SHALL use Uber's zap library as the sole logging implementation throughout the system.
2. THE Logger SHALL emit logs in JSON format in production mode and human-readable format in development mode, controlled by a `LOG_ENV` environment variable.
3. WHEN any component logs an event, THE Logger SHALL include at minimum the fields: `timestamp`, `level`, `message`, and `component`.
4. WHEN an error is logged, THE Logger SHALL include the `error` field containing the error message.
5. IF the `LOG_ENV` environment variable is not set, THEN THE Logger SHALL default to production (JSON) mode.

### Requirement 12: OpenTelemetry Metrics

**User Story:** As a platform engineer, I want the application to expose webhook event counters as OpenTelemetry metrics, so that I can monitor event throughput and detect anomalies.

#### Acceptance Criteria

1. WHEN a `pull_request` webhook event is received and validated, THE Metrics_Exporter SHALL increment a counter named `webhook_events_total` with a label `event_type=pull_request`.
2. WHEN a `pull_request_review` webhook event is received and validated, THE Metrics_Exporter SHALL increment a counter named `webhook_events_total` with a label `event_type=pull_request_review`.
3. WHEN a `pull_request_review_comment` webhook event is received and validated, THE Metrics_Exporter SHALL increment a counter named `webhook_events_total` with a label `event_type=pull_request_review_comment`.
4. THE Metrics_Exporter SHALL expose metrics via a Prometheus-compatible HTTP endpoint at `GET /metrics`.
5. WHERE an OTLP exporter endpoint is configured via the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable, THE Metrics_Exporter SHALL additionally push metrics to that endpoint.
6. WHEN the application starts, THE Metrics_Exporter SHALL initialize the OpenTelemetry SDK with a service name read from the `OTEL_SERVICE_NAME` environment variable, defaulting to `github-webhook-notifier`.

### Requirement 13: Kubernetes Health Probes

**User Story:** As a platform engineer, I want the application to expose liveness and readiness endpoints, so that Kubernetes can manage the pod lifecycle correctly.

#### Acceptance Criteria

1. THE Health_Handler SHALL expose a `GET /healthz` endpoint for Kubernetes liveness probes.
2. WHEN the application process is running and the HTTP server is responsive, THE Health_Handler SHALL return HTTP 200 with body `{"status":"ok"}` on `GET /healthz`.
3. THE Health_Handler SHALL expose a `GET /readyz` endpoint for Kubernetes readiness probes.
4. WHEN all required dependencies (Subscription_Config loaded, environment variables validated) are initialized, THE Health_Handler SHALL return HTTP 200 with body `{"status":"ready"}` on `GET /readyz`.
5. IF any required dependency is not yet initialized, THEN THE Health_Handler SHALL return HTTP 503 with body `{"status":"not ready"}` on `GET /readyz`.

### Requirement 14: Dockerfile for Containerization

**User Story:** As a platform engineer, I want a Dockerfile to build a minimal container image for the application, so that it can be deployed to Kubernetes.

#### Acceptance Criteria

1. THE Dockerfile SHALL use a multi-stage build: a builder stage using the official Go image and a final stage using a minimal base image (e.g., `gcr.io/distroless/static` or `alpine`).
2. THE Dockerfile SHALL produce an image that runs the compiled Go binary as the container entrypoint.
3. THE Dockerfile SHALL not include the Go toolchain or build artifacts in the final image.
4. THE Dockerfile SHALL expose the application's HTTP port via an `EXPOSE` instruction.
5. THE Dockerfile SHALL run the application as a non-root user in the final image.

### Requirement 15: Docker Compose for Local Development

**User Story:** As a developer, I want a Docker Compose configuration to run the full application stack locally with mocked dependencies, so that I can develop and test without real GitHub or Slack credentials.

#### Acceptance Criteria

1. THE docker-compose.yml SHALL define a service for the application built from the local Dockerfile.
2. THE docker-compose.yml SHALL define a mock GitHub webhook sender service that can replay sample webhook payloads to the application.
3. THE docker-compose.yml SHALL define a mock Slack API service that accepts Slack API calls and logs received messages without forwarding them.
4. THE docker-compose.yml SHALL wire all services on a shared Docker network so they can communicate by service name.
5. THE docker-compose.yml SHALL define environment variable defaults for the application service sufficient to run in the local mock environment.
6. WHEN the Docker Compose stack is started, THE application service SHALL be reachable at a documented local port (e.g., `localhost:8080`).
