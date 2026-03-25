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
// If baseURL is non-empty the client points at that URL instead of api.github.com
// (useful for local development against a mock server).
func NewClient(token, baseURL string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	gh := github.NewClient(tc)

	if baseURL != "" {
		// WithEnterpriseURLs expects a trailing slash.
		if baseURL[len(baseURL)-1] != '/' {
			baseURL += "/"
		}
		var err error
		gh, err = gh.WithEnterpriseURLs(baseURL, baseURL)
		if err != nil {
			// baseURL is validated at startup via config; panic is appropriate here.
			panic(fmt.Sprintf("github: invalid base URL %q: %v", baseURL, err))
		}
	}

	return &Client{gh: gh}
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
