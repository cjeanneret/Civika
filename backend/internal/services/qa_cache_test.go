package services

import (
	"testing"
	"time"

	"civika/backend/config"
)

func TestQACacheSemanticRequiresContextMatch(t *testing.T) {
	cache := NewQACache(config.QACacheConfig{
		Enabled:                  true,
		ExactTTL:                 10 * time.Minute,
		ExactMaxEntries:          10,
		SemanticEnabled:          true,
		SemanticTTL:              time.Hour,
		SemanticMaxEntries:       10,
		SimilarityThreshold:      0.90,
		MinSemanticQuestionChars: 5,
	})
	output := QAQueryOutput{Answer: "cached", Language: "fr"}
	ctxA := qaCacheContext{Language: "fr", RAGMode: "llm", TopK: 5, VotationID: "ch-123"}
	ctxB := qaCacheContext{Language: "fr", RAGMode: "llm", TopK: 5, VotationID: "ch-999"}

	cache.Set("impact energie", []float32{1, 0}, ctxA, output)
	if _, _, hit := cache.GetSemantic([]float32{1, 0}, "impact energie", ctxA); !hit {
		t.Fatalf("expected semantic hit for same context")
	}
	if _, _, hit := cache.GetSemantic([]float32{1, 0}, "impact energie", ctxB); hit {
		t.Fatalf("expected semantic miss for different context")
	}
}

func TestQACacheSemanticThresholdApplied(t *testing.T) {
	cache := NewQACache(config.QACacheConfig{
		Enabled:                  true,
		ExactTTL:                 10 * time.Minute,
		ExactMaxEntries:          10,
		SemanticEnabled:          true,
		SemanticTTL:              time.Hour,
		SemanticMaxEntries:       10,
		SimilarityThreshold:      0.95,
		MinSemanticQuestionChars: 5,
	})
	output := QAQueryOutput{Answer: "cached", Language: "fr"}
	ctx := qaCacheContext{Language: "fr", RAGMode: "llm", TopK: 5}

	cache.Set("impact energie", []float32{1, 0}, ctx, output)
	if _, _, hit := cache.GetSemantic([]float32{0.8, 0.6}, "impact energie", ctx); hit {
		t.Fatalf("expected semantic miss below threshold")
	}
}

func TestQACacheMetricsSnapshotTracksHitsMissesAndSavings(t *testing.T) {
	cache := NewQACache(config.QACacheConfig{
		Enabled:                  true,
		ExactTTL:                 10 * time.Minute,
		ExactMaxEntries:          10,
		SemanticEnabled:          true,
		SemanticTTL:              time.Hour,
		SemanticMaxEntries:       10,
		SimilarityThreshold:      0.90,
		MinSemanticQuestionChars: 5,
	})
	ctx := qaCacheContext{Language: "fr", RAGMode: "llm", TopK: 5}
	cache.Set("impact energie", []float32{1, 0}, ctx, QAQueryOutput{Answer: "reponse test", Language: "fr"})

	if _, hit := cache.GetExact("impact energie", ctx); !hit {
		t.Fatalf("expected exact hit")
	}
	cache.RecordMiss()
	cache.RecordBypassSensitiveQuestion()

	snapshot := cache.MetricsSnapshot()
	if snapshot.ExactHits != 1 {
		t.Fatalf("expected exact hits=1, got %d", snapshot.ExactHits)
	}
	if snapshot.Misses != 1 {
		t.Fatalf("expected misses=1, got %d", snapshot.Misses)
	}
	if snapshot.BypassSensitiveQuestion != 1 {
		t.Fatalf("expected bypassSensitiveQuestion=1, got %d", snapshot.BypassSensitiveQuestion)
	}
	if snapshot.SavedTotalTokensEstimate <= 0 {
		t.Fatalf("expected savedTotalTokensEstimate > 0, got %d", snapshot.SavedTotalTokensEstimate)
	}
}
