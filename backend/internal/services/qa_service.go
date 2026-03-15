package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"civika/backend/internal/debuglog"
	"civika/backend/internal/langs"
	"civika/backend/internal/rag"
)

var (
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phonePattern = regexp.MustCompile(`\+?[0-9][0-9\-\s]{7,}[0-9]`)
)

type QAService struct {
	store      rag.VectorStore
	embedder   rag.Embedder
	summarizer rag.Summarizer
	topK       int
	metrics    rag.UsageMetricsWriter
	ragMode    string
	cache      *QACache
}

func NewQAService(store rag.VectorStore, embedder rag.Embedder, summarizer rag.Summarizer, topK int, metrics rag.UsageMetricsWriter, ragMode string, cache *QACache) *QAService {
	if topK <= 0 {
		topK = 5
	}
	return &QAService{
		store:      store,
		embedder:   embedder,
		summarizer: summarizer,
		topK:       topK,
		metrics:    metrics,
		ragMode:    strings.TrimSpace(ragMode),
		cache:      cache,
	}
}

func (s *QAService) Query(ctx context.Context, input QAQueryInput) (QAQueryOutput, error) {
	if s.store == nil || s.embedder == nil || s.summarizer == nil {
		return QAQueryOutput{}, errors.New("qa service is not configured")
	}

	question := strings.TrimSpace(input.Question)
	if question == "" {
		return QAQueryOutput{}, errors.New("question is required")
	}

	safeQuestion := sanitizeQuestion(question)
	cacheableQuestion := isCacheableQuestion(safeQuestion)
	cacheCtx := qaCacheContext{
		Language:       normalizeLanguage(input.Language),
		VotationID:     strings.TrimSpace(input.Context.VotationID),
		ObjectID:       strings.TrimSpace(input.Context.ObjectID),
		Canton:         strings.TrimSpace(input.Context.Canton),
		RAGMode:        normalizeNonEmptyString(s.ragMode, "unknown"),
		EmbedderName:   normalizeNonEmptyString(s.embedder.Name(), "unknown"),
		SummarizerName: normalizeNonEmptyString(s.summarizer.Name(), "unknown"),
		TopK:           s.topK,
	}
	requestID := strings.TrimSpace(debuglog.RunIDFromContext(ctx))
	ctx = rag.WithUsageScope(ctx, rag.UsageScope{
		Flow:      "qa_query",
		Mode:      normalizeNonEmptyString(s.ragMode, "unknown"),
		RequestID: requestID,
		RunID:     requestID,
	})
	ctx = rag.WithUsageEmitter(ctx, s.recordUsageEvent)
	if s.cache != nil && cacheableQuestion {
		if exactOutput, hit := s.cache.GetExact(safeQuestion, cacheCtx); hit {
			debuglog.Log(ctx, "H2", "backend/internal/services/qa_service.go:Query", "qa exact cache hit", map[string]any{
				"questionChars": len(safeQuestion),
			})
			return exactOutput, nil
		}
	}

	var (
		hits        []rag.SearchHit
		queryVector []float32
		err         error
	)
	semanticAttempted := false
	if s.cache != nil && cacheableQuestion && s.cache.IsSemanticEnabledForQuestion(safeQuestion) {
		semanticAttempted = true
		queryVector, err = s.embedder.EmbedQuery(ctx, safeQuestion)
		if err != nil {
			return QAQueryOutput{}, fmt.Errorf("embed query for semantic cache: %w", err)
		}
		if semanticOutput, score, hit := s.cache.GetSemantic(queryVector, safeQuestion, cacheCtx); hit {
			debuglog.Log(ctx, "H2", "backend/internal/services/qa_service.go:Query", "qa semantic cache hit", map[string]any{
				"score":         score,
				"questionChars": len(safeQuestion),
			})
			return semanticOutput, nil
		}
	}
	// #region agent log
	debuglog.Log(ctx, "H2", "backend/internal/services/qa_service.go:Query", "query rag start", map[string]any{
		"embedder":      s.embedder.Name(),
		"summarizer":    s.summarizer.Name(),
		"topK":          s.topK,
		"questionChars": len(safeQuestion),
		"semanticCache": semanticAttempted,
	})
	// #endregion
	queryStart := time.Now()
	if len(queryVector) > 0 {
		hits, err = s.store.SearchSimilar(ctx, queryVector, s.topK)
		if err != nil {
			err = fmt.Errorf("search similar: %w", err)
		}
	} else {
		hits, err = rag.QueryRAG(ctx, s.store, s.embedder, safeQuestion, s.topK)
	}
	// #region agent log
	debuglog.Log(ctx, "H2", "backend/internal/services/qa_service.go:Query", "query rag end", map[string]any{
		"durationMs": time.Since(queryStart).Milliseconds(),
		"error":      fmt.Sprint(err),
		"hits":       len(hits),
	})
	// #endregion
	if err != nil {
		return QAQueryOutput{}, err
	}
	if len(hits) == 0 {
		output := QAQueryOutput{
			Answer:    "Aucune source pertinente n'a ete trouvee.",
			Language:  normalizeLanguage(input.Language),
			Citations: []Citation{},
			Meta: QAQueryMeta{
				Confidence:    0,
				UsedDocuments: []string{},
			},
		}
		if s.cache != nil && cacheableQuestion {
			s.cache.Set(safeQuestion, queryVector, cacheCtx, output)
		}
		return output, nil
	}

	// #region agent log
	debuglog.Log(ctx, "H4", "backend/internal/services/qa_service.go:Query", "summarization start", map[string]any{
		"hits": len(hits),
	})
	// #endregion
	summaryStart := time.Now()
	answer, err := rag.ExplainVotation(ctx, s.summarizer, safeQuestion, hits)
	// #region agent log
	debuglog.Log(ctx, "H4", "backend/internal/services/qa_service.go:Query", "summarization end", map[string]any{
		"durationMs":  time.Since(summaryStart).Milliseconds(),
		"error":       fmt.Sprint(err),
		"answerChars": len(answer),
	})
	// #endregion
	if err != nil {
		return QAQueryOutput{}, err
	}

	citations := make([]Citation, 0, len(hits))
	usedDocsSet := map[string]struct{}{}
	for _, hit := range hits {
		if hit.Chunk.DocumentID != "" {
			usedDocsSet[hit.Chunk.DocumentID] = struct{}{}
		}
		citations = append(citations, Citation{
			SourceType: sourceTypeFromChunk(hit.Chunk),
			URL:        hit.Chunk.Source.SourceURI,
			Title:      hit.Chunk.Title,
		})
	}

	usedDocs := make([]string, 0, len(usedDocsSet))
	for docID := range usedDocsSet {
		usedDocs = append(usedDocs, docID)
	}
	sort.Strings(usedDocs)

	output := QAQueryOutput{
		Answer:    answer,
		Language:  normalizeLanguage(input.Language),
		Citations: dedupeCitations(citations),
		Meta: QAQueryMeta{
			Confidence:    computeConfidence(hits),
			UsedDocuments: usedDocs,
		},
	}
	if s.cache != nil && cacheableQuestion {
		s.cache.Set(safeQuestion, queryVector, cacheCtx, output)
	}
	return output, nil
}

