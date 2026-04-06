package subscription

import (
	_ "embed"
	"fmt"
	"os"

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

func parse(data []byte) (Registry, error) {
	var f subscriptionFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	reg := make(Registry, len(f.Subscriptions))
	for _, s := range f.Subscriptions {
		reg[s.GitHubUsername] = s.Email
	}
	return reg, nil
}

// Load parses the embedded subscriptions.yaml and returns a Registry.
func Load() (Registry, error) {
	return parse(subscriptionsData)
}

// LoadFromPath reads the YAML file at path and returns a Registry.
// Use this when running from a pre-built Docker image and supplying
// subscriptions via the SUBSCRIPTIONS_PATH environment variable.
func LoadFromPath(path string) (Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading subscriptions file %q: %v", path, err)
	}
	return parse(data)
}
