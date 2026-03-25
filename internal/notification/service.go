package notification

import (
	"context"
	"encoding/json"
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

	prNumber := e.GetPullRequest().GetNumber()
	prTitle := e.GetPullRequest().GetTitle()
	prURL := e.GetPullRequest().GetHTMLURL()
	msg := buildReviewRequestedBlocks(authorLogin, prNumber, prTitle, prURL)

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

	prNumber := e.GetPullRequest().GetNumber()
	prTitle := e.GetPullRequest().GetTitle()
	prURL := e.GetPullRequest().GetHTMLURL()
	approved := e.GetReview().GetState() == "approved"
	msg := buildReviewSubmittedBlocks(reviewerLogin, prNumber, prTitle, prURL, approved)

	recipients := map[string]struct{}{authorLogin: {}}
	return s.sendToRecipients(ctx, recipients, "", msg)
}

// handleReviewComment notifies the PR author and any mentioned subscribers.
func (s *NotificationService) handleReviewComment(ctx context.Context, e *github.PullRequestReviewCommentEvent) error {
	authorLogin := e.GetPullRequest().GetUser().GetLogin()
	commenterLogin := e.GetComment().GetUser().GetLogin()

	recipients := make(map[string]struct{})
	if authorLogin != "" {
		recipients[authorLogin] = struct{}{}
	}

	// Parse @mentions from the comment body.
	body := e.GetComment().GetBody()
	for _, match := range mentionRegex.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 {
			recipients[match[1]] = struct{}{}
		}
	}

	prNumber := e.GetPullRequest().GetNumber()
	prTitle := e.GetPullRequest().GetTitle()
	prURL := e.GetPullRequest().GetHTMLURL()
	msg := buildReviewCommentBlocks(commenterLogin, prNumber, prTitle, prURL)

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

// --- Block Kit message builders ---

// buildReviewRequestedBlocks returns a Block Kit JSON array for a review_requested event.
func buildReviewRequestedBlocks(requesterLogin string, prNumber int, prTitle, prURL string) string {
	blocks := []any{
		map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Your review was requested by <@%s>*", requesterLogin),
			},
		},
		map[string]any{"type": "divider"},
		map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*PR*: #%d | %s", prNumber, prTitle),
			},
		},
		map[string]any{
			"type": "actions",
			"elements": []any{
				map[string]any{
					"type": "button",
					"text": map[string]any{
						"type":  "plain_text",
						"text":  "Review now!",
						"emoji": true,
					},
					"value": prURL,
					"url":   prURL,
				},
			},
		},
	}
	return mustMarshalBlocks(blocks)
}

// buildReviewSubmittedBlocks returns a Block Kit JSON array for a pull_request_review submitted event.
// When approved is true, the message indicates the PR is ready to merge.
func buildReviewSubmittedBlocks(reviewerLogin string, prNumber int, prTitle, prURL string, approved bool) string {
	headerText := fmt.Sprintf("*<@%s> submitted a review on your pull request*", reviewerLogin)
	buttonText := "View review"
	if approved {
		headerText = fmt.Sprintf("*<@%s> approved your pull request — it's ready to merge! :rocket:*", reviewerLogin)
		buttonText = "Merge now!"
	}
	blocks := []any{
		map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": headerText,
			},
		},
		map[string]any{"type": "divider"},
		map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*PR*: #%d | %s", prNumber, prTitle),
			},
		},
		map[string]any{
			"type": "actions",
			"elements": []any{
				map[string]any{
					"type": "button",
					"text": map[string]any{
						"type":  "plain_text",
						"text":  buttonText,
						"emoji": true,
					},
					"value": prURL,
					"url":   prURL,
				},
			},
		},
	}
	return mustMarshalBlocks(blocks)
}

// buildReviewCommentBlocks returns a Block Kit JSON array for a pull_request_review_comment event.
func buildReviewCommentBlocks(commenterLogin string, prNumber int, prTitle, prURL string) string {
	blocks := []any{
		map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*<@%s> commented on your pull request*", commenterLogin),
			},
		},
		map[string]any{"type": "divider"},
		map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*PR*: #%d | %s", prNumber, prTitle),
			},
		},
		map[string]any{
			"type": "actions",
			"elements": []any{
				map[string]any{
					"type": "button",
					"text": map[string]any{
						"type":  "plain_text",
						"text":  "View comment",
						"emoji": true,
					},
					"value": prURL,
					"url":   prURL,
				},
			},
		},
	}
	return mustMarshalBlocks(blocks)
}

func mustMarshalBlocks(blocks []any) string {
	b, err := json.Marshal(blocks)
	if err != nil {
		panic(fmt.Sprintf("notification: failed to marshal blocks: %v", err))
	}
	return string(b)
}
