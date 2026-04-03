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

	// Resolve the reviewer's Slack ID so the PR author receives a clickable mention.
	// Falls back to the GitHub login if the reviewer is not subscribed.
	reviewerRef := reviewerLogin
	if email, ok := s.subs.EmailFor(reviewerLogin); ok {
		if slackID, err := s.notifier.LookupUserByEmail(ctx, email); err == nil {
			reviewerRef = slackID
		}
	}

	pr := e.GetPullRequest()
	approved := e.GetReview().GetState() == "approved"
	msg, err := buildReviewSubmittedBlocks(reviewerRef, pr.GetNumber(), pr.GetTitle(), pr.GetHTMLURL(), approved)
	if err != nil {
		return err
	}

	recipients := map[string]struct{}{authorLogin: {}}
	return s.sendToRecipients(ctx, recipients, "", msg, "pull_request_review")
}

func buildReviewSubmittedBlocks(reviewerLogin string, prNumber int, prTitle, prURL string, approved bool) (string, error) {
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
	return marshalBlocks(blocks)
}
