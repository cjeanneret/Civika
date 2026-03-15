package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"civika/backend/config"
	"civika/backend/internal/rag"
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

type captureQAService struct {
	lastInput services.QAQueryInput
}

func (f *captureQAService) Query(_ context.Context, input services.QAQueryInput) (services.QAQueryOutput, error) {
	f.lastInput = input
	return services.QAQueryOutput{Answer: "ok", Language: input.Language}, nil
}

type fakeUsageMetrics struct{}

func (f fakeUsageMetrics) ListUsageEvents(_ context.Context, _ rag.UsageListFilter) ([]rag.UsageEventRow, error) {
	return []rag.UsageEventRow{
		{
			EventID:      "evt-1",
			CreatedAtUTC: time.Now().UTC().Format(time.RFC3339),
			Flow:         "qa_query",
			Operation:    "summarization",
			Mode:         "llm",
		},
	}, nil
}

func (f fakeUsageMetrics) ListUsageDailyAggregates(_ context.Context, _ rag.UsageListFilter) ([]rag.UsageDailyAggregate, error) {
	return []rag.UsageDailyAggregate{
		{
			Day:            "2026-03-14",
			Flow:           "qa_query",
			Operation:      "summarization",
			Mode:           "llm",
			TotalTokensSum: 100,
		},
	}, nil
}

type captureUsageMetrics struct {
	lastEventFilter *rag.UsageListFilter
	lastDayFilter   *rag.UsageListFilter
}

func (c *captureUsageMetrics) ListUsageEvents(_ context.Context, filter rag.UsageListFilter) ([]rag.UsageEventRow, error) {
	cloned := filter
	c.lastEventFilter = &cloned
	return []rag.UsageEventRow{
		{
			EventID:      "evt-capture-1",
			CreatedAtUTC: time.Now().UTC().Format(time.RFC3339),
			Flow:         "qa_query",
			Operation:    "embedding",
			Mode:         "llm",
		},
	}, nil
}

func (c *captureUsageMetrics) ListUsageDailyAggregates(_ context.Context, filter rag.UsageListFilter) ([]rag.UsageDailyAggregate, error) {
	cloned := filter
	c.lastDayFilter = &cloned
	return []rag.UsageDailyAggregate{
		{
			Day:            "2026-03-14",
			Flow:           "rag_index",
			Operation:      "translation",
			Mode:           "llm",
			TotalTokensSum: 10,
		},
	}, nil
}

type captureLangVotationService struct {
	lastLang string
}

type fakeQACacheMetrics struct{}

func (f fakeQACacheMetrics) MetricsSnapshot() services.QACacheMetricsSnapshot {
	return services.QACacheMetricsSnapshot{
		Enabled:                   true,
		SemanticEnabled:           true,
		ExactEntries:              12,
		SemanticEntries:           34,
		ExactHits:                 20,
		SemanticHits:              15,
		Misses:                    10,
		BypassSensitiveQuestion:   3,
		BypassSemanticDisabled:    0,
		BypassQuestionTooShort:    2,
		HitRate:                   0.7778,
		SemanticHitRate:           0.3333,
		SemanticScoreMeanOnHit:    0.9123,
		SavedInputTokensEstimate:  500,
		SavedOutputTokensEstimate: 300,
		SavedTotalTokensEstimate:  800,
	}
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
		UsageMetrics:    fakeUsageMetrics{},
		QACacheMetrics:  fakeQACacheMetrics{},
		APIVersion:      "v1",
		RAGMode:         "local",
	})
}

func buildRouterForMetricsTest(metrics rag.UsageMetricsReader) http.Handler {
	cfg := config.LoadFromEnv()
	return NewRouter(cfg, RouterDependencies{
		VotationService: fakeVotationService{},
		QAService:       fakeQAService{},
		UsageMetrics:    metrics,
		QACacheMetrics:  fakeQACacheMetrics{},
		APIVersion:      "v1",
		RAGMode:         "local",
	})
}

