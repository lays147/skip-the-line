package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/logger"
	"github.com/slack-go/slack"
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
		start := time.Now()
		members, err := s.resolver.GetTeamMembers(ctx, org, teamSlug)
		s.metrics.RecordTeamMembersDuration(ctx, time.Since(start), outcomeFor(err))
		if err != nil {
			logger.FromContext(ctx, s.logger).Error("failed to resolve team members",
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
	blocks := buildReviewRequestedBlocks(authorRef, pr.GetNumber(), pr.GetTitle(), pr.GetHTMLURL())
	return s.sendToRecipients(ctx, recipients, authorLogin, blocks, "pull_request")
}

func buildReviewRequestedBlocks(requesterLogin string, prNumber int, prTitle, prURL string) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("*Your review was requested by <@%s>*", requesterLogin),
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
			btnTxt := slack.NewTextBlockObject("plain_text", "Review now!", false, false)
			btn := slack.NewButtonBlockElement("", "review_button", btnTxt)
			btn.URL = prURL
			return slack.NewActionBlock("", btn)
		}(),
	}
}
