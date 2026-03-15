package config

import "testing"

func TestValidateRAGModeLocal(t *testing.T) {
	cfg := LoadFromEnv()
	cfg.RAG.Mode = "local"
	if err := cfg.ValidateRAGMode(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRAGModeLLMRequiresFlags(t *testing.T) {
	cfg := LoadFromEnv()
	cfg.RAG.Mode = "llm"
	cfg.LLM.Enabled = false
	cfg.LLMEmbedding.Enabled = false
	if err := cfg.ValidateRAGMode(); err == nil {
		t.Fatalf("expected error for invalid llm mode config")
	}
}

func TestLoadFromEnvQARateLimitDefaults(t *testing.T) {
	cfg := LoadFromEnv()
	if cfg.QARateLimit.QPS != 1.0 {
		t.Fatalf("expected default QPS 1.0, got %v", cfg.QARateLimit.QPS)
	}
	if cfg.QARateLimit.Burst != 3 {
		t.Fatalf("expected default Burst 3, got %d", cfg.QARateLimit.Burst)
	}
	if cfg.QARateLimit.CleanupInterval.String() != "1m0s" {
		t.Fatalf("expected default CleanupInterval 1m0s, got %s", cfg.QARateLimit.CleanupInterval)
	}
}

func TestLoadFromEnvQARateLimitOverrides(t *testing.T) {
	t.Setenv("API_QA_RATE_LIMIT_QPS", "2.5")
	t.Setenv("API_QA_RATE_LIMIT_BURST", "7")
	t.Setenv("API_QA_RATE_LIMIT_CLEANUP_INTERVAL", "30s")

	cfg := LoadFromEnv()
	if cfg.QARateLimit.QPS != 2.5 {
		t.Fatalf("expected QPS 2.5, got %v", cfg.QARateLimit.QPS)
	}
	if cfg.QARateLimit.Burst != 7 {
		t.Fatalf("expected Burst 7, got %d", cfg.QARateLimit.Burst)
	}
	if cfg.QARateLimit.CleanupInterval.String() != "30s" {
		t.Fatalf("expected CleanupInterval 30s, got %s", cfg.QARateLimit.CleanupInterval)
	}
}

func TestLoadFromEnvLLMOutputTokenOverrides(t *testing.T) {
	t.Setenv("LLM_MAX_OUTPUT_TOKENS_SUMMARIZATION", "180")
	t.Setenv("LLM_MAX_OUTPUT_TOKENS_TRANSLATION", "620")
	t.Setenv("LLM_TRANSLATION_MAX_RETRIES", "4")

	cfg := LoadFromEnv()
	if cfg.LLM.MaxOutputTokens != 180 {
		t.Fatalf("expected summary output cap 180, got %d", cfg.LLM.MaxOutputTokens)
	}
	if cfg.LLM.TranslationMaxTokens != 620 {
		t.Fatalf("expected translation output cap 620, got %d", cfg.LLM.TranslationMaxTokens)
	}
	if cfg.LLM.TranslationMaxRetries != 4 {
		t.Fatalf("expected translation retries 4, got %d", cfg.LLM.TranslationMaxRetries)
	}
}

func TestLoadFromEnvQACacheDefaults(t *testing.T) {
	cfg := LoadFromEnv()
	if cfg.QACache.Enabled {
		t.Fatalf("expected QA cache disabled by default")
	}
	if cfg.QACache.ExactTTL.String() != "10m0s" {
		t.Fatalf("expected exact ttl 10m0s, got %s", cfg.QACache.ExactTTL)
	}
	if cfg.QACache.ExactMaxEntries != 500 {
		t.Fatalf("expected exact max entries 500, got %d", cfg.QACache.ExactMaxEntries)
	}
	if cfg.QACache.SemanticEnabled {
		t.Fatalf("expected semantic cache disabled by default")
	}
	if cfg.QACache.SemanticTTL.String() != "24h0m0s" {
		t.Fatalf("expected semantic ttl 24h0m0s, got %s", cfg.QACache.SemanticTTL)
	}
	if cfg.QACache.SemanticMaxEntries != 2000 {
		t.Fatalf("expected semantic max entries 2000, got %d", cfg.QACache.SemanticMaxEntries)
	}
	if cfg.QACache.SimilarityThreshold != 0.90 {
		t.Fatalf("expected similarity threshold 0.90, got %v", cfg.QACache.SimilarityThreshold)
	}
	if cfg.QACache.MinSemanticQuestionChars != 24 {
		t.Fatalf("expected min semantic question chars 24, got %d", cfg.QACache.MinSemanticQuestionChars)
	}
}

func TestLoadFromEnvQACacheOverrides(t *testing.T) {
	t.Setenv("QA_CACHE_ENABLED", "true")
	t.Setenv("QA_CACHE_EXACT_TTL", "20m")
	t.Setenv("QA_CACHE_EXACT_MAX_ENTRIES", "1000")
	t.Setenv("QA_CACHE_SEMANTIC_ENABLED", "true")
	t.Setenv("QA_CACHE_SEMANTIC_TTL", "36h")
	t.Setenv("QA_CACHE_SEMANTIC_MAX_ENTRIES", "5000")
	t.Setenv("QA_CACHE_SEMANTIC_SIMILARITY_THRESHOLD", "0.93")
	t.Setenv("QA_CACHE_SEMANTIC_MIN_QUESTION_CHARS", "18")

	cfg := LoadFromEnv()
	if !cfg.QACache.Enabled {
		t.Fatalf("expected QA cache enabled override")
	}
	if cfg.QACache.ExactTTL.String() != "20m0s" {
		t.Fatalf("expected exact ttl 20m0s, got %s", cfg.QACache.ExactTTL)
	}
	if cfg.QACache.ExactMaxEntries != 1000 {
		t.Fatalf("expected exact max entries 1000, got %d", cfg.QACache.ExactMaxEntries)
	}
	if !cfg.QACache.SemanticEnabled {
		t.Fatalf("expected semantic cache enabled override")
	}
	if cfg.QACache.SemanticTTL.String() != "36h0m0s" {
		t.Fatalf("expected semantic ttl 36h0m0s, got %s", cfg.QACache.SemanticTTL)
	}
	if cfg.QACache.SemanticMaxEntries != 5000 {
		t.Fatalf("expected semantic max entries 5000, got %d", cfg.QACache.SemanticMaxEntries)
	}
	if cfg.QACache.SimilarityThreshold != 0.93 {
		t.Fatalf("expected similarity threshold 0.93, got %v", cfg.QACache.SimilarityThreshold)
	}
	if cfg.QACache.MinSemanticQuestionChars != 18 {
		t.Fatalf("expected min semantic question chars 18, got %d", cfg.QACache.MinSemanticQuestionChars)
	}
}
