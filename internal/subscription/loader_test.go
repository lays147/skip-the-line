package subscription

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoad(t *testing.T) {
	t.Run("successful parse returns non-empty registry", func(t *testing.T) {
		reg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned unexpected error: %v", err)
		}
		if len(reg) == 0 {
			t.Fatal("Load() returned empty registry, expected at least one subscription")
		}
		for username, email := range reg {
			if username == "" {
				t.Error("registry contains entry with empty GitHub username")
			}
			if email == "" {
				t.Errorf("registry entry for %q has empty email", username)
			}
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
			name:    "empty yaml returns empty registry",
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

func TestLoadFromPath(t *testing.T) {
	t.Run("valid file returns registry", func(t *testing.T) {
		content := []byte(`subscriptions:
  - github_username: octocat
    email: octocat@example.com
`)
		path := filepath.Join(t.TempDir(), "subscriptions.yaml")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}

		reg, err := LoadFromPath(path)
		if err != nil {
			t.Fatalf("LoadFromPath() returned unexpected error: %v", err)
		}
		email, ok := reg.EmailFor("octocat")
		if !ok || email != "octocat@example.com" {
			t.Errorf("got email=%q ok=%v, want octocat@example.com true", email, ok)
		}
	})

	t.Run("non-existent file returns error", func(t *testing.T) {
		_, err := LoadFromPath("/non/existent/subscriptions.yaml")
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})

	t.Run("malformed yaml returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subscriptions.yaml")
		if err := os.WriteFile(path, []byte("subscriptions:\n  - github_username: [unclosed\n"), 0o600); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}
		_, err := LoadFromPath(path)
		if err == nil {
			t.Fatal("expected error for malformed yaml, got nil")
		}
	})
}

func TestRegistryEmailFor(t *testing.T) {
	reg := Registry{
		"octocat": "octocat@example.com",
	}

	t.Run("known username returns email", func(t *testing.T) {
		email, ok := reg.EmailFor("octocat")
		if !ok {
			t.Fatal("expected ok=true for known username")
		}
		if email != "octocat@example.com" {
			t.Errorf("got %q, want %q", email, "octocat@example.com")
		}
	})

	t.Run("unknown username returns not found", func(t *testing.T) {
		_, ok := reg.EmailFor("unknown")
		if ok {
			t.Fatal("expected ok=false for unknown username")
		}
	})
}
