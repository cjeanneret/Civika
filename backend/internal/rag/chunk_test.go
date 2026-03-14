package rag

import (
	"strings"
	"testing"
)

func TestChunkDocumentsProducesChunks(t *testing.T) {
	words := make([]string, 0, 1200)
	for i := 0; i < 1200; i++ {
		words = append(words, "token")
	}
	docs := []Document{
		{
			ID:         "doc1",
			SourcePath: "doc1.md",
			Title:      "Doc 1",
			Content:    strings.Join(words, " "),
		},
	}

	chunks, err := ChunkDocuments(docs, ChunkConfig{
		ChunkSizeTokens: 512,
		OverlapRatio:    0.10,
	})
	if err != nil {
		t.Fatalf("ChunkDocuments returned error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	if chunks[0].TokenCount != 512 {
		t.Fatalf("expected first chunk size 512, got %d", chunks[0].TokenCount)
	}
}

func TestChunkDocumentsRejectsInvalidConfig(t *testing.T) {
	_, err := ChunkDocuments([]Document{{ID: "doc1", Content: "a b c"}}, ChunkConfig{
		ChunkSizeTokens: 256,
		OverlapRatio:    0.30,
	})
	if err == nil {
		t.Fatal("expected error for invalid chunk config")
	}
}

func TestChunkDocumentsUsesTranslationAwareIDs(t *testing.T) {
	words := strings.Repeat("token ", 600)
	docs := []Document{
		{
			ID:            "doc-shared",
			TranslationID: "doc-shared:fr",
			Language:      "fr",
			Content:       words,
		},
		{
			ID:            "doc-shared",
			TranslationID: "doc-shared:de",
			Language:      "de",
			Content:       words,
		},
	}

	chunks, err := ChunkDocuments(docs, ChunkConfig{
		ChunkSizeTokens: 512,
		OverlapRatio:    0.10,
	})
	if err != nil {
		t.Fatalf("ChunkDocuments returned error: %v", err)
	}
	seen := map[string]struct{}{}
	for _, chunk := range chunks {
		if _, exists := seen[chunk.ID]; exists {
			t.Fatalf("duplicate chunk id detected: %s", chunk.ID)
		}
		seen[chunk.ID] = struct{}{}
	}
}
