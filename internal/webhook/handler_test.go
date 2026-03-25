package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/skip-the-line/internal/mocks"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
)

const testSecret = "test-secret"

// signPayload computes the HMAC-SHA256 signature for a payload.
func signPayload(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// pullRequestPayload returns a minimal pull_request event JSON body.
func pullRequestPayload() []byte {
	payload := map[string]any{
		"action": "review_requested",
		"pull_request": map[string]any{
			"number": 1,
			"title":  "Test PR",
			"html_url": "https://github.com/org/repo/pull/1",
			"user": map[string]any{
				"login": "author",
			},
			"requested_reviewers": []any{
				map[string]any{"login": "reviewer1"},
			},
		},
		"repository": map[string]any{
			"full_name": "org/repo",
			"owner": map[string]any{
				"login": "org",
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func newTestHandler(svc *mocks.NotificationServicerMock) *Handler {
	mp := noop.NewMeterProvider()
	counter, _ := mp.Meter("test").Int64Counter("webhook_events_total")
	return NewHandler(svc, testSecret, counter, noopLogger())
}

func noopLogger() *zap.Logger {
	return zap.NewNop()
}

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		eventType      string
		body           []byte
		// sigOverride: if non-empty, use this as the X-Hub-Signature-256 header value.
		// If empty and validSig is false, no signature header is set.
		sigOverride    string
		validSig       bool
		notifyErr      error
		wantStatus     int
		wantDispatched bool
	}{
		{
			name:           "valid signature and supported event dispatches to service",
			eventType:      "pull_request",
			body:           pullRequestPayload(),
			validSig:       true,
			wantStatus:     http.StatusOK,
			wantDispatched: true,
		},
		{
			name:        "invalid signature returns 401",
			eventType:   "pull_request",
			body:        pullRequestPayload(),
			sigOverride: "sha256=invalidsignature",
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:       "missing signature returns 401",
			eventType:  "pull_request",
			body:       pullRequestPayload(),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:      "unsupported event type returns 200 no-op",
			eventType: "unknown_event_xyz",
			body:      []byte(`{}`),
			validSig:  true,
			wantStatus: http.StatusOK,
		},
		{
			name:           "service error returns 500",
			eventType:      "pull_request",
			body:           pullRequestPayload(),
			validSig:       true,
			notifyErr:      errors.New("service failure"),
			wantStatus:     http.StatusInternalServerError,
			wantDispatched: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dispatched := false
			mockSvc := &mocks.NotificationServicerMock{
				NotifyFunc: func(ctx context.Context, eventType string, event any) error {
					dispatched = true
					return tc.notifyErr
				},
			}

			h := newTestHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Event", tc.eventType)

			switch {
			case tc.sigOverride != "":
				req.Header.Set("X-Hub-Signature-256", tc.sigOverride)
			case tc.validSig:
				req.Header.Set("X-Hub-Signature-256", signPayload([]byte(testSecret), tc.body))
			}

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			if dispatched != tc.wantDispatched {
				t.Errorf("expected dispatched=%v, got %v", tc.wantDispatched, dispatched)
			}

			// Error responses must have JSON content type.
			if rr.Code >= 400 {
				ct := rr.Header().Get("Content-Type")
				if ct != "application/json" {
					t.Errorf("expected Content-Type application/json on error, got %q", ct)
				}
				var respBody map[string]string
				if err := json.NewDecoder(rr.Body).Decode(&respBody); err != nil {
					t.Errorf("error response body is not valid JSON: %v", err)
				}
				if _, ok := respBody["error"]; !ok {
					t.Error("error response missing 'error' field")
				}
			}
		})
	}
}

