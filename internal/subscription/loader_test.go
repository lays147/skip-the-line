package subscription

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoad(t *testing.T) {
	t.Run("successful parse returns correct slice", func(t *testing.T) {
		subs, err := Load()
		if err != nil {
			t.Fatalf("Load() returned unexpected error: %v", err)
		}
		if len(subs) == 0 {
			t.Fatal("Load() returned empty slice, expected at least one subscription")
		}
		// Verify the first entry matches the known subscriptions.yaml content
		first := subs[0]
		if first.GitHubUsername == "" {
			t.Error("first subscription has empty GitHubUsername")
		}
		if first.Email == "" {
			t.Error("first subscription has empty Email")
		}
	})
}

func TestParseSubscriptions(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantLen int
		wantErr bool
	}{
		{
			name: "valid yaml with two entries",
			input: []byte(`subscriptions:
  - github_username: octocat
    email: octocat@example.com
  - github_username: monalisa
    email: monalisa@example.com
`),
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "valid yaml with single entry",
			input: []byte(`subscriptions:
  - github_username: octocat
    email: octocat@example.com
`),
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "malformed yaml returns error",
			input:   []byte("subscriptions:\n  - github_username: [unclosed bracket\n"),
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "empty yaml returns empty slice",
			input:   []byte(`subscriptions: []`),
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var f subscriptionFile
			err := yaml.Unmarshal(tc.input, &f)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(f.Subscriptions) != tc.wantLen {
				t.Errorf("got %d subscriptions, want %d", len(f.Subscriptions), tc.wantLen)
			}
		})
	}
}

func TestSubscriptionFields(t *testing.T) {
	input := []byte(`subscriptions:
  - github_username: octocat
    email: octocat@example.com
`)
	var f subscriptionFile
	if err := yaml.Unmarshal(input, &f); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub := f.Subscriptions[0]
	if sub.GitHubUsername != "octocat" {
		t.Errorf("GitHubUsername = %q, want %q", sub.GitHubUsername, "octocat")
	}
	if sub.Email != "octocat@example.com" {
		t.Errorf("Email = %q, want %q", sub.Email, "octocat@example.com")
	}
}
