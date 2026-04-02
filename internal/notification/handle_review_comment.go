package notification

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/v62/github"
)

// mentionRegex matches @username patterns in comment bodies.
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9-]+)`)

func (s *NotificationService) handleReviewComment(ctx context.Context, e *github.PullRequestReviewCommentEvent) error {
	authorLogin := e.GetPullRequest().GetUser().GetLogin()
	commenterLogin := e.GetComment().GetUser().GetLogin()

	recipients := make(map[string]struct{})
	if authorLogin != "" {
		recipients[authorLogin] = struct{}{}
	}

	// Parse @mentions from the comment body.
	for _, match := range mentionRegex.FindAllStringSubmatch(e.GetComment().GetBody(), -1) {
		if len(match) > 1 {
			recipients[match[1]] = struct{}{}
		}
	}

	pr := e.GetPullRequest()
	msg := buildReviewCommentBlocks(commenterLogin, pr.GetNumber(), pr.GetTitle(), pr.GetHTMLURL())

	// Exclude the commenter from notifications.
	return s.sendToRecipients(ctx, recipients, commenterLogin, msg)
}

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
