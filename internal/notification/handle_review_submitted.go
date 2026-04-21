package notification

import (
	"context"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/notifier"
)

func (s *NotificationService) handleReviewSubmitted(ctx context.Context, e *github.PullRequestReviewEvent) error {
	authorLogin := e.GetPullRequest().GetUser().GetLogin()
	reviewerLogin := e.GetReview().GetUser().GetLogin()

	// Exclude the reviewer themselves (don't notify the reviewer).
	if authorLogin == reviewerLogin {
		return nil
	}

	// Resolve the reviewer's platform user ID so the PR author receives a clickable mention.
	// Falls back to the GitHub login if the reviewer is not subscribed.
	reviewerRef := reviewerLogin
	if email, ok := s.subs.EmailFor(reviewerLogin); ok {
		if userID, err := s.notifier.LookupUserByEmail(ctx, email); err == nil {
			reviewerRef = userID
		}
	}

	pr := e.GetPullRequest()
	msg := notifier.Message{
		EventType:   notifier.EventReviewSubmitted,
		AuthorID:    reviewerRef,
		PRNumber:    pr.GetNumber(),
		PRTitle:     pr.GetTitle(),
		PRURL:       pr.GetHTMLURL(),
		ReviewState: e.GetReview().GetState(),
	}

	recipients := map[string]struct{}{authorLogin: {}}
	return s.sendToRecipients(ctx, recipients, "", msg, "pull_request_review")
}
