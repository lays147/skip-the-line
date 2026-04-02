package notification

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"
)

func (s *NotificationService) handleReviewSubmitted(ctx context.Context, e *github.PullRequestReviewEvent) error {
	authorLogin := e.GetPullRequest().GetUser().GetLogin()
	reviewerLogin := e.GetReview().GetUser().GetLogin()

	// Exclude the reviewer themselves (don't notify the reviewer).
	if authorLogin == reviewerLogin {
		return nil
	}

	pr := e.GetPullRequest()
	approved := e.GetReview().GetState() == "approved"
	msg := buildReviewSubmittedBlocks(reviewerLogin, pr.GetNumber(), pr.GetTitle(), pr.GetHTMLURL(), approved)

	recipients := map[string]struct{}{authorLogin: {}}
	return s.sendToRecipients(ctx, recipients, "", msg)
}

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
