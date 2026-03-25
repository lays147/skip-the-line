package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/mocks"
	"github.com/skip-the-line/internal/notification"
	"github.com/skip-the-line/internal/subscription"
)

func strPtr(s string) *string { return &s }

var testSubs = subscription.Registry{
	"octocat":     "octocat@example.com",
	"reviewer1":   "reviewer1@example.com",
	"reviewer2":   "reviewer2@example.com",
	"teamMember1": "tm1@example.com",
	"teamMember2": "tm2@example.com",
	"author":      "author@example.com",
}

// --- 8.2: pull_request review_requested ---

func TestNotify_PullRequest_ReviewRequested(t *testing.T) {
	tests := []struct {
		name            string
		event           *github.PullRequestEvent
		resolverMembers map[string][]string // team slug -> members
		wantDMEmails    []string
		wantDMCount     int
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
			wantDMCount: 2,
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
					// return a predictable user ID derived from the email so we can assert counts
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

			svc := notification.NewNotificationService(mockResolver, mockNotifier, testSubs)
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
		})
	}
}

// --- 8.3: pull_request_review submitted ---

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

			svc := notification.NewNotificationService(mockResolver, mockNotifier, testSubs)
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

// --- 8.4: pull_request_review_comment ---

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

			svc := notification.NewNotificationService(mockResolver, mockNotifier, testSubs)
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
