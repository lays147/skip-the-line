package subscription

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed subscriptions.yaml
var subscriptionsData []byte

// Subscription maps a GitHub username to an email address for Slack lookup.
type Subscription struct {
	GitHubUsername string `yaml:"github_username"`
	Email          string `yaml:"email"`
}

type subscriptionFile struct {
	Subscriptions []Subscription `yaml:"subscriptions"`
}

// Load parses the embedded subscriptions.yaml and returns the list of subscriptions.
func Load() ([]Subscription, error) {
	var f subscriptionFile
	if err := yaml.Unmarshal(subscriptionsData, &f); err != nil {
		return nil, err
	}
	return f.Subscriptions, nil
}
