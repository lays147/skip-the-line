package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/skip-the-line/internal/config"
	"github.com/skip-the-line/internal/health"
	"github.com/skip-the-line/internal/metrics"
	"github.com/skip-the-line/internal/notification"
	"github.com/skip-the-line/internal/subscription"
	"github.com/skip-the-line/internal/webhook"

	githubclient "github.com/skip-the-line/internal/github"
	googlechatclient "github.com/skip-the-line/internal/googlechat"
	slackclient "github.com/skip-the-line/internal/slack"
)

func main() {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Initialise logger (stdout JSON).
	logger, err := newLogger(cfg)
	if err != nil {
		panic("failed to initialise logger: " + err.Error())
	}
	defer logger.Sync() //nolint:errcheck

	// Load subscriptions into in-memory registry.
	// If SUBSCRIPTIONS_PATH is set, load from that external file (useful when
	// running the published Docker image). Otherwise fall back to the embedded
	// subscriptions.yaml baked in at compile time.
	var subs subscription.Registry
	if cfg.SubscriptionsPath != "" {
		subs, err = subscription.LoadFromPath(cfg.SubscriptionsPath)
	} else {
		subs, err = subscription.Load()
	}
	if err != nil {
		logger.Fatal("failed to load subscriptions", zap.Error(err))
	}

	// Initialise OTel trace provider (sets global TracerProvider + propagator).
	tp, err := newTracerProvider(cfg)
	if err != nil {
		logger.Fatal("failed to initialise trace provider", zap.Error(err))
	}

	// Initialise OTel meter provider.
	mp, err := metrics.NewMeterProvider(cfg)
	if err != nil {
		logger.Fatal("failed to initialise meter provider", zap.Error(err))
	}

	// Register all metric instruments.
	m, err := metrics.New(mp)
	if err != nil {
		logger.Fatal("failed to register metrics", zap.Error(err))
	}

	// Construct the platform-specific notifier based on NOTIFICATION_PLATFORM.
	var n notification.Notifier
	switch cfg.NotificationPlatform {
	case "google_chat":
		if cfg.GoogleChat.CredentialsJSON == "" {
			logger.Fatal("GCHAT_CREDENTIALS_JSON is required when NOTIFICATION_PLATFORM=google_chat")
		}
		if cfg.GoogleChat.AdminEmail == "" {
			logger.Fatal("GCHAT_ADMIN_EMAIL is required when NOTIFICATION_PLATFORM=google_chat")
		}
		gcClient, gcErr := googlechatclient.NewClient([]byte(cfg.GoogleChat.CredentialsJSON), cfg.GoogleChat.AdminEmail)
		if gcErr != nil {
			logger.Fatal("failed to initialise Google Chat client", zap.Error(gcErr))
		}
		n = gcClient
		logger.Info("notification platform: google_chat")
	default: // "slack"
		if cfg.Slack.BotToken == "" {
			logger.Fatal("SLACK_BOT_TOKEN is required when NOTIFICATION_PLATFORM=slack")
		}
		n = slackclient.NewClient(cfg.Slack.BotToken, cfg.Slack.APIURL)
		logger.Info("notification platform: slack")
	}

	// Construct clients and services.
	ghClient := githubclient.NewClient(cfg.GitHub.Token, cfg.GitHub.APIURL)
	notifSvc := notification.NewNotificationService(ghClient, n, subs, logger, m)

	// Construct handlers.
	dedupTTL := time.Duration(cfg.HTTP.DeliveryDedupTTLHours) * time.Hour
	webhookHandler := webhook.NewHandler(notifSvc, cfg.GitHub.WebhookSecret, m, subs, logger, webhook.NewDedupCache(dedupTTL))
	healthHandler := health.NewHandler()

	// Register routes.
	// Webhook processing is wrapped with a hard deadline so that slow API calls
	// cannot stall a goroutine indefinitely. The handler timeout is kept below
	// WriteTimeout so the server can still write the 503 response.
	handlerTimeout := time.Duration(cfg.HTTP.HandlerTimeoutSeconds) * time.Second
	r := chi.NewRouter()
	r.Post("/webhook", http.TimeoutHandler(webhookHandler, handlerTimeout, `{"error":"request timeout"}`).ServeHTTP)
	r.Get("/healthz", healthHandler.Healthz)
	r.Get("/readyz", healthHandler.Readyz)

	// Mark service as ready.
	healthHandler.SetReady(true)

	server := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.HTTP.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTP.WriteTimeoutSeconds) * time.Second,
	}

	// Start server in background.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		logger.Info("server starting", zap.String("port", cfg.HTTP.Port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal.
	<-quit
	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Warn("shutdown timeout exceeded, forcing exit", zap.Error(err))
	}

	if err := tp.Shutdown(ctx); err != nil {
		logger.Warn("trace provider shutdown error", zap.Error(err))
	}

	if err := mp.Shutdown(ctx); err != nil {
		logger.Warn("meter provider shutdown error", zap.Error(err))
	}

}
