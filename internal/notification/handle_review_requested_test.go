package notification_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/mocks"
	"github.com/skip-the-line/internal/notification"
	"go.uber.org/zap"
)

func TestNotify_PullRequest_ReviewRequested(t *testing.T) {
	tests := []struct {
		name            string
		event           *github.PullRequestEvent
		resolverMembers map[string][]string // team slug -> members
		wantDMEmails    []string
		wantDMCount     int
		wantActorRef    string // expected value inside <@...> in the DM message
	}{
		{
			name: "single individual reviewer notified",
			event: &github.PullRequestEvent{
				Action: strPtr("review_requested"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/1"),
					Title:   strPtr("My PR"),
					RequestedReviewers: []*github.User{
						{Login: strPtr("reviewer1")},
					},
				},
				Repo: &github.Repository{
					Owner: &github.User{Login: strPtr("org")},
				},
			},
			wantDMEmails: []string{"reviewer1@example.com"},
			wantDMCount:  1,
			wantActorRef: "U-author@example.com", // author is subscribed → Slack ID used
		},
		{
			name: "team reviewer expanded to members",
			event: &github.PullRequestEvent{
				Action: strPtr("review_requested"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/2"),
					Title:   strPtr("Team PR"),
					RequestedTeams: []*github.Team{
						{Slug: strPtr("backend-team")},
					},
				},
				Repo: &github.Repository{
					Owner: &github.User{Login: strPtr("org")},
				},
			},
			resolverMembers: map[string][]string{
				"backend-team": {"teamMember1", "teamMember2"},
			},
			wantDMEmails: []string{"tm1@example.com", "tm2@example.com"},
			wantDMCount:  2,
			wantActorRef: "U-author@example.com",
		},
		{
			name: "author excluded from notifications",
			event: &github.PullRequestEvent{
				Action: strPtr("review_requested"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/3"),
					Title:   strPtr("Author PR"),
					RequestedReviewers: []*github.User{
						{Login: strPtr("author")},
						{Login: strPtr("reviewer1")},
					},
				},
				Repo: &github.Repository{
					Owner: &github.User{Login: strPtr("org")},
				},
			},
			wantDMEmails: []string{"reviewer1@example.com"},
			wantDMCount:  1,
			wantActorRef: "U-author@example.com",
		},
		{
			name: "duplicate recipients deduplicated",
			event: &github.PullRequestEvent{
				Action: strPtr("review_requested"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/4"),
					Title:   strPtr("Dup PR"),
					RequestedReviewers: []*github.User{
						{Login: strPtr("reviewer1")},
					},
					RequestedTeams: []*github.Team{
						{Slug: strPtr("team-a")},
					},
				},
				Repo: &github.Repository{
					Owner: &github.User{Login: strPtr("org")},
				},
			},
			resolverMembers: map[string][]string{
				"team-a": {"reviewer1", "reviewer2"},
			},
			// reviewer1 appears both as individual and team member — only one DM
			wantDMCount:  2,
			wantActorRef: "U-author@example.com",
		},
		{
			name: "unsubscribed author falls back to GitHub login in message",
			event: &github.PullRequestEvent{
				Action: strPtr("review_requested"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("external-author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/5"),
					Title:   strPtr("External PR"),
					RequestedReviewers: []*github.User{
						{Login: strPtr("reviewer1")},
					},
				},
				Repo: &github.Repository{
					Owner: &github.User{Login: strPtr("org")},
				},
			},
			wantDMEmails: []string{"reviewer1@example.com"},
			wantDMCount:  1,
			wantActorRef: "external-author", // not in testSubs → GitHub login used
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedMessages []string
			dmUserIDs := []string{}
			mockNotifier := &mocks.SlackNotifierMock{
				SendDMFunc: func(ctx context.Context, slackUserID, message string) error {
					dmUserIDs = append(dmUserIDs, slackUserID)
					capturedMessages = append(capturedMessages, message)
					return nil
				},
				LookupUserByEmailFunc: func(ctx context.Context, email string) (string, error) {
					return "U-" + email, nil
				},
			}
			mockResolver := &mocks.GitHubTeamResolverMock{
				GetTeamMembersFunc: func(ctx context.Context, org, team string) ([]string, error) {
					if tc.resolverMembers != nil {
						if members, ok := tc.resolverMembers[team]; ok {
							return members, nil
						}
					}
					return nil, nil
				},
			}

			svc := notification.NewNotificationService(mockResolver, mockNotifier, testSubs, zap.NewNop(), noopMetrics())
			err := svc.Notify(context.Background(), "pull_request", tc.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(dmUserIDs) != tc.wantDMCount {
				t.Errorf("expected %d DMs, got %d (user IDs: %v)", tc.wantDMCount, len(dmUserIDs), dmUserIDs)
			}
			for _, wantEmail := range tc.wantDMEmails {
				wantID := "U-" + wantEmail
				found := false
				for _, got := range dmUserIDs {
					if got == wantID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected DM to user ID %s (email %s) but not found in %v", wantID, wantEmail, dmUserIDs)
				}
			}
			if tc.wantActorRef != "" {
				for _, msg := range capturedMessages {
					if !strings.Contains(msg, tc.wantActorRef) {
						t.Errorf("expected message to contain actor ref %q, got: %s", tc.wantActorRef, msg)
					}
				}
			}
		})
	}
}
