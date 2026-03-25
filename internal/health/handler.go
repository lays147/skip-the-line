package health

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// Handler handles health and readiness check endpoints.
type Handler struct {
	ready atomic.Bool
}

// NewHandler creates a new Handler.
func NewHandler() *Handler {
	return &Handler{}
}

// SetReady sets the ready flag. Call this after all dependencies are initialised.
func (h *Handler) SetReady(ready bool) {
	h.ready.Store(ready)
}

// Healthz handles GET /healthz — always returns 200 {"status":"ok"}.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Readyz handles GET /readyz — returns 200 when ready, 503 otherwise.
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.ready.Load() {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
}
