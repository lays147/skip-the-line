package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

// Client wraps the go-github SDK and implements notification.GitHubTeamResolver.
type Client struct {
	gh *github.Client
}

// NewClient creates a new GitHub Client authenticated with the provided token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		gh: github.NewClient(tc),
	}
}

// GetTeamMembers returns all member usernames for the given org/team slug.
// It satisfies the notification.GitHubTeamResolver interface.
func (c *Client) GetTeamMembers(ctx context.Context, org, team string) ([]string, error) {
	var members []string
	opts := &github.TeamListTeamMembersOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		page, resp, err := c.gh.Teams.ListTeamMembersBySlug(ctx, org, team, opts)
		if err != nil {
			return nil, fmt.Errorf("get team members %s/%s: %w", org, team, err)
		}

		for _, m := range page {
			if m.Login != nil {
				members = append(members, *m.Login)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return members, nil
}
