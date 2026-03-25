package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz_AlwaysOK(t *testing.T) {
	h := NewHandler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	assertJSON(t, rr, "ok")
}

func TestReadyz_NotReadyBefore_SetReady(t *testing.T) {
	h := NewHandler()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	h.Readyz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	assertJSON(t, rr, "not ready")
}

func TestReadyz_OKAfter_SetReady(t *testing.T) {
	h := NewHandler()
	h.SetReady(true)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	h.Readyz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	assertJSON(t, rr, "ready")
}

func TestReadyz_BackToNotReady(t *testing.T) {
	h := NewHandler()
	h.SetReady(true)
	h.SetReady(false)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	h.Readyz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	assertJSON(t, rr, "not ready")
}

func TestHealthz_ContentType(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestReadyz_ContentType(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	h.Readyz(rr, req)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

// assertJSON decodes the response body and checks the "status" field.
func assertJSON(t *testing.T, rr *httptest.ResponseRecorder, wantStatus string) {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body["status"] != wantStatus {
		t.Fatalf("expected status %q, got %q", wantStatus, body["status"])
	}
}
