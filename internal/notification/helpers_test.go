package notification_test

import "github.com/skip-the-line/internal/subscription"

func strPtr(s string) *string { return &s }

var testSubs = subscription.Registry{
	"octocat":     "octocat@example.com",
	"reviewer1":   "reviewer1@example.com",
	"reviewer2":   "reviewer2@example.com",
	"teamMember1": "tm1@example.com",
	"teamMember2": "tm2@example.com",
	"author":      "author@example.com",
}
