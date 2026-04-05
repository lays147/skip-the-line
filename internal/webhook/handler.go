package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/logger"
	"github.com/skip-the-line/internal/metrics"
	"github.com/skip-the-line/internal/notification"
	"github.com/skip-the-line/internal/subscription"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

// Handler handles incoming GitHub webhook requests.
type Handler struct {
	svc           notification.NotificationServicer
	webhookSecret string
	metrics       *metrics.Metrics
	subs          subscription.Registry
	logger        *zap.Logger
	dedup         *dedupCache
}

// NewHandler constructs a new webhook Handler.
func NewHandler(svc notification.NotificationServicer, secret string, m *metrics.Metrics, subs subscription.Registry, logger *zap.Logger, dedup *dedupCache) *Handler {
	return &Handler{
		svc:           svc,
		webhookSecret: secret,
		metrics:       m,
		subs:          subs,
		logger:        logger,
		dedup:         dedup,
	}
}

// ServeHTTP handles POST /webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	// Start a root span for the full webhook lifecycle.
	ctx, span := otel.Tracer("github.com/skip-the-line").Start(ctx, "webhook.ServeHTTP")
	defer span.End()
	r = r.WithContext(ctx)

	log := logger.FromContext(ctx, h.logger)

	// Suppress duplicate deliveries — GitHub retries on transient errors.
	if id := r.Header.Get("X-GitHub-Delivery"); id != "" && h.dedup.SeenOrRecord(id) {
		log.Debug("duplicate webhook delivery ignored", zap.String("delivery_id", id))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Validate signature and read body.
	body, err := github.ValidatePayload(r, []byte(h.webhookSecret))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid signature")
		log.Warn("invalid webhook signature", zap.Error(err))
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	span.SetAttributes(attribute.String("event_type", eventType))
	h.metrics.RecordWebhookEvent(r.Context(), eventType)

	// Parse the webhook payload into a typed SDK struct.
	event, err := github.ParseWebHook(eventType, body)
	if err != nil {
		log.Debug("unsupported or unparseable event type",
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		// Return 200 no-op for unsupported event types.
		w.WriteHeader(http.StatusOK)
		return
	}

	// Record PR merge duration when a pull_request is closed and merged.
	if pr, ok := event.(*github.PullRequestEvent); ok &&
		pr.GetAction() == "closed" && pr.GetPullRequest().GetMerged() {
		authorLogin := pr.GetPullRequest().GetUser().GetLogin()
		_, subscribed := h.subs.EmailFor(authorLogin)
		h.metrics.RecordPRMergeDuration(
			r.Context(),
			pr.GetPullRequest().GetCreatedAt().Time,
			pr.GetPullRequest().GetMergedAt().Time,
			subscribed,
		)
	}

	// Dispatch to notification service.
	if err := h.svc.Notify(r.Context(), eventType, event); err != nil {
		log.Error("notification service error",
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
