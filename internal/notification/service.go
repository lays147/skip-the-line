package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/metrics"
	"github.com/skip-the-line/internal/subscription"
	"go.uber.org/zap"
)

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
func (s *NotificationService) sendToRecipients(ctx context.Context, recipients map[string]struct{}, exclude, msg, eventType string) error {
	for username := range recipients {
		if username == exclude {
			continue
		}
		email, ok := s.subs.EmailFor(username)
		if !ok {
			// not a subscriber — skip silently
			continue
		}
		slackUserID, err := s.notifier.LookupUserByEmail(ctx, email)
		if err != nil {
			s.logger.Warn("failed to look up Slack user",
				zap.String("github_username", username),
				zap.String("email", email),
				zap.Error(err),
			)
			s.metrics.RecordNotificationDelivery(ctx, eventType, metrics.OutcomeSlackLookupFailed)
			continue
		}
		if err := s.notifier.SendDM(ctx, slackUserID, msg); err != nil {
			s.logger.Error("failed to send Slack DM",
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

func mustMarshalBlocks(blocks []any) string {
	b, err := json.Marshal(blocks)
	if err != nil {
		panic(fmt.Sprintf("notification: failed to marshal blocks: %v", err))
	}
	return string(b)
}