func buildRouterForQACacheMetricsTest(cacheMetrics services.QACacheMetricsReader) http.Handler {
	cfg := config.LoadFromEnv()
	return NewRouter(cfg, RouterDependencies{
		VotationService: fakeVotationService{},
		QAService:       fakeQAService{},
		UsageMetrics:    fakeUsageMetrics{},
		QACacheMetrics:  cacheMetrics,
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

func TestQAQueryRejectsUnsupportedBodyLanguage(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	body := []byte(`{"question":"Que change cette votation ?","language":"es"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported language, got %d", rec.Code)
	}
	var payload apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected API error JSON: %v", err)
	}
	if payload.Code != "invalid_body" {
		t.Fatalf("expected invalid_body code, got %q", payload.Code)
	}
}

func TestQAQueryResolvesLanguageFromAcceptLanguageHeader(t *testing.T) {
	qaSvc := &captureQAService{}
	router := buildRouterForTest(fakeVotationService{}, qaSvc)
	body := []byte(`{"question":"Que change cette votation ?","language":""}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	req.Header.Set("Accept-Language", "de-CH,de;q=0.8,en;q=0.6")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if qaSvc.lastInput.Language != "de" {
		t.Fatalf("expected resolved language de, got %q", qaSvc.lastInput.Language)
	}
}

func TestQAQueryRejectsUnsupportedAcceptLanguage(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	body := []byte(`{"question":"Que change cette votation ?","language":""}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	req.Header.Set("Accept-Language", "es-ES,pt;q=0.8")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported accept-language, got %d", rec.Code)
	}
}

func TestQAQueryBodyLanguageTakesPriorityOverHeader(t *testing.T) {
	qaSvc := &captureQAService{}
	router := buildRouterForTest(fakeVotationService{}, qaSvc)
	body := []byte(`{"question":"Que change cette votation ?","language":"it"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/qa/query", bytes.NewReader(body))
	req.Header.Set("Accept-Language", "de-CH,de;q=0.8")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if qaSvc.lastInput.Language != "it" {
		t.Fatalf("expected language it from body, got %q", qaSvc.lastInput.Language)
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

func TestMetricsUsageDay(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/ai-usage?granularity=day&limit=10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json, got error: %v", err)
	}
	if payload["granularity"] != "day" {
		t.Fatalf("expected granularity day, got %v", payload["granularity"])
	}
}

func TestMetricsUsageRejectsInvalidGranularity(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/ai-usage?granularity=month", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMetricsUsageDefaultsToDayGranularity(t *testing.T) {
	metrics := &captureUsageMetrics{}
	router := buildRouterForMetricsTest(metrics)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/ai-usage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if metrics.lastDayFilter == nil {
		t.Fatalf("expected daily aggregate reader to be called")
	}
	if metrics.lastEventFilter != nil {
		t.Fatalf("did not expect event reader to be called")
	}
}

func TestMetricsUsageEventPropagatesFilters(t *testing.T) {
	metrics := &captureUsageMetrics{}
	router := buildRouterForMetricsTest(metrics)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/metrics/ai-usage?granularity=event&from=2026-03-01T00:00:00Z&to=2026-03-10T00:00:00Z&flow=qa_query&operation=embedding&mode=llm&limit=9&offset=2",
		nil,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if metrics.lastEventFilter == nil {
		t.Fatalf("expected event reader to be called")
	}
	filter := metrics.lastEventFilter
	if filter.Flow != "qa_query" || filter.Operation != "embedding" || filter.Mode != "llm" {
		t.Fatalf("unexpected filter values: %#v", *filter)
	}
	if filter.Limit != 9 || filter.Offset != 2 {
		t.Fatalf("unexpected pagination values: %#v", *filter)
	}
	if filter.FromUTC == nil || filter.ToUTC == nil {
		t.Fatalf("expected from/to to be parsed")
	}
}

func TestMetricsUsageRejectsInvalidFilters(t *testing.T) {
	testCases := []struct {
		name string
		url  string
	}{
		{
			name: "invalid limit lower bound",
			url:  "/api/v1/metrics/ai-usage?granularity=event&limit=0",
		},
		{
			name: "invalid limit upper bound",
			url:  "/api/v1/metrics/ai-usage?granularity=event&limit=1001",
		},
		{
			name: "invalid offset",
			url:  "/api/v1/metrics/ai-usage?granularity=event&offset=-2",
		},
		{
			name: "invalid flow",
			url:  "/api/v1/metrics/ai-usage?granularity=event&flow=other",
		},
		{
			name: "invalid operation",
			url:  "/api/v1/metrics/ai-usage?granularity=event&operation=chat",
		},
		{
			name: "invalid mode",
			url:  "/api/v1/metrics/ai-usage?granularity=event&mode=hybrid",
		},
		{
			name: "invalid from",
			url:  "/api/v1/metrics/ai-usage?granularity=event&from=2026-03-10",
		},
		{
			name: "invalid to",
			url:  "/api/v1/metrics/ai-usage?granularity=event&to=invalid",
		},
		{
			name: "from after to",
			url:  "/api/v1/metrics/ai-usage?granularity=event&from=2026-03-11T00:00:00Z&to=2026-03-10T00:00:00Z",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
			var payload apiError
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("expected API error JSON: %v", err)
			}
			if payload.Code != "invalid_query" {
				t.Fatalf("expected invalid_query code, got %q", payload.Code)
			}
		})
	}
}

func TestMetricsUsageErrorMessageDoesNotEchoRawDateInput(t *testing.T) {
	router := buildRouterForTest(fakeVotationService{}, fakeQAService{})
	rawInput := "not-a-date-with-private-fragment-123"
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/metrics/ai-usage?granularity=event&from="+rawInput,
		nil,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var payload apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected API error JSON: %v", err)
	}
	if strings.Contains(payload.Message, rawInput) {
		t.Fatalf("error message should not echo raw input, got %q", payload.Message)
	}
}

func TestMetricsQACache(t *testing.T) {
	router := buildRouterForQACacheMetricsTest(fakeQACacheMetrics{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/qa-cache", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json, got error: %v", err)
	}
	if payload["type"] != "qa-cache" {
		t.Fatalf("expected type qa-cache, got %v", payload["type"])
	}
	items, ok := payload["items"].(map[string]any)
	if !ok {
		t.Fatalf("expected items object")
	}
	if items["savedTotalTokensEstimate"] != float64(800) {
		t.Fatalf("expected savedTotalTokensEstimate=800, got %v", items["savedTotalTokensEstimate"])
	}
}

func TestMetricsQACacheServiceUnavailableWhenDisabled(t *testing.T) {
	router := buildRouterForQACacheMetricsTest(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/qa-cache", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
