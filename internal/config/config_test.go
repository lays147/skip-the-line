package config

import (
	"os"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_TOKEN", "ghtoken")
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-token")
}

func TestLoad_MissingRequiredVariables(t *testing.T) {
	tests := []struct {
		name    string
		setEnv  func(t *testing.T)
		wantErr bool
	}{
		{
			name:    "missing GITHUB_WEBHOOK_SECRET",
			setEnv:  func(t *testing.T) {
				t.Setenv("GITHUB_TOKEN", "ghtoken")
				t.Setenv("SLACK_BOT_TOKEN", "xoxb-token")
			},
			wantErr: true,
		},
		{
			name:    "missing GITHUB_TOKEN",
			setEnv:  func(t *testing.T) {
				t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
				t.Setenv("SLACK_BOT_TOKEN", "xoxb-token")
			},
			wantErr: true,
		},
		{
			name:    "missing SLACK_BOT_TOKEN",
			setEnv:  func(t *testing.T) {
				t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
				t.Setenv("GITHUB_TOKEN", "ghtoken")
			},
			wantErr: true,
		},
		{
			name:    "all required variables set",
			setEnv:  setRequiredEnv,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unset all required env vars so each sub-test starts clean.
			// t.Setenv is used for vars we want to restore; os.Unsetenv for
			// vars that must be absent to trigger the "required" error.
			for _, key := range []string{"GITHUB_WEBHOOK_SECRET", "GITHUB_TOKEN", "SLACK_BOT_TOKEN"} {
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

	if cfg.Port != "8080" {
		t.Errorf("expected Port=8080, got %q", cfg.Port)
	}
	if cfg.LogEnv != "prod" {
		t.Errorf("expected LogEnv=prod, got %q", cfg.LogEnv)
	}
	if cfg.OTELServiceName != "github-webhook-notifier" {
		t.Errorf("expected OTELServiceName=github-webhook-notifier, got %q", cfg.OTELServiceName)
	}
}

func TestLoad_OptionalOverrides(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_ENV", "dev")
	t.Setenv("OTEL_SERVICE_NAME", "my-service")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4317")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected Port=9090, got %q", cfg.Port)
	}
	if cfg.LogEnv != "dev" {
		t.Errorf("expected LogEnv=dev, got %q", cfg.LogEnv)
	}
	if cfg.OTELServiceName != "my-service" {
		t.Errorf("expected OTELServiceName=my-service, got %q", cfg.OTELServiceName)
	}
	if cfg.OTELExporterOTLPEndpoint != "http://collector:4317" {
		t.Errorf("expected OTELExporterOTLPEndpoint=http://collector:4317, got %q", cfg.OTELExporterOTLPEndpoint)
	}
}
