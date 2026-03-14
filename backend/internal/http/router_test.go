package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"civika/backend/config"
)

func TestHealthRoute(t *testing.T) {
	cfg := config.LoadFromEnv()
	handler := NewRouter(cfg, RouterDependencies{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRootRouteExposesAPIMap(t *testing.T) {
	cfg := config.LoadFromEnv()
	handler := NewRouter(cfg, RouterDependencies{
		APIVersion: "v1",
		RAGMode:    "local",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	if payload["service"] != "civika-api" {
		t.Fatalf("expected service civika-api, got %v", payload["service"])
	}
}

func TestQARateLimitDoesNotAffectHealthRoute(t *testing.T) {
	t.Setenv("API_QA_RATE_LIMIT_QPS", "1")
	t.Setenv("API_QA_RATE_LIMIT_BURST", "1")
	t.Setenv("API_QA_RATE_LIMIT_CLEANUP_INTERVAL", "1m")

	cfg := config.LoadFromEnv()
	handler := NewRouter(cfg, RouterDependencies{
		QAService: fakeQAService{},
	})

	body := []byte(`{"question":"Question auto","language":"fr"}`)
	firstQA := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	firstQA.RemoteAddr = "203.0.113.20:1234"
	firstQARec := httptest.NewRecorder()
	handler.ServeHTTP(firstQARec, firstQA)

	secondQA := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	secondQA.RemoteAddr = "203.0.113.20:1234"
	secondQARec := httptest.NewRecorder()
	handler.ServeHTTP(secondQARec, secondQA)
	if secondQARec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on second QA request, got %d", secondQARec.Code)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected health route status 200, got %d", healthRec.Code)
	}
}
