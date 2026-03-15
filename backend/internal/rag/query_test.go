package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	hits []SearchHit
}

func (s *fakeStore) InitSchema(_ context.Context) error {
	return nil
}

func (s *fakeStore) UpsertChunks(_ context.Context, _ []EmbeddedChunk) error {
	return nil
}

func (s *fakeStore) SearchSimilar(_ context.Context, _ []float32, _ int) ([]SearchHit, error) {
	return s.hits, nil
}

func TestQueryRAGWithDeterministicEmbedder(t *testing.T) {
	store := &fakeStore{
		hits: []SearchHit{
			{
				Chunk: Chunk{
					ID:         "chunk-1",
					DocumentID: "doc-1",
					SourcePath: "doc.md",
					Title:      "Doc",
					Text:       "contenu",
					TokenCount: 1,
				},
				Score: 0.9,
			},
		},
	}
	embedder := NewDeterministicEmbedder(16)

	hits, err := QueryRAG(context.Background(), store, embedder, "question test", 3)
	if err != nil {
		t.Fatalf("QueryRAG returned error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
}

func TestExplainVotationDeterministic(t *testing.T) {
	summarizer := NewDeterministicSummarizer()
	hits := []SearchHit{
		{
			Chunk: Chunk{
				Title:      "Titre source",
				SourcePath: "source.md",
			},
			Score: 0.8,
		},
	}

	summary, err := ExplainVotation(context.Background(), summarizer, "Question", hits)
	if err != nil {
		t.Fatalf("ExplainVotation returned error: %v", err)
	}
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestLLMSummarizerSendsOutputTokenCapAndShortResponseInstruction(t *testing.T) {
	var captured struct {
		MaxTokens int `json:"max_tokens"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/chat/completions") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "Resume court.",
					},
				},
			},
		})
	}))
	defer server.Close()

	summarizer, err := NewLLMSummarizer(LLMSummarizerConfig{
		Enabled:         true,
		BaseURL:         server.URL,
		ModelName:       "test-model",
		Timeout:         2 * time.Second,
		MaxPromptChars:  1200,
		MaxOutputTokens: 77,
	})
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	_, err = summarizer.Summarize(context.Background(), "Quels sont les impacts?", []SearchHit{
		{
			Chunk: Chunk{
				SourcePath: "src.md",
				Title:      "Source",
				Text:       strings.Repeat("texte ", 200),
				Source: SourceMetadata{
					SourceURI: "https://example.test/source",
				},
			},
			Score: 0.91,
		},
	})
	if err != nil {
		t.Fatalf("summarize error: %v", err)
	}

	if captured.MaxTokens != 77 {
		t.Fatalf("expected max_tokens=77, got %d", captured.MaxTokens)
	}
	if len(captured.Messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(captured.Messages))
	}
	userPrompt := captured.Messages[1].Content
	if !strings.Contains(userPrompt, "1 ou 2 phrases maximum") {
		t.Fatalf("expected short-response instruction in prompt, got: %q", userPrompt)
	}
	if len(userPrompt) > 1200 {
		t.Fatalf("expected prompt <= 1200 chars, got %d", len(userPrompt))
	}
}
