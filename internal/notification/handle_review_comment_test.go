package notification_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/mocks"
	"github.com/skip-the-line/internal/notification"
	"go.uber.org/zap"
)

func TestNotify_PullRequestReviewComment(t *testing.T) {
	tests := []struct {
		name        string
		event       *github.PullRequestReviewCommentEvent
		wantDMCount int
		wantEmails  []string
	}{
		{
			name: "author is notified on review comment",
			event: &github.PullRequestReviewCommentEvent{
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/1"),
					Title:   strPtr("My PR"),
				},
				Comment: &github.PullRequestComment{
					User: &github.User{Login: strPtr("reviewer1")},
					Body: strPtr("Looks good!"),
				},
			},
			wantDMCount: 1,
			wantEmails:  []string{"author@example.com"},
		},
		{
			name: "mentioned subscribers are notified",
			event: &github.PullRequestReviewCommentEvent{
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/2"),
					Title:   strPtr("Mention PR"),
				},
				Comment: &github.PullRequestComment{
					User: &github.User{Login: strPtr("reviewer1")},
					Body: strPtr("Hey @reviewer2 can you take a look?"),
				},
			},
			wantDMCount: 2,
			wantEmails:  []string{"author@example.com", "reviewer2@example.com"},
		},
		{
			name: "duplicates deduplicated (author also mentioned)",
			event: &github.PullRequestReviewCommentEvent{
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/3"),
					Title:   strPtr("Dup Mention PR"),
				},
				Comment: &github.PullRequestComment{
					User: &github.User{Login: strPtr("reviewer1")},
					Body: strPtr("@author please check this"),
				},
			},
			// author appears both as PR author and @mention — only one DM
			wantDMCount: 1,
			wantEmails:  []string{"author@example.com"},
		},
		{
			name: "commenter excluded from notifications",
			event: &github.PullRequestReviewCommentEvent{
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("reviewer1")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/4"),
					Title:   strPtr("Self Comment PR"),
				},
				Comment: &github.PullRequestComment{
					User: &github.User{Login: strPtr("reviewer1")},
					Body: strPtr("I'm commenting on my own PR"),
				},
			},
			// reviewer1 is both author and commenter — excluded
			wantDMCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dmUserIDs := []string{}
			mockNotifier := &mocks.SlackNotifierMock{
				SendDMFunc: func(ctx context.Context, slackUserID, message string) error {
					dmUserIDs = append(dmUserIDs, slackUserID)
					return nil
				},
				LookupUserByEmailFunc: func(ctx context.Context, email string) (string, error) {
					return "U-" + email, nil
				},
			}
			mockResolver := &mocks.GitHubTeamResolverMock{
				GetTeamMembersFunc: func(ctx context.Context, org, team string) ([]string, error) {
					return nil, nil
				},
			}

			svc := notification.NewNotificationService(mockResolver, mockNotifier, testSubs, zap.NewNop())
			err := svc.Notify(context.Background(), "pull_request_review_comment", tc.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(dmUserIDs) != tc.wantDMCount {
				t.Errorf("expected %d DMs, got %d (user IDs: %v)", tc.wantDMCount, len(dmUserIDs), dmUserIDs)
			}
			for _, wantEmail := range tc.wantEmails {
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
		})
	}
}
