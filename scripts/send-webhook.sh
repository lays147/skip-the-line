#!/usr/bin/env bash
# send-webhook.sh — send a sample GitHub webhook payload to the local app.
#
# Usage:
#   ./scripts/send-webhook.sh <event>
#
# Supported events:
#   pull_request                — review_requested (individual reviewers only)
#   pull_request_review         — review submitted (approved)
#   pull_request_review_comment — comment posted on a review
#
# Environment:
#   WEBHOOK_URL          target URL                     (default: http://localhost:8080/webhook)
#   WEBHOOK_SECRET       HMAC signing secret            (default: test-secret)
#   NOTIFICATION_TARGET  GitHub username that should receive the Slack DM.
#                        Maps to the reviewer on pull_request events and to the
#                        PR author on pull_request_review / pull_request_review_comment
#                        events. Set this to your own GitHub username to test e2e.
#                        (default: lays147)

set -euo pipefail

WEBHOOK_URL="${WEBHOOK_URL:-http://localhost:8080/webhook}"
WEBHOOK_SECRET="${WEBHOOK_SECRET:-test-secret}"
NOTIFICATION_TARGET="${NOTIFICATION_TARGET:-lays147}"
EVENT="${1:-}"

usage() {
  echo "Usage: $0 <event>"
  echo ""
  echo "Supported events:"
  echo "  pull_request              review_requested with individual reviewers"
  echo "  pull_request_review       review submitted (state: approved)"
  echo "  pull_request_review_comment  comment posted on a review"
  echo ""
  echo "Set NOTIFICATION_TARGET to your GitHub username to receive the Slack DM."
  exit 1
}

[ -z "$EVENT" ] && usage

# ── payloads ────────────────────────────────────────────────────────────────

payload_pull_request() {
  cat <<EOF
{
  "action": "review_requested",
  "number": 42,
  "pull_request": {
    "number": 42,
    "title": "feat: add awesome feature",
    "html_url": "https://github.com/lays147/skip-the-line/pull/1",
    "user": { "login": "sender" },
    "requested_reviewers": [
      { "login": "$NOTIFICATION_TARGET" }
    ],
    "requested_teams": []
  },
  "repository": {
    "full_name": "org/repo",
    "owner": { "login": "org" }
  },
  "sender": { "login": "sender" }
}
EOF
}

payload_pull_request_review() {
  cat <<EOF
{
  "action": "submitted",
  "review": {
    "id": 1,
    "state": "approved",
    "user": { "login": "reviewer" },
    "body": "Looks great, ship it!"
  },
  "pull_request": {
    "number": 42,
    "title": "feat: add awesome feature",
    "html_url": "https://github.com/org/repo/pull/42",
    "user": { "login": "$NOTIFICATION_TARGET" }
  },
  "repository": {
    "full_name": "org/repo",
    "owner": { "login": "org" }
  },
  "sender": { "login": "reviewer" }
}
EOF
}

payload_pull_request_review_comment() {
  cat <<EOF
{
  "action": "created",
  "comment": {
    "id": 1,
    "body": "Hey, can you take a look at this part?",
    "user": { "login": "reviewer" },
    "html_url": "https://github.com/org/repo/pull/42#discussion_r1"
  },
  "pull_request": {
    "number": 42,
    "title": "feat: add awesome feature",
    "html_url": "https://github.com/org/repo/pull/42",
    "user": { "login": "$NOTIFICATION_TARGET" }
  },
  "repository": {
    "full_name": "org/repo",
    "owner": { "login": "org" }
  },
  "sender": { "login": "reviewer" }
}
EOF
}

# ── select payload ───────────────────────────────────────────────────────────

case "$EVENT" in
  pull_request)                PAYLOAD=$(payload_pull_request) ;;
  pull_request_review)         PAYLOAD=$(payload_pull_request_review) ;;
  pull_request_review_comment) PAYLOAD=$(payload_pull_request_review_comment) ;;
  *) echo "Unknown event: $EVENT"; echo ""; usage ;;
esac

# ── sign & send ──────────────────────────────────────────────────────────────

SIG=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | awk '{print "sha256="$2}')

echo "→ POST $WEBHOOK_URL"
echo "  X-GitHub-Event: $EVENT"
echo "  X-Hub-Signature-256: $SIG"
echo "  NOTIFICATION_TARGET: $NOTIFICATION_TARGET"
echo ""

HTTP_STATUS=$(curl -s -o /tmp/webhook-response.json -w "%{http_code}" \
  -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: $EVENT" \
  -H "X-Hub-Signature-256: $SIG" \
  -d "$PAYLOAD")

echo "← HTTP $HTTP_STATUS"
[ -s /tmp/webhook-response.json ] && cat /tmp/webhook-response.json && echo ""
