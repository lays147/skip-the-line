package notification

import (
	"context"
	"regexp"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/notifier"
)

// mentionRegex matches @username patterns in comment bodies.
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_-]+)`)

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

	// Resolve the commenter's platform user ID so the message contains a proper
	// clickable mention instead of a raw GitHub login.
	// Falls back to the GitHub login if the commenter is not subscribed.
	commenterRef := commenterLogin
	if email, ok := s.subs.EmailFor(commenterLogin); ok {
		if userID, err := s.notifier.LookupUserByEmail(ctx, email); err == nil {
			commenterRef = userID
		}
	}

	pr := e.GetPullRequest()
	msg := notifier.Message{
		EventType: notifier.EventReviewComment,
		AuthorID:  commenterRef,
		PRNumber:  pr.GetNumber(),
		PRTitle:   pr.GetTitle(),
		PRURL:     pr.GetHTMLURL(),
	}

	// Exclude the commenter from notifications.
	return s.sendToRecipients(ctx, recipients, commenterLogin, msg, "pull_request_review_comment")
}
