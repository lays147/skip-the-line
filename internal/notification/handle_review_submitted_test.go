package notification_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/mocks"
	"github.com/skip-the-line/internal/notification"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

func TestNotify_PullRequestReview_Submitted(t *testing.T) {
	tests := []struct {
		name         string
		event        *github.PullRequestReviewEvent
		wantDMCount  int
		wantActorRef string // expected value inside <@...> in the DM message
		slackErr     error
		lookupErr    error
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
			wantDMCount:  1,
			wantActorRef: "U-reviewer1@example.com", // reviewer1 is subscribed → Slack ID used
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
			slackErr:     errors.New("slack unavailable"),
			wantDMCount:  1, // attempted once, error swallowed
			wantActorRef: "U-reviewer1@example.com",
		},
		{
			name: "LookupUserByEmail error: no DM sent, Notify returns nil",
			event: &github.PullRequestReviewEvent{
				Action: strPtr("submitted"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/5"),
					Title:   strPtr("Lookup Error PR"),
				},
				Review: &github.PullRequestReview{
					User: &github.User{Login: strPtr("reviewer1")},
				},
			},
			lookupErr:   errors.New("slack lookup unavailable"),
			wantDMCount: 0, // lookup failed → no DM attempted, error swallowed
		},
		{
			name: "unsubscribed reviewer falls back to GitHub login in message",
			event: &github.PullRequestReviewEvent{
				Action: strPtr("submitted"),
				PullRequest: &github.PullRequest{
					User:    &github.User{Login: strPtr("author")},
					HTMLURL: strPtr("https://github.com/org/repo/pull/4"),
					Title:   strPtr("External Review PR"),
				},
				Review: &github.PullRequestReview{
					User: &github.User{Login: strPtr("external-reviewer")},
				},
			},
			wantDMCount:  1,
			wantActorRef: "external-reviewer", // not in testSubs → GitHub login used
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedBlocks [][]slack.Block
			dmCount := 0
			mockNotifier := &mocks.SlackNotifierMock{
				SendDMFunc: func(ctx context.Context, slackUserID string, blocks []slack.Block) error {
					dmCount++
					capturedBlocks = append(capturedBlocks, blocks)
					return tc.slackErr
				},
				LookupUserByEmailFunc: func(ctx context.Context, email string) (string, error) {
					if tc.lookupErr != nil {
						return "", tc.lookupErr
					}
					return "U-" + email, nil
				},
			}
			mockResolver := &mocks.GitHubTeamResolverMock{
				GetTeamMembersFunc: func(ctx context.Context, org, team string) ([]string, error) {
					return nil, nil
				},
			}

			svc := notification.NewNotificationService(mockResolver, mockNotifier, testSubs, zap.NewNop(), noopMetrics())
			err := svc.Notify(context.Background(), "pull_request_review", tc.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dmCount != tc.wantDMCount {
				t.Errorf("expected %d DM attempts, got %d", tc.wantDMCount, dmCount)
			}
			if tc.wantActorRef != "" {
				for _, blocks := range capturedBlocks {
					jsonStr, _ := json.Marshal(blocks)
					if !strings.Contains(string(jsonStr), tc.wantActorRef) {
						t.Errorf("expected message to contain actor ref %q, got: %s", tc.wantActorRef, jsonStr)
					}
				}
			}
		})
	}
}
