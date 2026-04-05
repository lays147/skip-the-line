package notification

import (
	"context"

	"github.com/slack-go/slack"
)

//go:generate moq -out ../mocks/mock_github_team_resolver.go -pkg mocks . GitHubTeamResolver
//go:generate moq -out ../mocks/mock_slack_notifier.go -pkg mocks . SlackNotifier

// GitHubTeamResolver resolves GitHub team membership.
// GetTeamMembers returns all member usernames for a given org/team slug.
// The webhook payload already identifies individual and team reviewers;
// this interface is used only to expand team slugs into individual usernames.
type GitHubTeamResolver interface {
	GetTeamMembers(ctx context.Context, org, team string) ([]string, error)
}

// SlackNotifier sends direct messages via Slack.
type SlackNotifier interface {
	// LookupUserByEmail returns the Slack user ID for the given email address.
	LookupUserByEmail(ctx context.Context, email string) (string, error)
	// SendDM sends Block Kit blocks to the user identified by their Slack user ID.
	SendDM(ctx context.Context, slackUserID string, blocks []slack.Block) error
}
