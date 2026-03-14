package rag

import "testing"

func TestBuildChunkDebugStatsAggregates(t *testing.T) {
	documents := []Document{
		{ID: "doc-a"},
		{ID: "doc-b"},
	}
	chunks := []Chunk{
		{
			ID:         "a1",
			DocumentID: "doc-a",
			Title:      "Doc A",
			SourcePath: "a.md",
			Language:   "fr",
			TokenCount: 100,
			Source:     SourceMetadata{SourceSystem: "openparldata"},
		},
		{
			ID:         "a2",
			DocumentID: "doc-a",
			Title:      "Doc A",
			SourcePath: "a.md",
			Language:   "fr",
			TokenCount: 200,
			Source:     SourceMetadata{SourceSystem: "openparldata"},
		},
		{
			ID:         "b1",
			DocumentID: "doc-b",
			Title:      "Doc B",
			SourcePath: "b.md",
			Language:   "de",
			TokenCount: 300,
			Source:     SourceMetadata{SourceSystem: "fixture"},
		},
	}

	stats := BuildChunkDebugStats(documents, chunks)
	if stats.DocumentCount != 2 {
		t.Fatalf("expected 2 documents, got %d", stats.DocumentCount)
	}
	if stats.ChunkCount != 3 {
		t.Fatalf("expected 3 chunks, got %d", stats.ChunkCount)
	}
	if stats.Tokens.Min != 100 {
		t.Fatalf("expected min 100, got %d", stats.Tokens.Min)
	}
	if stats.Tokens.Max != 300 {
		t.Fatalf("expected max 300, got %d", stats.Tokens.Max)
	}
	if stats.Tokens.Avg != 200 {
		t.Fatalf("expected avg 200, got %d", stats.Tokens.Avg)
	}
	if stats.Tokens.P95 != 300 {
		t.Fatalf("expected p95 300, got %d", stats.Tokens.P95)
	}
	if len(stats.ByLanguage) != 2 {
		t.Fatalf("expected 2 language stats, got %d", len(stats.ByLanguage))
	}
	if stats.ByLanguage[0].Key != "fr" || stats.ByLanguage[0].Count != 2 {
		t.Fatalf("unexpected first language stat: %+v", stats.ByLanguage[0])
	}
	if len(stats.TopDocuments) != 2 {
		t.Fatalf("expected 2 top document stats, got %d", len(stats.TopDocuments))
	}
	if stats.TopDocuments[0].DocumentID != "doc-a" || stats.TopDocuments[0].ChunkCount != 2 {
		t.Fatalf("unexpected top document stat: %+v", stats.TopDocuments[0])
	}
}

func TestBuildChunkDebugStatsEmptyChunks(t *testing.T) {
	stats := BuildChunkDebugStats([]Document{{ID: "doc-a"}}, nil)
	if stats.DocumentCount != 1 {
		t.Fatalf("expected document count 1, got %d", stats.DocumentCount)
	}
	if stats.ChunkCount != 0 {
		t.Fatalf("expected chunk count 0, got %d", stats.ChunkCount)
	}
	if stats.Tokens.P95 != 0 {
		t.Fatalf("expected p95 0, got %d", stats.Tokens.P95)
	}
}

