package notification

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"
	"go.uber.org/zap"
)

func (s *NotificationService) handleReviewRequested(ctx context.Context, e *github.PullRequestEvent) error {
	authorLogin := e.GetPullRequest().GetUser().GetLogin()
	recipients := make(map[string]struct{})

	// Collect individual reviewers.
	for _, reviewer := range e.GetPullRequest().RequestedReviewers {
		if username := reviewer.GetLogin(); username != "" {
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

	// Resolve the PR author's Slack ID so reviewers receive a clickable mention.
	// Falls back to the GitHub login if the author is not subscribed.
	authorRef := authorLogin
	if email, ok := s.subs.EmailFor(authorLogin); ok {
		if slackID, err := s.notifier.LookupUserByEmail(ctx, email); err == nil {
			authorRef = slackID
		}
	}

	pr := e.GetPullRequest()
	msg, err := buildReviewRequestedBlocks(authorRef, pr.GetNumber(), pr.GetTitle(), pr.GetHTMLURL())
	if err != nil {
		return err
	}
	return s.sendToRecipients(ctx, recipients, authorLogin, msg, "pull_request")
}

func buildReviewRequestedBlocks(requesterLogin string, prNumber int, prTitle, prURL string) (string, error) {
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
	return marshalBlocks(blocks)
}
