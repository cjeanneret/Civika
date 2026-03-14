package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"civika/backend/config"
	"civika/backend/internal/rag"
	"civika/backend/internal/security"
	"civika/backend/internal/services"
)

type RouterDependencies struct {
	VotationService services.VotationService
	QAService       services.QueryService
	UsageMetrics    rag.UsageMetricsReader
	APIVersion      string
	RAGMode         string
}

func NewRouter(cfg config.Config, deps RouterDependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(security.ApplyHeaders)
	r.Use(recoverMiddleware())
	r.Use(requestIDMiddleware())
	r.Use(accessLogMiddleware())
	r.Use(bodyLimitMiddleware(cfg.BodyMaxBytes))

	handlers := apiHandlers{
		votationService: deps.VotationService,
		qaService:       deps.QAService,
		usageMetrics:    deps.UsageMetrics,
		apiVersion:      deps.APIVersion,
		ragMode:         deps.RAGMode,
	}

	r.Get("/", handlers.rootHandler)
	r.Get("/health", healthHandler)
	r.Get("/info", handlers.infoHandler)
	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/votations", handlers.listVotationsHandler)
		api.Get("/votations/{id}", handlers.getVotationHandler)
		api.Get("/votations/{id}/objects", handlers.listVotationObjectsHandler)
		api.Get("/objects/{objectId}", handlers.getObjectHandler)
		api.Get("/objects/{objectId}/sources", handlers.listObjectSourcesHandler)
		api.Get("/taxonomies", handlers.taxonomiesHandler)
		api.With(qaRateLimitMiddleware(cfg)).Post("/qa/query", handlers.qaQueryHandler)
		api.Get("/metrics/ai-usage", handlers.metricsUsageHandler)
	})

	return r
}

func isSafeMetricEnum(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "event", "day", "rag_index", "qa_query", "embedding", "translation", "summarization", "local", "llm":
		return true
	default:
		return false
	}
}