func isCacheableQuestion(sanitizedQuestion string) bool {
	value := strings.TrimSpace(sanitizedQuestion)
	if value == "" {
		return false
	}
	if strings.Contains(value, "[redacted-email]") || strings.Contains(value, "[redacted-phone]") {
		return false
	}
	return true
}

func (s *QAService) recordUsageEvent(ctx context.Context, event rag.UsageEvent) {
	if s.metrics == nil {
		return
	}
	if err := s.metrics.RecordUsageEvent(ctx, event); err != nil {
		debuglog.Log(ctx, "H5", "backend/internal/services/qa_service.go:recordUsageEvent", "usage metrics write failed", map[string]any{
			"error": err.Error(),
		})
	}
}

func normalizeNonEmptyString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func sanitizeQuestion(question string) string {
	trimmed := strings.TrimSpace(question)
	if len(trimmed) > 1200 {
		trimmed = trimmed[:1200]
	}
	emailRegex := emailPattern
	trimmed = emailRegex.ReplaceAllString(trimmed, "[redacted-email]")
	phoneRegex := phonePattern
	trimmed = phoneRegex.ReplaceAllString(trimmed, "[redacted-phone]")
	return strings.TrimSpace(trimmed)
}

func normalizeLanguage(lang string) string {
	l := langs.Normalize(lang)
	if l == "" {
		return "fr"
	}
	return l
}

func sourceTypeFromChunk(chunk rag.Chunk) string {
	system := strings.ToLower(strings.TrimSpace(chunk.Source.SourceSystem))
	switch {
	case strings.Contains(system, "openparl"), strings.Contains(system, "admin"), strings.Contains(system, "opendata"):
		return "official"
	default:
		return "other"
	}
}

func dedupeCitations(items []Citation) []Citation {
	seen := map[string]struct{}{}
	out := make([]Citation, 0, len(items))
	for _, item := range items {
		key := item.URL + "|" + item.Title
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func computeConfidence(hits []rag.SearchHit) float64 {
	if len(hits) == 0 {
		return 0
	}
	var sum float64
	for _, hit := range hits {
		sum += hit.Score
	}
	score := sum / float64(len(hits))
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return math.Round(score*100) / 100
}
