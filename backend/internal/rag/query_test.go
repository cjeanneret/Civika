package rag

import (
	"context"
	"testing"
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
