package notification

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/v62/github"
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
	subs     []subscription.Subscription
	logger   *zap.Logger
}

// NewNotificationService constructs a NotificationService.
func NewNotificationService(resolver GitHubTeamResolver, notifier SlackNotifier, subs []subscription.Subscription) *NotificationService {
	return &NotificationService{
		resolver: resolver,
		notifier: notifier,
		subs:     subs,
		logger:   zap.NewNop(),
	}
}

// mentionRegex matches @username patterns in comment bodies.
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9-]+)`)

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

// handleReviewRequested notifies requested reviewers (individual + team members) for a PR.
func (s *NotificationService) handleReviewRequested(ctx context.Context, e *github.PullRequestEvent) error {
	authorLogin := e.GetPullRequest().GetUser().GetLogin()
	recipients := make(map[string]struct{})

	// Collect individual reviewers.
	for _, reviewer := range e.GetPullRequest().RequestedReviewers {
		username := reviewer.GetLogin()
		if username != "" {
			recipients[username] = struct{}{}
		}
	}

	// Expand team reviewers.
	org := e.GetRepo().GetOwner().GetLogin()
	for _, team := range e.GetPullRequest().RequestedTeams {
		teamSlug := team.GetSlug()
		members, err := s.resolver.GetTeamMembers(ctx, org, teamSlug)
		if err != nil {
			s.logger.Error("failed to resolve team members",
				zap.String("team", teamSlug),
				zap.Error(err),
			)
			continue
		}
		for _, m := range members {
			recipients[m] = struct{}{}
		}
	}

	// Exclude PR author and send DMs.
	prURL := e.GetPullRequest().GetHTMLURL()
	prTitle := e.GetPullRequest().GetTitle()
	msg := fmt.Sprintf("You have been requested to review a pull request: [%s](%s)", prTitle, prURL)

	return s.sendToRecipients(ctx, recipients, authorLogin, msg)
}

// handleReviewSubmitted notifies the PR author when a review is submitted.
func (s *NotificationService) handleReviewSubmitted(ctx context.Context, e *github.PullRequestReviewEvent) error {
	authorLogin := e.GetPullRequest().GetUser().GetLogin()
	reviewerLogin := e.GetReview().GetUser().GetLogin()

	// Exclude the reviewer themselves (don't notify the reviewer).
	if authorLogin == reviewerLogin {
		return nil
	}

	prURL := e.GetPullRequest().GetHTMLURL()
	prTitle := e.GetPullRequest().GetTitle()
	reviewerName := reviewerLogin
	msg := fmt.Sprintf("%s submitted a review on your pull request: [%s](%s)", reviewerName, prTitle, prURL)

	recipients := map[string]struct{}{
		authorLogin: {},
	}
	return s.sendToRecipients(ctx, recipients, "" /* no exclusion beyond reviewer check above */, msg)
}

// handleReviewComment notifies the PR author and any mentioned subscribers.
func (s *NotificationService) handleReviewComment(ctx context.Context, e *github.PullRequestReviewCommentEvent) error {
	authorLogin := e.GetPullRequest().GetUser().GetLogin()
	commenterLogin := e.GetComment().GetUser().GetLogin()

	recipients := make(map[string]struct{})

	// Always notify the PR author.
	if authorLogin != "" {
		recipients[authorLogin] = struct{}{}
	}

	// Parse @mentions from the comment body.
	body := e.GetComment().GetBody()
	matches := mentionRegex.FindAllStringSubmatch(body, -1)
	for _, match := range matches {
		if len(match) > 1 {
			recipients[match[1]] = struct{}{}
		}
	}

	prURL := e.GetPullRequest().GetHTMLURL()
	prTitle := e.GetPullRequest().GetTitle()
	msg := fmt.Sprintf("%s commented on your pull request: [%s](%s)", commenterLogin, prTitle, prURL)

	// Exclude the commenter from notifications.
	return s.sendToRecipients(ctx, recipients, commenterLogin, msg)
}

// sendToRecipients sends a DM to each recipient that is a subscriber, excluding the given login.
func (s *NotificationService) sendToRecipients(ctx context.Context, recipients map[string]struct{}, exclude string, msg string) error {
	for username := range recipients {
		if username == exclude {
			continue
		}
		sub, ok := s.findSubscription(username)
		if !ok {
			continue
		}
		if err := s.notifier.SendDM(ctx, sub.Email, msg); err != nil {
			s.logger.Error("failed to send Slack DM",
				zap.String("github_username", username),
				zap.String("email", sub.Email),
				zap.Error(err),
			)
			// best-effort: log and continue
		}
	}
	return nil
}

// findSubscription looks up a subscription by GitHub username.
func (s *NotificationService) findSubscription(username string) (subscription.Subscription, bool) {
	for _, sub := range s.subs {
		if sub.GitHubUsername == username {
			return sub, true
		}
	}
	return subscription.Subscription{}, false
}
