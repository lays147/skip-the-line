package config

import (
	"os"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_TOKEN", "ghtoken")
}

func TestLoad_MissingRequiredVariables(t *testing.T) {
	tests := []struct {
		name    string
		setEnv  func(t *testing.T)
		wantErr bool
	}{
		{
			name: "missing GITHUB_WEBHOOK_SECRET",
			setEnv: func(t *testing.T) {
				t.Setenv("GITHUB_TOKEN", "ghtoken")
			},
			wantErr: true,
		},
		{
			name: "missing GITHUB_TOKEN",
			setEnv: func(t *testing.T) {
				t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
			},
			wantErr: true,
		},
		{
			name:    "all required variables set (no SLACK_BOT_TOKEN needed at config level)",
			setEnv:  setRequiredEnv,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unset all required env vars so each sub-test starts clean.
			for _, key := range []string{"GITHUB_WEBHOOK_SECRET", "GITHUB_TOKEN"} {
				saved, exists := os.LookupEnv(key)
				os.Unsetenv(key)
				if exists {
					t.Cleanup(func() { os.Setenv(key, saved) })
				} else {
					t.Cleanup(func() { os.Unsetenv(key) })
				}
			}

			tt.setEnv(t)

			_, err := Load()
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoad_OptionalDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTP.Port != "8080" {
		t.Errorf("expected HTTP.Port=8080, got %q", cfg.HTTP.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel=info, got %q", cfg.LogLevel)
	}
	if cfg.Environment != "dev" {
		t.Errorf("expected Environment=dev, got %q", cfg.Environment)
	}
	if cfg.OTEL.ServiceName != "github-webhook-notifier" {
		t.Errorf("expected OTEL.ServiceName=github-webhook-notifier, got %q", cfg.OTEL.ServiceName)
	}
	if cfg.OTEL.ServiceVersion != "dev" {
		t.Errorf("expected OTEL.ServiceVersion=dev, got %q", cfg.OTEL.ServiceVersion)
	}
	if cfg.NotificationPlatform != "slack" {
		t.Errorf("expected NotificationPlatform=slack, got %q", cfg.NotificationPlatform)
	}
}

func TestLoad_OptionalOverrides(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("ENVIRONMENT", "staging")
	t.Setenv("OTEL_SERVICE_NAME", "my-service")
	t.Setenv("OTEL_SERVICE_VERSION", "1.2.3")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4317")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTP.Port != "9090" {
		t.Errorf("expected HTTP.Port=9090, got %q", cfg.HTTP.Port)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel=debug, got %q", cfg.LogLevel)
	}
	if cfg.Environment != "staging" {
		t.Errorf("expected Environment=staging, got %q", cfg.Environment)
	}
	if cfg.OTEL.ServiceName != "my-service" {
		t.Errorf("expected OTEL.ServiceName=my-service, got %q", cfg.OTEL.ServiceName)
	}
	if cfg.OTEL.ServiceVersion != "1.2.3" {
		t.Errorf("expected OTEL.ServiceVersion=1.2.3, got %q", cfg.OTEL.ServiceVersion)
	}
	if cfg.OTEL.ExporterEndpoint != "http://collector:4317" {
		t.Errorf("expected OTEL.ExporterEndpoint=http://collector:4317, got %q", cfg.OTEL.ExporterEndpoint)
	}
}

func TestLoad_GitHubAndSlackConfig(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "webhook-secret")
	t.Setenv("GITHUB_TOKEN", "gh-token")
	t.Setenv("GITHUB_API_URL", "http://github.mock:4000/")
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-slack")
	t.Setenv("SLACK_API_URL", "http://slack.mock:3000/api/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitHub.WebhookSecret != "webhook-secret" {
		t.Errorf("expected GitHub.WebhookSecret=webhook-secret, got %q", cfg.GitHub.WebhookSecret)
	}
	if cfg.GitHub.Token != "gh-token" {
		t.Errorf("expected GitHub.Token=gh-token, got %q", cfg.GitHub.Token)
	}
	if cfg.GitHub.APIURL != "http://github.mock:4000/" {
		t.Errorf("expected GitHub.APIURL=http://github.mock:4000/, got %q", cfg.GitHub.APIURL)
	}
	if cfg.Slack.BotToken != "xoxb-slack" {
		t.Errorf("expected Slack.BotToken=xoxb-slack, got %q", cfg.Slack.BotToken)
	}
	if cfg.Slack.APIURL != "http://slack.mock:3000/api/" {
		t.Errorf("expected Slack.APIURL=http://slack.mock:3000/api/, got %q", cfg.Slack.APIURL)
	}
}

func TestLoad_GoogleChatConfig(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("NOTIFICATION_PLATFORM", "google_chat")
	t.Setenv("GCHAT_CREDENTIALS_JSON", `{"type":"service_account"}`)
	t.Setenv("GCHAT_ADMIN_EMAIL", "admin@example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.NotificationPlatform != "google_chat" {
		t.Errorf("expected NotificationPlatform=google_chat, got %q", cfg.NotificationPlatform)
	}
	if cfg.GoogleChat.CredentialsJSON != `{"type":"service_account"}` {
		t.Errorf("unexpected GoogleChat.CredentialsJSON: %q", cfg.GoogleChat.CredentialsJSON)
	}
	if cfg.GoogleChat.AdminEmail != "admin@example.com" {
		t.Errorf("expected GoogleChat.AdminEmail=admin@example.com, got %q", cfg.GoogleChat.AdminEmail)
	}
}
