// Package notifier defines the platform-agnostic notification types shared
// between the notification service and the platform-specific notifier clients
// (Slack, Google Chat, etc.).
package notifier

// EventType identifies the GitHub event that triggered a notification.
const (
	EventReviewRequested = "review_requested"
	EventReviewSubmitted = "review_submitted"
	EventReviewComment   = "review_comment"
)

// Message carries platform-agnostic notification content for a single
// GitHub PR event. The platform-specific notifier client is responsible
// for rendering this into its native message format (Slack Block Kit,
// Google Chat cards, etc.).
type Message struct {
	// EventType is one of the EventReview* constants above.
	EventType string
	// AuthorID is the platform-native user ID of the actor (PR author,
	// reviewer, or commenter) to use for @mentions. It is resolved by
	// the notification service via LookupUserByEmail before building the
	// Message, falling back to the GitHub login when the user is not found.
	AuthorID string
	// PRNumber is the pull request number.
	PRNumber int
	// PRTitle is the pull request title.
	PRTitle string
	// PRURL is the HTML URL of the pull request.
	PRURL string
	// ReviewState is the review state for EventReviewSubmitted
	// (e.g. "approved", "changes_requested", "commented").
	ReviewState string
}
