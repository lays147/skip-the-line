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
	slackclient "github.com/skip-the-line/internal/slack"
)

func main() {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		// zap not yet initialised; use stdlib log for fatal.
		panic("failed to load config: " + err.Error())
	}

	// Initialise logger.
	var logger *zap.Logger
	if cfg.LogEnv == "dev" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic("failed to initialise logger: " + err.Error())
	}
	defer logger.Sync() //nolint:errcheck

	// Load subscriptions into in-memory registry.
	subs, err := subscription.Load()
	if err != nil {
		logger.Fatal("failed to load subscriptions", zap.Error(err))
	}

	// Initialise OTel meter provider.
	mp, err := metrics.NewMeterProvider(cfg)
	if err != nil {
		logger.Fatal("failed to initialise meter provider", zap.Error(err))
	}

	// Create webhook_events_total counter.
	counter, err := metrics.WebhookEventsCounter(mp)
	if err != nil {
		logger.Fatal("failed to create webhook_events_total counter", zap.Error(err))
	}

	// Construct clients and services.
	ghClient := githubclient.NewClient(cfg.GitHubToken)
	slClient := slackclient.NewClient(cfg.SlackBotToken)
	notifSvc := notification.NewNotificationService(ghClient, slClient, subs)

	// Construct handlers.
	webhookHandler := webhook.NewHandler(notifSvc, cfg.GitHubWebhookSecret, counter, logger)
	healthHandler := health.NewHandler()

	// Register routes.
	r := chi.NewRouter()
	r.Post("/webhook", webhookHandler.ServeHTTP)
	r.Get("/healthz", healthHandler.Healthz)
	r.Get("/readyz", healthHandler.Readyz)

	// Mark service as ready.
	healthHandler.SetReady(true)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Start server in background.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		logger.Info("server starting", zap.String("port", cfg.Port))
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
}
