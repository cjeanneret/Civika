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
				DocumentID: "doc-1",
				Title:      "Titre source",
				SourcePath: "source.md",
				Text:       "Le Conseil federal propose une reforme fiscale pour renforcer le financement des infrastructures cantonales.",
			},
			Score: 0.8,
		},
		{
			Chunk: Chunk{
				DocumentID: "doc-1",
				Title:      "Titre source",
				SourcePath: "source.md",
				Text:       "Texte dupliqué qui ne doit pas creer une seconde phrase.",
			},
			Score: 0.79,
		},
		{
			Chunk: Chunk{
				DocumentID: "doc-2",
				Title:      "Deuxieme source",
				SourcePath: "source2.md",
				Text:       "Le projet prevoit aussi un mecanisme de controle budgetaire et une application progressive.",
			},
			Score: 0.7,
		},
	}

	summary, err := ExplainVotation(context.Background(), summarizer, "Question", hits, "fr")
	if err != nil {
		t.Fatalf("ExplainVotation returned error: %v", err)
	}
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if strings.Contains(summary, "Question:") {
		t.Fatalf("expected summary not to echo question, got %q", summary)
	}
	if strings.Contains(summary, "Texte dupliqué") {
		t.Fatalf("expected duplicate source to be ignored, got %q", summary)
	}
	if !strings.Contains(summary, "reforme fiscale") {
		t.Fatalf("expected summary to use source text, got %q", summary)
	}
}

func TestLLMSummarizerSendsShortResponseInstruction(t *testing.T) {
	var captured struct {
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
	}, "de")
	if err != nil {
		t.Fatalf("summarize error: %v", err)
	}

	if len(captured.Messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(captured.Messages))
	}
	userPrompt := captured.Messages[1].Content
	if !strings.Contains(userPrompt, "language code \"de\"") {
		t.Fatalf("expected language instruction in prompt, got: %q", userPrompt)
	}
	if len(userPrompt) > 1200 {
		t.Fatalf("expected prompt <= 1200 chars, got %d", len(userPrompt))
	}
}

func TestLLMSummarizerRetriesWhenReasoningOnlyResponse(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, exists := payload["max_tokens"]; exists {
			t.Fatalf("did not expect max_tokens in payload")
		}
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"content":   "",
							"reasoning": "draft reasoning only",
						},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "Resume final.",
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
	})
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	summary, err := summarizer.Summarize(context.Background(), "Quels sont les enjeux?", []SearchHit{
		{
			Chunk: Chunk{
				SourcePath: "src.md",
				Title:      "Source",
				Text:       "Texte source",
				Source: SourceMetadata{
					SourceURI: "https://example.test/source",
				},
			},
			Score: 0.91,
		},
	}, "fr")
	if err != nil {
		t.Fatalf("summarize error: %v", err)
	}
	if summary != "Resume final." {
		t.Fatalf("expected final summary, got %q", summary)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 attempts, got %d", callCount)
	}
}
