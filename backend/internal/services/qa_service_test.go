package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"civika/backend/config"
	"civika/backend/internal/rag"
)

type qaTestStore struct {
	searchCalls int
}

func (s *qaTestStore) InitSchema(_ context.Context) error {
	return nil
}

func (s *qaTestStore) UpsertChunks(_ context.Context, _ []rag.EmbeddedChunk) error {
	return nil
}

func (s *qaTestStore) SearchSimilar(_ context.Context, _ []float32, _ int) ([]rag.SearchHit, error) {
	s.searchCalls++
	return []rag.SearchHit{
		{
			Chunk: rag.Chunk{
				DocumentID: "doc-1",
				Title:      "Titre",
				Source: rag.SourceMetadata{
					SourceURI: "https://example.test/doc",
				},
			},
			Score: 0.91,
		},
	}, nil
}

type qaTestEmbedder struct {
	embedCalls int
}

func (e *qaTestEmbedder) Name() string {
	return "deterministic"
}

func (e *qaTestEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	e.embedCalls += len(texts)
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		out = append(out, e.embedFor(text))
	}
	return out, nil
}

func (e *qaTestEmbedder) EmbedQuery(_ context.Context, text string) ([]float32, error) {
	e.embedCalls++
	return e.embedFor(text), nil
}

func (e *qaTestEmbedder) embedFor(text string) []float32 {
	normalized := strings.ToLower(strings.TrimSpace(text))
	switch {
	case strings.Contains(normalized, "energie"):
		return []float32{1.0, 0.0}
	case strings.Contains(normalized, "fiscalite"):
		return []float32{0.0, 1.0}
	default:
		return []float32{0.98, 0.02}
	}
}

type qaTestSummarizer struct {
	calls int
}

func (s *qaTestSummarizer) Name() string {
	return "mock"
}

func (s *qaTestSummarizer) Summarize(_ context.Context, question string, _ []rag.SearchHit, language string) (string, error) {
	s.calls++
	return "Reponse (" + strings.TrimSpace(language) + "): " + strings.TrimSpace(question), nil
}

func TestQAServiceExactCacheHitSkipsRepeatedLLMPath(t *testing.T) {
	store := &qaTestStore{}
	embedder := &qaTestEmbedder{}
	summarizer := &qaTestSummarizer{}
	cache := NewQACache(config.QACacheConfig{
		Enabled:                  true,
		ExactTTL:                 30 * time.Minute,
		ExactMaxEntries:          100,
		SemanticEnabled:          false,
		SemanticTTL:              time.Hour,
		SemanticMaxEntries:       100,
		SimilarityThreshold:      0.90,
		MinSemanticQuestionChars: 24,
	})
	service := NewQAService(store, embedder, summarizer, 5, nil, "llm", cache)

	input := QAQueryInput{
		Question: "Quels sont les effets de la votation energie?",
		Language: "fr",
	}
	first, err := service.Query(context.Background(), input)
	if err != nil {
		t.Fatalf("first query error: %v", err)
	}
	second, err := service.Query(context.Background(), input)
	if err != nil {
		t.Fatalf("second query error: %v", err)
	}

	if first.Answer != second.Answer {
		t.Fatalf("expected identical cached answer")
	}
	if summarizer.calls != 1 {
		t.Fatalf("expected summarizer called once, got %d", summarizer.calls)
	}
	if store.searchCalls != 1 {
		t.Fatalf("expected store search called once, got %d", store.searchCalls)
	}
}

func TestQAServiceSemanticCacheHitForSimilarQuestion(t *testing.T) {
	store := &qaTestStore{}
	embedder := &qaTestEmbedder{}
	summarizer := &qaTestSummarizer{}
	cache := NewQACache(config.QACacheConfig{
		Enabled:                  true,
		ExactTTL:                 30 * time.Minute,
		ExactMaxEntries:          100,
		SemanticEnabled:          true,
		SemanticTTL:              24 * time.Hour,
		SemanticMaxEntries:       100,
		SimilarityThreshold:      0.90,
		MinSemanticQuestionChars: 10,
	})
	service := NewQAService(store, embedder, summarizer, 5, nil, "llm", cache)

	firstInput := QAQueryInput{
		Question: "Quels sont les effets de la votation energie?",
		Language: "fr",
	}
	secondInput := QAQueryInput{
		Question: "Impact de la votation sur l energie en Suisse?",
		Language: "fr",
	}

	first, err := service.Query(context.Background(), firstInput)
	if err != nil {
		t.Fatalf("first query error: %v", err)
	}
	second, err := service.Query(context.Background(), secondInput)
	if err != nil {
		t.Fatalf("second query error: %v", err)
	}

	if first.Answer != second.Answer {
		t.Fatalf("expected semantic cache answer reuse")
	}
	if summarizer.calls != 1 {
		t.Fatalf("expected summarizer called once with semantic hit, got %d", summarizer.calls)
	}
	if store.searchCalls != 1 {
		t.Fatalf("expected store search called once with semantic hit, got %d", store.searchCalls)
	}
	if embedder.embedCalls < 2 {
		t.Fatalf("expected at least two embed calls for semantic flow, got %d", embedder.embedCalls)
	}
}
