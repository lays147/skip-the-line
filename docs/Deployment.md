# Deployment Guide

## Prerequisites

### 1. Notification Platform

Set `NOTIFICATION_PLATFORM` to choose where notifications are delivered. Accepted values: `slack` (default) or `google_chat`.

#### Slack

Create a Slack app with the following OAuth permission scopes:

| Scope | Purpose |
|---|---|
| `chat:write` | Send DMs to users |
| `im:write` | Open direct message channels |
| `users:read` | Resolve user profiles |
| `users:read.email` | Look up users by email address |

Install the app into your Slack workspace and save the **Bot User OAuth Token** (`xoxb-…`).

#### Google Chat

1. Create a **Google Cloud service account** and download its JSON key.
2. Enable **Domain-wide Delegation** on the service account and grant it these OAuth scopes:
   - `https://www.googleapis.com/auth/chat.bot` — send messages as the Chat app
   - `https://www.googleapis.com/auth/admin.directory.user.readonly` — look up users by email via the Admin SDK
3. In the Google Admin console, authorize the service account's client ID with the scopes above.
4. Create a **Google Chat app** in Google Cloud Console (Pub/Sub or HTTP endpoint type) and associate the service account with it.
5. Note a **Google Workspace admin email** to use for Admin SDK impersonation (`GCHAT_ADMIN_EMAIL`).

### 2. GitHub Personal Access Token

Create a GitHub PAT with `read:org` scope (required to resolve team members). Save the token value.

### 3. Webhook Secret

Generate a random secret string to be shared between GitHub and this service for HMAC signature validation:

```bash
openssl rand -hex 32
```

---

## Subscriptions File

The service maps GitHub usernames to email addresses using a YAML file:

```yaml
subscriptions:
  - github_username: octocat
    email: octocat@example.com
  - github_username: monalisa
    email: monalisa@example.com
```

There are two ways to provide this file depending on how you run the service:

| Mode | How it works |
|---|---|
| **Fork the repo** | Edit `internal/subscription/subscriptions.yaml` and build your own image. The file is embedded at compile time — no runtime configuration needed. |
| **Pull the published image** | Mount your `subscriptions.yaml` into the container and set `SUBSCRIPTIONS_PATH` to its path. The embedded file is ignored when this variable is set. |

---

## Environment Variables

### Common

| Variable | Required | Default | Description |
|---|---|---|---|
| `GITHUB_WEBHOOK_SECRET` | Yes | — | HMAC secret shared with the GitHub webhook |
| `GITHUB_TOKEN` | Yes | — | GitHub PAT with `read:org` scope |
| `NOTIFICATION_PLATFORM` | No | `slack` | Chat platform to use: `slack` or `google_chat` |
| `SUBSCRIPTIONS_PATH` | No | — | Path to an external `subscriptions.yaml`. When set, the embedded file is ignored. |
| `PORT` | No | `8080` | HTTP port the server listens on |
| `LOG_LEVEL` | No | `info` | Logging level (`debug`, `info`, `warn`, `error`) |
| `ENVIRONMENT` | No | `dev` | Environment label added to all log lines |
| `READ_TIMEOUT_SECONDS` | No | `10` | HTTP server read timeout |
| `WRITE_TIMEOUT_SECONDS` | No | `10` | HTTP server write timeout |
| `HANDLER_TIMEOUT_SECONDS` | No | `8` | Per-request deadline for webhook processing |
| `DELIVERY_DEDUP_TTL_HOURS` | No | `4` | How long to suppress duplicate webhook deliveries |
| `OTEL_SERVICE_NAME` | No | `github-webhook-notifier` | Service name reported to the OTel collector |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | — | OTLP gRPC endpoint (e.g. `otel-collector:4317`). Omit to disable tracing/metrics export. |

### Slack (`NOTIFICATION_PLATFORM=slack`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `SLACK_BOT_TOKEN` | Yes | — | Slack bot OAuth token (`xoxb-…`) |
| `SLACK_API_URL` | No | — | Override Slack API base URL (for local testing) |

### Google Chat (`NOTIFICATION_PLATFORM=google_chat`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `GCHAT_CREDENTIALS_JSON` | Yes | — | Google service account key JSON (raw string) |
| `GCHAT_ADMIN_EMAIL` | Yes | — | Google Workspace admin email used to impersonate Admin SDK calls |

### Local development only

| Variable | Description |
|---|---|
| `GITHUB_API_URL` | Override GitHub API base URL |

---

## Kubernetes

### Secrets

#### Slack

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: skip-the-line
  namespace: default
type: Opaque
stringData:
  GITHUB_WEBHOOK_SECRET: "<your-webhook-secret>"
  GITHUB_TOKEN: "<your-github-pat>"
  SLACK_BOT_TOKEN: "<your-slack-bot-token>"
```

#### Google Chat

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: skip-the-line
  namespace: default
type: Opaque
stringData:
  GITHUB_WEBHOOK_SECRET: "<your-webhook-secret>"
  GITHUB_TOKEN: "<your-github-pat>"
  GCHAT_CREDENTIALS_JSON: |
    {
      "type": "service_account",
      "project_id": "...",
      ...
    }
  GCHAT_ADMIN_EMAIL: "admin@yourdomain.com"
```

### Subscriptions ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: skip-the-line-subscriptions
  namespace: default
data:
  subscriptions.yaml: |
    subscriptions:
      - github_username: octocat
        email: octocat@example.com
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: skip-the-line
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: skip-the-line
  template:
    metadata:
      labels:
        app: skip-the-line
    spec:
      containers:
        - name: skip-the-line
          image: public.ecr.aws/s1h5y5v1/skip-the-line:<tag>
          ports:
            - containerPort: 8080
          env:
            - name: SUBSCRIPTIONS_PATH
              value: /etc/skip-the-line/subscriptions.yaml
            - name: LOG_LEVEL
              value: info
            - name: ENVIRONMENT
              value: production
            - name: NOTIFICATION_PLATFORM
              value: slack   # or google_chat
          envFrom:
            - secretRef:
                name: skip-the-line
          volumeMounts:
            - name: subscriptions
              mountPath: /etc/skip-the-line
              readOnly: true
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
      volumes:
        - name: subscriptions
          configMap:
            name: skip-the-line-subscriptions
---
apiVersion: v1
kind: Service
metadata:
  name: skip-the-line
  namespace: default
spec:
  selector:
    app: skip-the-line
  ports:
    - port: 80
      targetPort: 8080
```

To update the subscriber list, edit the `ConfigMap` and restart the pod:

```bash
kubectl edit configmap skip-the-line-subscriptions
kubectl rollout restart deployment/skip-the-line
```

## GitHub Webhook Configuration

1. Go to your GitHub Organization → **Settings** → **Webhooks** → **Add webhook**
2. Set **Payload URL** to your public endpoint (e.g. `https://your-domain/webhook`)
3. Set **Content type** to `application/json`
4. Set **Secret** to the value of `GITHUB_WEBHOOK_SECRET`
5. Under **Which events**, select **Let me select individual events** and enable:
   - Pull requests
   - Pull request review comments
   - Pull request reviews
6. Ensure **Active** is checked and save
