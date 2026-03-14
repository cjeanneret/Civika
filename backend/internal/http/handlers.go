package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"civika/backend/internal/debuglog"
	"civika/backend/internal/services"
)

type healthResponse struct {
	Status string `json:"status"`
}

type infoResponse struct {
	Version  string   `json:"version"`
	Mode     string   `json:"ragMode"`
	Features []string `json:"features"`
}

type rootResponse struct {
	Service    string         `json:"service"`
	APIVersion string         `json:"apiVersion"`
	RAGMode    string         `json:"ragMode"`
	Endpoints  map[string]any `json:"endpoints"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

type apiHandlers struct {
	votationService services.VotationService
	qaService       services.QueryService
	apiVersion      string
	ragMode         string
}

func (h apiHandlers) rootHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, rootResponse{
		Service:    "civika-api",
		APIVersion: h.apiVersion,
		RAGMode:    h.ragMode,
		Endpoints: map[string]any{
			"health": "/health",
			"info":   "/info",
			"apiV1": map[string]any{
				"basePath": "/api/v1",
				"routes": []string{
					"GET /api/v1/votations",
					"GET /api/v1/votations/{id}",
					"GET /api/v1/votations/{id}/objects",
					"GET /api/v1/objects/{objectId}",
					"GET /api/v1/objects/{objectId}/sources",
					"GET /api/v1/taxonomies",
					"POST /api/v1/qa/query",
				},
			},
		},
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func (h apiHandlers) infoHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, infoResponse{
		Version: h.apiVersion,
		Mode:    h.ragMode,
		Features: []string{
			"votations",
			"objects",
			"taxonomies",
			"qa",
		},
	})
}

func (h apiHandlers) listVotationsHandler(w http.ResponseWriter, r *http.Request) {
	if h.votationService == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "service_unavailable", "service indisponible")
		return
	}
	filters, err := parseVotationFilters(r)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	result, err := h.votationService.ListVotations(r.Context(), filters)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h apiHandlers) getVotationHandler(w http.ResponseWriter, r *http.Request) {
	if h.votationService == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "service_unavailable", "service indisponible")
		return
	}
	votationID := strings.TrimSpace(chi.URLParam(r, "id"))
	if !isSafeID(votationID) {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_path_param", "id invalide")
		return
	}
	lang := normalizeLanguage(r.URL.Query().Get("lang"))
	item, err := h.votationService.GetVotationByID(r.Context(), votationID, lang)
	if errors.Is(err, services.ErrNotFound) {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "votation introuvable")
		return
	}
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h apiHandlers) listVotationObjectsHandler(w http.ResponseWriter, r *http.Request) {
	if h.votationService == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "service_unavailable", "service indisponible")
		return
	}
	votationID := strings.TrimSpace(chi.URLParam(r, "id"))
	if !isSafeID(votationID) {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_path_param", "id invalide")
		return
	}
	lang := normalizeLanguage(r.URL.Query().Get("lang"))
	items, err := h.votationService.ListObjectsByVotation(r.Context(), votationID, lang)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h apiHandlers) getObjectHandler(w http.ResponseWriter, r *http.Request) {
	if h.votationService == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "service_unavailable", "service indisponible")
		return
	}
	objectID := strings.TrimSpace(chi.URLParam(r, "objectId"))
	if !isSafeID(objectID) {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_path_param", "objectId invalide")
		return
	}
	lang := normalizeLanguage(r.URL.Query().Get("lang"))
	item, err := h.votationService.GetObjectByID(r.Context(), objectID, lang)
	if errors.Is(err, services.ErrNotFound) {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "objet introuvable")
		return
	}
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h apiHandlers) listObjectSourcesHandler(w http.ResponseWriter, r *http.Request) {
	if h.votationService == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "service_unavailable", "service indisponible")
		return
	}
	objectID := strings.TrimSpace(chi.URLParam(r, "objectId"))
	if !isSafeID(objectID) {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_path_param", "objectId invalide")
		return
	}
	items, err := h.votationService.ListObjectSources(r.Context(), objectID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h apiHandlers) taxonomiesHandler(w http.ResponseWriter, r *http.Request) {
	if h.votationService == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "service_unavailable", "service indisponible")
		return
	}
	taxonomies, err := h.votationService.GetTaxonomies(r.Context())
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, taxonomies)
}

func (h apiHandlers) qaQueryHandler(w http.ResponseWriter, r *http.Request) {
	if h.qaService == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "service_unavailable", "service indisponible")
		return
	}
	requestID, _ := r.Context().Value(requestIDKey).(string)
	queryCtx := debuglog.WithRunID(r.Context(), requestID)
	// #region agent log
	debuglog.Log(queryCtx, "H1", "backend/internal/http/handlers.go:qaQueryHandler", "qa handler entered", map[string]any{
		"method":      r.Method,
		"path":        r.URL.Path,
		"contentType": r.Header.Get("Content-Type"),
	})
	// #endregion
	requestBody, err := decodeQARequest(r)
	if err != nil {
		// #region agent log
		debuglog.Log(queryCtx, "H1", "backend/internal/http/handlers.go:qaQueryHandler", "qa request decode failed", map[string]any{
			"error": err.Error(),
		})
		// #endregion
		writeAPIError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	start := time.Now()
	// #region agent log
	debuglog.Log(queryCtx, "H5", "backend/internal/http/handlers.go:qaQueryHandler", "qa service query start", map[string]any{
		"questionChars": len(requestBody.Question),
		"language":      requestBody.Language,
	})
	// #endregion
	output, err := h.qaService.Query(queryCtx, requestBody)
	// #region agent log
	debuglog.Log(queryCtx, "H5", "backend/internal/http/handlers.go:qaQueryHandler", "qa service query end", map[string]any{
		"durationMs":    time.Since(start).Milliseconds(),
		"ctxErr":        fmt.Sprint(queryCtx.Err()),
		"error":         fmt.Sprint(err),
		"answerChars":   len(output.Answer),
		"citationsSize": len(output.Citations),
	})
	// #endregion
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, output)
}

func parseVotationFilters(r *http.Request) (services.VotationFilters, error) {
	q := r.URL.Query()
	limit := 20
	offset := 0
	var err error
	if raw := q.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return services.VotationFilters{}, errors.New("limit doit etre entre 1 et 100")
		}
	}
	if raw := q.Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return services.VotationFilters{}, errors.New("offset doit etre >= 0")
		}
	}

	filters := services.VotationFilters{
		Level:  strings.ToLower(strings.TrimSpace(q.Get("level"))),
		Canton: strings.ToUpper(strings.TrimSpace(q.Get("canton"))),
		Status: strings.ToLower(strings.TrimSpace(q.Get("status"))),
		Lang:   normalizeLanguage(q.Get("lang")),
		Limit:  limit,
		Offset: offset,
	}
	if filters.Level != "" && !isAllowedValue(filters.Level, []string{"federal", "cantonal", "communal"}) {
		return services.VotationFilters{}, errors.New("level invalide")
	}
	if filters.Status != "" && !isAllowedValue(filters.Status, []string{"past", "upcoming"}) {
		return services.VotationFilters{}, errors.New("status invalide")
	}
	if filters.Canton != "" && !regexp.MustCompile(`^[A-Z]{2,3}$`).MatchString(filters.Canton) {
		return services.VotationFilters{}, errors.New("canton invalide")
	}
	if raw := q.Get("date_from"); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return services.VotationFilters{}, errors.New("date_from doit etre au format RFC3339")
		}
		filters.DateFrom = &value
	}
	if raw := q.Get("date_to"); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return services.VotationFilters{}, errors.New("date_to doit etre au format RFC3339")
		}
		filters.DateTo = &value
	}
	return filters, nil
}

func decodeQARequest(r *http.Request) (services.QAQueryInput, error) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var payload services.QAQueryInput
	if err := decoder.Decode(&payload); err != nil {
		return services.QAQueryInput{}, fmt.Errorf("body invalide")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return services.QAQueryInput{}, fmt.Errorf("body invalide")
	}
	if strings.TrimSpace(payload.Question) == "" {
		return services.QAQueryInput{}, errors.New("question requise")
	}
	if len(payload.Question) > 2000 {
		return services.QAQueryInput{}, errors.New("question trop longue")
	}
	payload.Language = normalizeLanguage(payload.Language)
	if payload.Context.VotationID != "" && !isSafeID(payload.Context.VotationID) {
		return services.QAQueryInput{}, errors.New("context.votationId invalide")
	}
	if payload.Context.ObjectID != "" && !isSafeID(payload.Context.ObjectID) {
		return services.QAQueryInput{}, errors.New("context.objectId invalide")
	}
	if payload.Context.Canton != "" && !regexp.MustCompile(`^[A-Z]{2,3}$`).MatchString(strings.ToUpper(payload.Context.Canton)) {
		return services.QAQueryInput{}, errors.New("context.canton invalide")
	}
	return payload, nil
}

func isAllowedValue(value string, allowed []string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func isSafeID(id string) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9:_\-\.]{2,120}$`).MatchString(id)
}

func normalizeLanguage(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	if regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`).MatchString(lang) {
		return lang
	}
	return "fr"
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, r *http.Request, statusCode int, code string, message string) {
	requestID, _ := r.Context().Value(requestIDKey).(string)
	writeJSON(w, statusCode, apiError{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	})
}
