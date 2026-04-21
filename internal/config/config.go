package config

import "github.com/caarlos0/env/v11"

// GitHubConfig holds GitHub API and webhook configuration.
type GitHubConfig struct {
	WebhookSecret string `env:"GITHUB_WEBHOOK_SECRET,required"`
	Token         string `env:"GITHUB_TOKEN,required"`
	APIURL        string `env:"GITHUB_API_URL"`
}

// SlackConfig holds Slack API configuration.
// BotToken is only required when NOTIFICATION_PLATFORM=slack (validated in main).
type SlackConfig struct {
	BotToken string `env:"SLACK_BOT_TOKEN"`
	APIURL   string `env:"SLACK_API_URL"`
}

// GoogleChatConfig holds Google Chat API configuration.
// All fields are only required when NOTIFICATION_PLATFORM=google_chat (validated in main).
type GoogleChatConfig struct {
	// CredentialsJSON is the Google service account key JSON.
	// Supply the raw JSON string (e.g. via a secret manager or env var).
	CredentialsJSON string `env:"GCHAT_CREDENTIALS_JSON"`
	// AdminEmail is a Google Workspace admin email address used for
	// domain-wide delegation when looking up users via the Admin SDK.
	AdminEmail string `env:"GCHAT_ADMIN_EMAIL"`
}

// HTTPConfig holds HTTP server tuning parameters.
type HTTPConfig struct {
	Port                  string `env:"PORT" envDefault:"8080"`
	ReadTimeoutSeconds    int    `env:"READ_TIMEOUT_SECONDS" envDefault:"10"`
	WriteTimeoutSeconds   int    `env:"WRITE_TIMEOUT_SECONDS" envDefault:"10"`
	HandlerTimeoutSeconds int    `env:"HANDLER_TIMEOUT_SECONDS" envDefault:"8"`
	DeliveryDedupTTLHours int    `env:"DELIVERY_DEDUP_TTL_HOURS" envDefault:"4"`
}

// OTELConfig holds OpenTelemetry configuration.
type OTELConfig struct {
	ServiceName      string `env:"OTEL_SERVICE_NAME" envDefault:"github-webhook-notifier"`
	ServiceVersion   string `env:"OTEL_SERVICE_VERSION" envDefault:"dev"`
	ExporterEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

// Config holds all application configuration loaded from environment variables.
type Config struct {
	GitHub     GitHubConfig
	Slack      SlackConfig
	GoogleChat GoogleChatConfig
	HTTP       HTTPConfig
	OTEL       OTELConfig
	LogLevel   string `env:"LOG_LEVEL" envDefault:"info"`
	Environment string `env:"ENVIRONMENT" envDefault:"dev"`
	// NotificationPlatform selects the chat platform used to deliver notifications.
	// Accepted values: "slack" (default), "google_chat".
	NotificationPlatform string `env:"NOTIFICATION_PLATFORM" envDefault:"slack"`
	// SubscriptionsPath is an optional path to an external subscriptions YAML
	// file. When set, the embedded subscriptions.yaml is ignored. Intended for
	// users who pull the published Docker image and want to supply their own
	// subscriber list.
	SubscriptionsPath string `env:"SUBSCRIPTIONS_PATH"`
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
