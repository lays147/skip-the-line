# Product

skip-the-line is a GitHub webhook receiver that sends Slack DMs to reduce PR review toil.

When a PR review is requested, a review is submitted, or a review comment is posted, the service resolves the relevant GitHub users, looks them up in a subscription registry, and delivers a Slack Block Kit DM.

## Core Flow

```
GitHub webhook → HMAC signature validation → event routing → subscriber resolution → Slack DM
```

## Supported Events

| GitHub Event | Action | Who gets notified |
|---|---|---|
| `pull_request` | `review_requested` | Requested reviewers (individuals + team members) |
| `pull_request_review` | `submitted` | PR author |
| `pull_request_review_comment` | any | PR author + @mentioned subscribers |

## Subscription Model

Users opt in via `internal/subscription/subscriptions.yaml` (embedded at build time). Each entry maps a GitHub username to an email address used to look up their Slack user ID.
