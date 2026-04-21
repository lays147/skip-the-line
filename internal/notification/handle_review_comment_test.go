package notification_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/mocks"
	"github.com/skip-the-line/internal/notification"
	"github.com/skip-the-line/internal/notifier"
	"go.uber.org/zap"
)

func TestNotify_PullRequestReviewComment(t *testing.T) {
	tests := []struct {
		name             string
		event            *github.PullRequestReviewCommentEvent
		wantDMCount      int
		wantEmails       []string
		wantCommenterRef string // expected AuthorID in the notification message
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
			wantDMCount:      1,
			wantEmails:       []string{"author@example.com"},
			wantCommenterRef: "U-reviewer1@example.com", // reviewer1 is subscribed → platform ID used
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
			wantDMCount:      2,
			wantEmails:       []string{"author@example.com", "reviewer2@example.com"},
			wantCommenterRef: "U-reviewer1@example.com",
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
			wantDMCount:      1,
			wantEmails:       []string{"author@example.com"},
			wantCommenterRef: "U-reviewer1@example.com",
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
		{
			name: "underscore username in @mention is captured as recipient",
			event: &github.PullRequestReviewCommentEvent{
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/6"),
					Title:   strPtr("Underscore Mention PR"),
				},
				Comment: &github.PullRequestComment{
					User: &github.User{Login: strPtr("reviewer1")},
					Body: strPtr("Hey @my_reviewer please take a look"),
				},
			},
			// my_reviewer is not in testSubs so no DM, but author still gets one
			wantDMCount: 1,
			wantEmails:  []string{"author@example.com"},
		},
		{
			name: "unsubscribed commenter falls back to GitHub login in message",
			event: &github.PullRequestReviewCommentEvent{
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/5"),
					Title:   strPtr("External Comment PR"),
				},
				Comment: &github.PullRequestComment{
					User: &github.User{Login: strPtr("external-user")},
					Body: strPtr("Interesting approach"),
				},
			},
			wantDMCount:      1,
			wantEmails:       []string{"author@example.com"},
			wantCommenterRef: "external-user", // not in testSubs → GitHub login used
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedMsgs []notifier.Message
			dmUserIDs := []string{}
			mockNotifier := &mocks.NotifierMock{
				SendNotificationFunc: func(ctx context.Context, recipientID string, msg notifier.Message) error {
					dmUserIDs = append(dmUserIDs, recipientID)
					capturedMsgs = append(capturedMsgs, msg)
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

			svc := notification.NewNotificationService(mockResolver, mockNotifier, testSubs, zap.NewNop(), noopMetrics())
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
			if tc.wantCommenterRef != "" {
				for _, msg := range capturedMsgs {
					if msg.AuthorID != tc.wantCommenterRef {
						t.Errorf("expected AuthorID %q, got %q", tc.wantCommenterRef, msg.AuthorID)
					}
				}
			}
		})
	}
}
