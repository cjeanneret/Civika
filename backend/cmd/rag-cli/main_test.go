package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"civika/backend/config"
	"civika/backend/internal/rag"
)

type testTranslator struct {
	events *[]string
	fail   bool
}

func (t *testTranslator) Name() string {
	return "test-translator"
}

func (t *testTranslator) Translate(_ context.Context, req rag.TranslationRequest) (string, error) {
	*t.events = append(*t.events, "translate")
	if t.fail {
		return "", errors.New("translation failed")
	}
	return req.TargetLang + ":" + req.Text, nil
}

type testEmbedder struct {
	events *[]string
	calls  int
}

func (e *testEmbedder) Name() string {
	return "test-embedder"
}

func (e *testEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	*e.events = append(*e.events, "embed")
	e.calls++
	out := make([][]float32, 0, len(texts))
	for range texts {
		out = append(out, []float32{0.1, 0.2})
	}
	return out, nil
}

func (e *testEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	vectors, err := e.EmbedTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

type testIndexStore struct {
	events        *[]string
	upsertCalls   int
	metricsCalls  int
	lastChunkSize int
}

func (s *testIndexStore) UpsertChunks(_ context.Context, items []rag.EmbeddedChunk) error {
	*s.events = append(*s.events, "upsert")
	s.upsertCalls++
	s.lastChunkSize = len(items)
	return nil
}

func (s *testIndexStore) UpsertIndexDocumentMetrics(_ context.Context, _ rag.UsageDocumentMetrics) error {
	*s.events = append(*s.events, "metrics")
	s.metricsCalls++
	return nil
}

func TestProcessDocumentGroupOrdersPipelineByDocument(t *testing.T) {
	t.Setenv("RAG_MODE", "local")
	cfg := config.LoadFromEnv()
	cfg.RAG.Mode = "llm"
	cfg.RAG.SupportedLanguages = []string{"fr", "en"}
	cfg.RAG.DefaultLanguage = "fr"

	document := rag.Document{
		ID:            "doc-1",
		TranslationID: "doc-1:fr",
		Language:      "fr",
		SourcePath:    "fixture/doc-1.md",
		Title:         "Titre",
		Content:       strings.Repeat("mot ", 540),
		Metadata:      map[string]any{},
	}

	events := []string{}
	translator := &testTranslator{events: &events}
	embedder := &testEmbedder{events: &events}
	store := &testIndexStore{events: &events}

	result, err := processDocumentGroup(processDocumentGroupInput{
		Cfg:              cfg,
		Ctx:              context.Background(),
		Store:            store,
		Embedder:         embedder,
		Translator:       translator,
		ExistingState:    map[string]rag.IndexDocumentState{},
		ChunkCfg:         rag.ChunkConfig{ChunkSizeTokens: 512, OverlapRatio: 0.10},
		IndexRunID:       "test-run",
		DocumentGroup:    []rag.Document{document},
		ProgressPosition: 1,
		ProgressTotal:    1,
	})
	if err != nil {
		t.Fatalf("processDocumentGroup returned error: %v", err)
	}
	if result.ChunkCount == 0 {
		t.Fatal("expected at least one chunk")
	}
	if store.upsertCalls != 1 {
		t.Fatalf("expected 1 upsert call, got %d", store.upsertCalls)
	}
	if store.metricsCalls != 1 {
		t.Fatalf("expected 1 metrics call, got %d", store.metricsCalls)
	}

	firstEmbed := -1
	firstUpsert := -1
	lastTranslate := -1
	for i, event := range events {
		if event == "translate" {
			lastTranslate = i
		}
		if event == "embed" && firstEmbed == -1 {
			firstEmbed = i
		}
		if event == "upsert" && firstUpsert == -1 {
			firstUpsert = i
		}
	}
	if lastTranslate == -1 || firstEmbed == -1 || firstUpsert == -1 {
		t.Fatalf("unexpected pipeline events: %v", events)
	}
	if lastTranslate > firstEmbed {
		t.Fatalf("expected translations before embeddings, events=%v", events)
	}
	if firstEmbed > firstUpsert {
		t.Fatalf("expected embeddings before upsert, events=%v", events)
	}
}

func TestProcessDocumentGroupStopsOnTranslationError(t *testing.T) {
	t.Setenv("RAG_MODE", "local")
	cfg := config.LoadFromEnv()
	cfg.RAG.Mode = "llm"
	cfg.RAG.SupportedLanguages = []string{"fr", "en"}
	cfg.RAG.DefaultLanguage = "fr"

	document := rag.Document{
		ID:            "doc-err",
		TranslationID: "doc-err:fr",
		Language:      "fr",
		SourcePath:    "fixture/doc-err.md",
		Title:         "Titre",
		Content:       strings.Repeat("mot ", 530),
		Metadata:      map[string]any{},
	}

	events := []string{}
	translator := &testTranslator{events: &events, fail: true}
	embedder := &testEmbedder{events: &events}
	store := &testIndexStore{events: &events}

	_, err := processDocumentGroup(processDocumentGroupInput{
		Cfg:              cfg,
		Ctx:              context.Background(),
		Store:            store,
		Embedder:         embedder,
		Translator:       translator,
		ExistingState:    map[string]rag.IndexDocumentState{},
		ChunkCfg:         rag.ChunkConfig{ChunkSizeTokens: 512, OverlapRatio: 0.10},
		IndexRunID:       "test-run",
		DocumentGroup:    []rag.Document{document},
		ProgressPosition: 1,
		ProgressTotal:    1,
	})
	if err == nil {
		t.Fatal("expected translation error")
	}
	if embedder.calls != 0 {
		t.Fatalf("expected no embed calls, got %d", embedder.calls)
	}
	if store.upsertCalls != 0 {
		t.Fatalf("expected no upsert calls, got %d", store.upsertCalls)
	}
}
