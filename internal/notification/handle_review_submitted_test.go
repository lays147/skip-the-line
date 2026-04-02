package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/mocks"
	"github.com/skip-the-line/internal/notification"
	"go.uber.org/zap"
)

func TestNotify_PullRequestReview_Submitted(t *testing.T) {
	tests := []struct {
		name        string
		event       *github.PullRequestReviewEvent
		wantDMCount int
		slackErr    error
	}{
		{
			name: "author is notified when reviewer submits review",
			event: &github.PullRequestReviewEvent{
				Action: strPtr("submitted"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/1"),
					Title:   strPtr("My PR"),
				},
				Review: &github.PullRequestReview{
					User: &github.User{Login: strPtr("reviewer1")},
				},
			},
			wantDMCount: 1,
		},
		{
			name: "reviewer is not notified (author == reviewer excluded)",
			event: &github.PullRequestReviewEvent{
				Action: strPtr("submitted"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("reviewer1")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/2"),
					Title:   strPtr("Self Review PR"),
				},
				Review: &github.PullRequestReview{
					User: &github.User{Login: strPtr("reviewer1")},
				},
			},
			wantDMCount: 0,
		},
		{
			name: "Slack error is logged and skipped (best-effort)",
			event: &github.PullRequestReviewEvent{
				Action: strPtr("submitted"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/3"),
					Title:   strPtr("Error PR"),
				},
				Review: &github.PullRequestReview{
					User: &github.User{Login: strPtr("reviewer1")},
				},
			},
			slackErr:    errors.New("slack unavailable"),
			wantDMCount: 1, // attempted once, error swallowed
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dmCount := 0
			mockNotifier := &mocks.SlackNotifierMock{
				SendDMFunc: func(ctx context.Context, slackUserID, message string) error {
					dmCount++
					return tc.slackErr
				},
				LookupUserByEmailFunc: func(ctx context.Context, email string) (string, error) {
					return "U123", nil
				},
			}
			mockResolver := &mocks.GitHubTeamResolverMock{
				GetTeamMembersFunc: func(ctx context.Context, org, team string) ([]string, error) {
					return nil, nil
				},
			}

			svc := notification.NewNotificationService(mockResolver, mockNotifier, testSubs, zap.NewNop())
			err := svc.Notify(context.Background(), "pull_request_review", tc.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dmCount != tc.wantDMCount {
				t.Errorf("expected %d DM attempts, got %d", tc.wantDMCount, dmCount)
			}
		})
	}
}
