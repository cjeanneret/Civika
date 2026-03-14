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
