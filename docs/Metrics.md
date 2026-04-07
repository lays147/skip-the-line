# Metrics

Skip The Line exports OpenTelemetry (OTel) metrics to track application behavior and performance. All metrics are registered with the meter name `github.com/skip-the-line`.

## Metric Export

Metrics are exported via OTLP gRPC when the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable is set. If not configured, metrics are discarded (no-op).

See [Deployment.md](./Deployment.md) for instructions on configuring the OTLP endpoint.

## Available Metrics

### webhook_events_total

**Type:** Counter (Int64)  
**Unit:** Total count  
**Description:** Total number of webhook events received from GitHub.

**Labels:**
- `event_type` — The GitHub event type (e.g., `pull_request`, `pull_request_review`, etc.)

**Use case:** Monitor the volume and types of GitHub events being processed.

---

### pr_merge_duration_seconds

**Type:** Histogram (Float64)  
**Unit:** Seconds  
**Description:** Time elapsed from when a PR is opened to when it is merged.

**Labels:**
- `subscribed` — Boolean indicating whether the PR author is in the subscriber registry (true/false)

**Use case:** Measure Mean Time to Merge (MTTM) and correlate it with subscriber participation. Higher values for `subscribed=true` may indicate faster reviews when the author gets notifications.

---

### notification_deliveries_total

**Type:** Counter (Int64)  
**Unit:** Total count  
**Description:** Total number of Slack DM delivery attempts made by the service.

**Labels:**
- `event_type` — The GitHub event type that triggered the notification
- `outcome` — One of:
  - `ok` — Notification delivered successfully
  - `slack_lookup_failed` — Slack user lookup by email failed
  - `slack_send_failed` — Slack DM send failed
  - `error` — Other errors occurred

**Use case:** Track notification delivery success rate and identify failure modes. Calculate delivery success rate as `ok / total`.

---

### slack_lookup_duration_seconds

**Type:** Histogram (Float64)  
**Unit:** Seconds  
**Description:** Latency of Slack API `LookupUserByEmail` calls.

**Labels:**
- `outcome` — One of:
  - `ok` — User lookup succeeded
  - `error` — Lookup failed

**Use case:** Monitor Slack API performance and identify latency spikes or timeout issues.

---

### slack_send_duration_seconds

**Type:** Histogram (Float64)  
**Unit:** Seconds  
**Description:** Latency of Slack API `SendDM` (message send) calls.

**Labels:**
- `outcome` — One of:
  - `ok` — Message sent successfully
  - `error` — Send failed

**Use case:** Monitor Slack message delivery latency and detect performance degradation.

---

### github_team_members_duration_seconds

**Type:** Histogram (Float64)  
**Unit:** Seconds  
**Description:** Latency of GitHub API `GetTeamMembers` calls.

**Labels:**
- `outcome` — One of:
  - `ok` — Team members retrieved successfully
  - `error` — Retrieval failed

**Use case:** Monitor GitHub API performance when resolving team members for notifications.

---

## Correlation with Traces

When OpenTelemetry tracing is enabled, metrics are automatically correlated with distributed traces. Each metric data point may include trace context (`trace_id`, `span_id`) when recorded within an active span, enabling end-to-end observability across metrics and traces.

See [Deployment.md](./Deployment.md) for trace configuration.
