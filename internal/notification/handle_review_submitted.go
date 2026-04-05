package notification

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"
	"github.com/slack-go/slack"
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
	blocks := buildReviewSubmittedBlocks(reviewerRef, pr.GetNumber(), pr.GetTitle(), pr.GetHTMLURL(), approved)

	recipients := map[string]struct{}{authorLogin: {}}
	return s.sendToRecipients(ctx, recipients, "", blocks, "pull_request_review")
}

func buildReviewSubmittedBlocks(reviewerLogin string, prNumber int, prTitle, prURL string, approved bool) []slack.Block {
	headerText := fmt.Sprintf("*<@%s> submitted a review on your pull request*", reviewerLogin)
	buttonText := "View review"
	if approved {
		headerText = fmt.Sprintf("*<@%s> approved your pull request — it's ready to merge! :rocket:*", reviewerLogin)
		buttonText = "Merge now!"
	}

	return []slack.Block{
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: headerText,
			},
			nil, nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("*PR*: #%d | %s", prNumber, prTitle),
			},
			nil, nil,
		),
		func() slack.Block {
			btnTxt := slack.NewTextBlockObject("plain_text", buttonText, false, false)
			btn := slack.NewButtonBlockElement("", "review_button", btnTxt)
			btn.URL = prURL
			return slack.NewActionBlock("", btn)
		}(),
	}
}
