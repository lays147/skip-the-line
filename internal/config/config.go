package config

import "github.com/caarlos0/env/v11"

// Config holds all application configuration loaded from environment variables.
type Config struct {
	GitHubWebhookSecret      string `env:"GITHUB_WEBHOOK_SECRET,required"`
	GitHubToken              string `env:"GITHUB_TOKEN,required"`
	SlackBotToken            string `env:"SLACK_BOT_TOKEN,required"`
	Port                     string `env:"PORT" envDefault:"8080"`
	ReadTimeoutSeconds       int    `env:"READ_TIMEOUT_SECONDS" envDefault:"10"`
	WriteTimeoutSeconds      int    `env:"WRITE_TIMEOUT_SECONDS" envDefault:"10"`
	HandlerTimeoutSeconds    int    `env:"HANDLER_TIMEOUT_SECONDS" envDefault:"8"`
	LogLevel                 string `env:"LOG_LEVEL" envDefault:"info"`
	Environment              string `env:"ENVIRONMENT" envDefault:"dev"`
	OTELExporterOTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OTELServiceName          string `env:"OTEL_SERVICE_NAME" envDefault:"github-webhook-notifier"`
	DeliveryDedupTTLHours    int    `env:"DELIVERY_DEDUP_TTL_HOURS" envDefault:"4"`
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
