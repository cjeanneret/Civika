package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQARateLimiterAllowsAfterRefill(t *testing.T) {
	limiter := newQARateLimiter(20, 1, time.Minute)
	clientKey := "203.0.113.50"
	now := time.Now()

	if !limiter.allow(clientKey, now) {
		t.Fatalf("expected first request to be allowed")
	}
	if limiter.allow(clientKey, now) {
		t.Fatalf("expected second immediate request to be blocked")
	}
	if !limiter.allow(clientKey, now.Add(60*time.Millisecond)) {
		t.Fatalf("expected request after refill delay to be allowed")
	}
}

func TestQAClientKeyUsesHostFromRemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "198.51.100.8:4567"

	key := qaClientKey(req)
	if key != "198.51.100.8" {
		t.Fatalf("expected host-only key, got %q", key)
	}
}

func TestRequestIDMiddlewareKeepsValidClientRequestID(t *testing.T) {
	middleware := requestIDMiddleware()
	var capturedRequestID string

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID, _ = r.Context().Value(requestIDKey).(string)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-Id", "client.req-123:abc")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != "client.req-123:abc" {
		t.Fatalf("expected response request id to be preserved, got %q", got)
	}
	if capturedRequestID != "client.req-123:abc" {
		t.Fatalf("expected context request id to be preserved, got %q", capturedRequestID)
	}
}

func TestRequestIDMiddlewareReplacesInvalidClientRequestID(t *testing.T) {
	middleware := requestIDMiddleware()
	var capturedRequestID string

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID, _ = r.Context().Value(requestIDKey).(string)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-Id", "invalid request id")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("X-Request-Id")
	if got == "invalid request id" {
		t.Fatalf("expected invalid request id to be replaced")
	}
	if !isValidRequestID(got) {
		t.Fatalf("expected generated request id to be valid, got %q", got)
	}
	if capturedRequestID != got {
		t.Fatalf("expected context request id %q, got %q", got, capturedRequestID)
	}
}

func TestRequestIDMiddlewareGeneratesWhenMissing(t *testing.T) {
	middleware := requestIDMiddleware()
	var capturedRequestID string

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID, _ = r.Context().Value(requestIDKey).(string)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("X-Request-Id")
	if got == "" {
		t.Fatalf("expected generated request id in response header")
	}
	if !isValidRequestID(got) {
		t.Fatalf("expected generated request id to be valid, got %q", got)
	}
	if capturedRequestID != got {
		t.Fatalf("expected context request id %q, got %q", got, capturedRequestID)
	}
}
