package notification

import (
	"context"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/logger"
	"github.com/skip-the-line/internal/metrics"
	"github.com/skip-the-line/internal/subscription"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

const tracerName = "github.com/skip-the-line"

//go:generate moq -out ../mocks/mock_notification_service.go -pkg mocks . NotificationServicer

// NotificationServicer orchestrates subscriber resolution and Slack delivery.
// It accepts typed structs from the google/go-github SDK directly.
//
// The event parameter accepts:
//   - *github.PullRequestEvent
//   - *github.PullRequestReviewEvent
//   - *github.PullRequestReviewCommentEvent
type NotificationServicer interface {
	Notify(ctx context.Context, eventType string, event any) error
}

// NotificationService is the concrete implementation of NotificationServicer.
type NotificationService struct {
	resolver GitHubTeamResolver
	notifier SlackNotifier
	subs     subscription.Registry
	logger   *zap.Logger
	metrics  *metrics.Metrics
}

// NewNotificationService constructs a NotificationService.
func NewNotificationService(resolver GitHubTeamResolver, notifier SlackNotifier, subs subscription.Registry, logger *zap.Logger, m *metrics.Metrics) *NotificationService {
	return &NotificationService{
		resolver: resolver,
		notifier: notifier,
		subs:     subs,
		logger:   logger,
		metrics:  m,
	}
}

// Notify dispatches a GitHub webhook event to the appropriate notification handler.
func (s *NotificationService) Notify(ctx context.Context, eventType string, event any) error {
	switch e := event.(type) {
	case *github.PullRequestEvent:
		if e.GetAction() == "review_requested" {
			return s.handleReviewRequested(ctx, e)
		}
	case *github.PullRequestReviewEvent:
		if e.GetAction() == "submitted" {
			return s.handleReviewSubmitted(ctx, e)
		}
	case *github.PullRequestReviewCommentEvent:
		return s.handleReviewComment(ctx, e)
	}
	// no-op for unrecognised event type/action combinations
	return nil
}

// sendToRecipients resolves emails and Slack user IDs for the recipient set,
// then sends a DM to each. The exclude login is skipped (e.g. the PR author
// when notifying reviewers, or the commenter when notifying the author).
//
// Flow: unique GitHub usernames → Registry.EmailFor (O(1)) → LookupUserByEmail → SendDM
func (s *NotificationService) sendToRecipients(ctx context.Context, recipients map[string]struct{}, exclude string, blocks []slack.Block, eventType string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "notification.sendToRecipients")
	span.SetAttributes(
		attribute.String("event_type", eventType),
		attribute.Int("recipient_count", len(recipients)),
	)
	defer span.End()

	for username := range recipients {
		if username == exclude {
			continue
		}
		email, ok := s.subs.EmailFor(username)
		if !ok {
			// not a subscriber — skip silently
			continue
		}

		lookupStart := time.Now()
		slackUserID, err := s.notifier.LookupUserByEmail(ctx, email)
		lookupDuration := time.Since(lookupStart)
		if err != nil {
			s.metrics.RecordSlackLookupDuration(ctx, lookupDuration, metrics.OutcomeSlackLookupFailed)
			logger.FromContext(ctx, s.logger).Warn("failed to look up Slack user",
				zap.String("github_username", username),
				zap.String("email", email),
				zap.Error(err),
			)
			s.metrics.RecordNotificationDelivery(ctx, eventType, metrics.OutcomeSlackLookupFailed)
			continue
		}
		s.metrics.RecordSlackLookupDuration(ctx, lookupDuration, metrics.OutcomeOK)

		sendStart := time.Now()
		err = s.notifier.SendDM(ctx, slackUserID, blocks)
		s.metrics.RecordSlackSendDuration(ctx, time.Since(sendStart), outcomeFor(err))
		if err != nil {
			logger.FromContext(ctx, s.logger).Error("failed to send Slack DM",
				zap.String("github_username", username),
				zap.String("slack_user_id", slackUserID),
				zap.Error(err),
			)
			s.metrics.RecordNotificationDelivery(ctx, eventType, metrics.OutcomeSlackSendFailed)
			continue
		}
		s.metrics.RecordNotificationDelivery(ctx, eventType, metrics.OutcomeOK)
	}
	return nil
}

func outcomeFor(err error) string {
	if err != nil {
		return metrics.OutcomeError
	}
	return metrics.OutcomeOK
}
