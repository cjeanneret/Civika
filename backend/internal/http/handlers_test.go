package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"civika/backend/config"
	"civika/backend/internal/services"
)

type fakeVotationService struct {
	notFound bool
}

func (f fakeVotationService) ListVotations(_ context.Context, _ services.VotationFilters) (services.VotationListResult, error) {
	return services.VotationListResult{
		Items:  []services.VotationListItem{},
		Limit:  20,
		Offset: 0,
		Total:  0,
	}, nil
}

func (f fakeVotationService) GetVotationByID(_ context.Context, _ string, _ string) (services.VotationDetail, error) {
	if f.notFound {
		return services.VotationDetail{}, services.ErrNotFound
	}
	return services.VotationDetail{ID: "v1"}, nil
}

func (f fakeVotationService) ListObjectsByVotation(_ context.Context, _ string, _ string) ([]services.ObjectSummary, error) {
	return []services.ObjectSummary{}, nil
}

func (f fakeVotationService) GetObjectByID(_ context.Context, _ string, _ string) (services.ObjectDetail, error) {
	return services.ObjectDetail{ID: "o1", Title: "Objet"}, nil
}

func (f fakeVotationService) ListObjectSources(_ context.Context, _ string) ([]services.ObjectSource, error) {
	return []services.ObjectSource{}, nil
}

func (f fakeVotationService) GetTaxonomies(_ context.Context) (services.Taxonomies, error) {
	return services.Taxonomies{Levels: []string{"cantonal"}}, nil
}

type fakeQAService struct{}

func (f fakeQAService) Query(_ context.Context, _ services.QAQueryInput) (services.QAQueryOutput, error) {
	return services.QAQueryOutput{Answer: "ok", Language: "fr"}, nil
}

type captureLangVotationService struct {
	lastLang string
}

func (s *captureLangVotationService) ListVotations(_ context.Context, _ services.VotationFilters) (services.VotationListResult, error) {
	return services.VotationListResult{Items: []services.VotationListItem{}, Limit: 20, Offset: 0, Total: 0}, nil
}

func (s *captureLangVotationService) GetVotationByID(_ context.Context, _ string, lang string) (services.VotationDetail, error) {
	s.lastLang = lang
	return services.VotationDetail{ID: "v1"}, nil
}

func (s *captureLangVotationService) ListObjectsByVotation(_ context.Context, _ string, _ string) ([]services.ObjectSummary, error) {
	return []services.ObjectSummary{}, nil
}

func (s *captureLangVotationService) GetObjectByID(_ context.Context, _ string, _ string) (services.ObjectDetail, error) {
	return services.ObjectDetail{ID: "o1", Title: "Objet"}, nil
}

func (s *captureLangVotationService) ListObjectSources(_ context.Context, _ string) ([]services.ObjectSource, error) {
	return []services.ObjectSource{}, nil
}

func (s *captureLangVotationService) GetTaxonomies(_ context.Context) (services.Taxonomies, error) {
	return services.Taxonomies{}, nil
}

func buildRouterForTest(votationSvc services.VotationService, qaSvc services.QueryService) http.Handler {
	cfg := config.LoadFromEnv()
	return NewRouter(cfg, RouterDependencies{
		VotationService: votationSvc,
		QAService:       qaSvc,
		APIVersion:      "v1",
		RAGMode:         "local",
	})
}

func TestListVotationsInvalidLimit(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/votations?limit=500", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetVotationNotFound(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{notFound: true}, fakeQAService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/votations/unknown-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestQAQueryUnknownField(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	payload := map[string]any{
		"question": "Que change cette votation ?",
		"language": "fr",
		"unknown":  "x",
	}
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestQAQueryRateLimitExceeded(t *testing.T) {
	t.Setenv("API_QA_RATE_LIMIT_QPS", "1")
	t.Setenv("API_QA_RATE_LIMIT_BURST", "1")
	t.Setenv("API_QA_RATE_LIMIT_CLEANUP_INTERVAL", "1m")

	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	body := []byte(`{"question":"Que change cette votation ?","language":"fr"}`)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	firstReq.RemoteAddr = "203.0.113.10:1234"
	firstRec := httptest.NewRecorder()
	router.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first request status 200, got %d", firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	secondReq.RemoteAddr = "203.0.113.10:1234"
	secondRec := httptest.NewRecorder()
	router.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d", secondRec.Code)
	}
}

func TestQAQueryRejectsTrailingJSON(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	body := []byte(`{"question":"Que change cette votation ?","language":"fr"}{"extra":true}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for trailing JSON, got %d", rec.Code)
	}
}

func TestQAQueryAcceptsStrictJSON(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	body := []byte(`{"question":"Que change cette votation ?","language":"fr"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for strict JSON payload, got %d", rec.Code)
	}
}

func TestTaxonomiesRoute(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/taxonomies", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetVotationPassesLangFromQuery(t *testing.T) {
	votationSvc := &captureLangVotationService{}
	router := buildRouterForTest(votationSvc, fakeQAService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/votations/v1?lang=it", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if votationSvc.lastLang != "it" {
		t.Fatalf("expected language it to be passed through, got %q", votationSvc.lastLang)
	}
}
