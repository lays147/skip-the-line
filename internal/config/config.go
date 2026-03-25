package config

import "github.com/caarlos0/env/v11"

// Config holds all application configuration loaded from environment variables.
type Config struct {
	GitHubWebhookSecret      string `env:"GITHUB_WEBHOOK_SECRET,required"`
	GitHubToken              string `env:"GITHUB_TOKEN,required"`
	SlackBotToken            string `env:"SLACK_BOT_TOKEN,required"`
	Port                     string `env:"PORT"                          envDefault:"8080"`
	LogEnv                   string `env:"LOG_ENV"                       envDefault:"prod"`
	OTELExporterOTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OTELServiceName          string `env:"OTEL_SERVICE_NAME"             envDefault:"github-webhook-notifier"`
	// Optional overrides for local development / testing against mock servers.
	SlackAPIURL  string `env:"SLACK_API_URL"`
	GitHubAPIURL string `env:"GITHUB_API_URL"`
}

// Load parses environment variables into a Config struct.
// Returns an error if any required variable is missing.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
