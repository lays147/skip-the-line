package notification

import (
	"context"

	"github.com/skip-the-line/internal/notifier"
)

//go:generate moq -out ../mocks/mock_github_team_resolver.go -pkg mocks . GitHubTeamResolver
//go:generate moq -out ../mocks/mock_notifier.go -pkg mocks . Notifier

// GitHubTeamResolver resolves GitHub team membership.
// GetTeamMembers returns all member usernames for a given org/team slug.
// The webhook payload already identifies individual and team reviewers;
// this interface is used only to expand team slugs into individual usernames.
type GitHubTeamResolver interface {
	GetTeamMembers(ctx context.Context, org, team string) ([]string, error)
}

// Notifier sends direct-message notifications on a chat platform.
// Implementations include the Slack client and the Google Chat client.
type Notifier interface {
	// LookupUserByEmail returns the platform-native user ID for the given email address.
	LookupUserByEmail(ctx context.Context, email string) (string, error)
	// SendNotification delivers a notification message to the user identified
	// by the platform-native recipientID returned by LookupUserByEmail.
	SendNotification(ctx context.Context, recipientID string, msg notifier.Message) error
}
