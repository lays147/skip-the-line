package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/google/go-github/v62/github"
	"github.com/skip-the-line/internal/metrics"
	"github.com/skip-the-line/internal/notification"
	"github.com/skip-the-line/internal/subscription"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// Handler handles incoming GitHub webhook requests.
type Handler struct {
	svc            notification.NotificationServicer
	webhookSecret  string
	counter        metric.Int64Counter
	mergeHistogram metric.Float64Histogram
	subs           subscription.Registry
	logger         *zap.Logger
}

// NewHandler constructs a new webhook Handler.
func NewHandler(svc notification.NotificationServicer, secret string, counter metric.Int64Counter, mergeHistogram metric.Float64Histogram, subs subscription.Registry, logger *zap.Logger) *Handler {
	return &Handler{
		svc:            svc,
		webhookSecret:  secret,
		counter:        counter,
		mergeHistogram: mergeHistogram,
		subs:           subs,
		logger:         logger,
	}
}

// ServeHTTP handles POST /webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Validate signature and read body.
	body, err := github.ValidatePayload(r, []byte(h.webhookSecret))
	if err != nil {
		h.logger.Warn("invalid webhook signature", zap.Error(err))
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")

	// Increment OTel counter after successful validation.
	h.counter.Add(r.Context(), 1, metric.WithAttributes(
		attribute.String("event_type", eventType),
	))

	// Parse the webhook payload into a typed SDK struct.
	event, err := github.ParseWebHook(eventType, body)
	if err != nil {
		h.logger.Debug("unsupported or unparseable event type",
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
		metrics.RecordPRMergeDuration(
			r.Context(),
			h.mergeHistogram,
			pr.GetPullRequest().GetCreatedAt().Time,
			pr.GetPullRequest().GetMergedAt().Time,
			subscribed,
		)
	}

	// Dispatch to notification service.
	if err := h.svc.Notify(r.Context(), eventType, event); err != nil {
		h.logger.Error("notification service error",
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
