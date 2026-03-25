package subscription

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed subscriptions.yaml
var subscriptionsData []byte

// Registry is an in-memory map of GitHub username → email for O(1) lookup.
type Registry map[string]string

// EmailFor returns the email address for the given GitHub username.
func (r Registry) EmailFor(githubUsername string) (string, bool) {
	email, ok := r[githubUsername]
	return email, ok
}

type entry struct {
	GitHubUsername string `yaml:"github_username"`
	Email          string `yaml:"email"`
}

type subscriptionFile struct {
	Subscriptions []entry `yaml:"subscriptions"`
}

// Load parses the embedded subscriptions.yaml and returns a Registry.
func Load() (Registry, error) {
	var f subscriptionFile
	if err := yaml.Unmarshal(subscriptionsData, &f); err != nil {
		return nil, err
	}
	reg := make(Registry, len(f.Subscriptions))
	for _, s := range f.Subscriptions {
		reg[s.GitHubUsername] = s.Email
	}
	return reg, nil
}
