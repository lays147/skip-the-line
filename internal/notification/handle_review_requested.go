package notification

import (
	"context"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/logger"
	"github.com/skip-the-line/internal/notifier"
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

	// Resolve the PR author's platform user ID so reviewers receive a clickable mention.
	// Falls back to the GitHub login if the author is not subscribed.
	authorRef := authorLogin
	if email, ok := s.subs.EmailFor(authorLogin); ok {
		if userID, err := s.notifier.LookupUserByEmail(ctx, email); err == nil {
			authorRef = userID
		}
	}

	pr := e.GetPullRequest()
	msg := notifier.Message{
		EventType: notifier.EventReviewRequested,
		AuthorID:  authorRef,
		PRNumber:  pr.GetNumber(),
		PRTitle:   pr.GetTitle(),
		PRURL:     pr.GetHTMLURL(),
	}
	return s.sendToRecipients(ctx, recipients, authorLogin, msg, "pull_request")
}
