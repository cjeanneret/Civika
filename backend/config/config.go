package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"civika/backend/internal/langs"
)

type Config struct {
	APIHost      string
	APIPort      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	BodyMaxBytes int64
	QARateLimit  QARateLimitConfig
	Debug        DebugConfig
	LLM          LLMConfig
	LLMEmbedding LLMEmbeddingConfig
	Postgres     PostgresConfig
	RAG          RAGConfig
}

type QARateLimitConfig struct {
	QPS             float64
	Burst           int
	CleanupInterval time.Duration
}

type DebugConfig struct {
	Enabled bool
	LogPath string
}

type LLMConfig struct {
	Enabled               bool
	BaseURL               string
	APIKey                string
	ModelName             string
	Timeout               time.Duration
	TranslationTimeout    time.Duration
	TranslationMaxRetries int
	MaxPromptChars        int
}

type LLMEmbeddingConfig struct {
	Enabled       bool
	BaseURL       string
	APIKey        string
	ModelName     string
	Timeout       time.Duration
	MaxInputChars int
}

type PostgresConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

type RAGConfig struct {
	Mode                string
	IndexTimeout        time.Duration
	TopK                int
	ChunkSizeTokens     int
	ChunkOverlapRatio   float64
	TableName           string
	EmbeddingDimensions int
	SupportedLanguages  []string
	DefaultLanguage     string
	FallbackLanguage    string
}

func LoadFromEnv() Config {
	cfg := Config{
		APIHost:      getEnv("API_HOST", "0.0.0.0"),
		APIPort:      getEnv("API_PORT", "8080"),
		ReadTimeout:  getEnvDuration("API_READ_TIMEOUT", 10*time.Second),
		WriteTimeout: getEnvDuration("API_WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:  getEnvDuration("API_IDLE_TIMEOUT", 30*time.Second),
		BodyMaxBytes: getEnvInt64("API_BODY_MAX_BYTES", 1_048_576),
		QARateLimit: QARateLimitConfig{
			QPS:             getEnvFloat64("API_QA_RATE_LIMIT_QPS", 1.0),
			Burst:           getEnvInt("API_QA_RATE_LIMIT_BURST", 3),
			CleanupInterval: getEnvDuration("API_QA_RATE_LIMIT_CLEANUP_INTERVAL", time.Minute),
		},
		Debug: DebugConfig{
			Enabled: getEnvBool("DEBUG_LOG_ENABLED", false),
			LogPath: getEnv("DEBUG_LOG_PATH", ""),
		},
		LLM: LLMConfig{
			Enabled:               getEnvBool("LLM_ENABLED", false),
			BaseURL:               getEnv("LLM_BASE_URL", ""),
			APIKey:                getEnv("LLM_API_KEY", ""),
			ModelName:             getEnv("LLM_MODEL_NAME", ""),
			Timeout:               getEnvDuration("LLM_TIMEOUT", 10*time.Second),
			TranslationTimeout:    getEnvDuration("LLM_TRANSLATION_TIMEOUT", getEnvDuration("LLM_TIMEOUT", 10*time.Second)),
			TranslationMaxRetries: getEnvInt("LLM_TRANSLATION_MAX_RETRIES", 2),
			MaxPromptChars:        getEnvInt("LLM_MAX_PROMPT_CHARS", 4000),
		},
		LLMEmbedding: LLMEmbeddingConfig{
			Enabled:       getEnvBool("LLM_EMBEDDING_ENABLED", false),
			BaseURL:       getEnv("LLM_EMBEDDING_BASE_URL", getEnv("LLM_BASE_URL", "")),
			APIKey:        getEnv("LLM_EMBEDDING_API_KEY", getEnv("LLM_API_KEY", "")),
			ModelName:     getEnv("LLM_EMBEDDING_MODEL_NAME", ""),
			Timeout:       getEnvDuration("LLM_EMBEDDING_TIMEOUT", 10*time.Second),
			MaxInputChars: getEnvInt("LLM_EMBEDDING_MAX_INPUT_CHARS", 4000),
		},
		Postgres: PostgresConfig{
			Host:         getEnv("POSTGRES_HOST", "127.0.0.1"),
			Port:         getEnv("POSTGRES_PORT", "5432"),
			User:         getEnv("POSTGRES_USER", "postgres"),
			Password:     getEnv("POSTGRES_PASSWORD", ""),
			DBName:       getEnv("POSTGRES_DB", "civika"),
			SSLMode:      getEnv("POSTGRES_SSLMODE", "disable"),
			MaxOpenConns: getEnvInt("POSTGRES_MAX_OPEN_CONNS", 10),
			MaxIdleConns: getEnvInt("POSTGRES_MAX_IDLE_CONNS", 5),
		},
		RAG: RAGConfig{
			Mode:                getEnv("RAG_MODE", "local"),
			IndexTimeout:        getEnvDuration("RAG_INDEX_TIMEOUT", 0),
			TopK:                getEnvInt("RAG_TOP_K", 5),
			ChunkSizeTokens:     getEnvInt("RAG_CHUNK_SIZE_TOKENS", 768),
			ChunkOverlapRatio:   getEnvFloat64("RAG_CHUNK_OVERLAP_RATIO", 0.15),
			TableName:           getEnv("RAG_TABLE_NAME", "rag_chunks"),
			EmbeddingDimensions: getEnvInt("RAG_EMBEDDING_DIMENSIONS", 128),
			SupportedLanguages:  langs.ParseSupported(getEnv("RAG_SUPPORTED_LANGUAGES", ""), []string{"fr", "de", "it", "rm", "en"}),
			DefaultLanguage:     getEnv("RAG_DEFAULT_LANGUAGE", "fr"),
			FallbackLanguage:    getEnv("RAG_FALLBACK_LANGUAGE", "en"),
		},
	}
	cfg.RAG.DefaultLanguage = normalizeLanguageWithPreferredFallback(cfg.RAG.DefaultLanguage, cfg.RAG.SupportedLanguages, "fr")
	cfg.RAG.FallbackLanguage = normalizeLanguageWithPreferredFallback(cfg.RAG.FallbackLanguage, cfg.RAG.SupportedLanguages, "en")
	return cfg
}

func (c Config) ValidateRAGMode() error {
	if len(c.RAG.SupportedLanguages) == 0 {
		return fmt.Errorf("RAG_SUPPORTED_LANGUAGES must include at least one valid language")
	}
	mode := c.RAG.Mode
	switch mode {
	case "local":
		return nil
	case "llm":
		if !c.LLM.Enabled {
			return fmt.Errorf("RAG_MODE=llm requires LLM_ENABLED=true")
		}
		if !c.LLMEmbedding.Enabled {
			return fmt.Errorf("RAG_MODE=llm requires LLM_EMBEDDING_ENABLED=true")
		}
		if c.LLM.BaseURL == "" || c.LLM.ModelName == "" {
			return fmt.Errorf("RAG_MODE=llm requires LLM_BASE_URL and LLM_MODEL_NAME")
		}
		if c.LLMEmbedding.BaseURL == "" || c.LLMEmbedding.ModelName == "" {
			return fmt.Errorf("RAG_MODE=llm requires LLM_EMBEDDING_BASE_URL and LLM_EMBEDDING_MODEL_NAME")
		}
		return nil
	default:
		return fmt.Errorf("invalid RAG_MODE %q (allowed: local|llm)", mode)
	}
}

func (c Config) APIAddress() string {
	return fmt.Sprintf("%s:%s", c.APIHost, c.APIPort)
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvFloat64(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeLanguageWithPreferredFallback(raw string, supported []string, preferredFallback string) string {
	normalized := langs.Normalize(raw)
	if normalized != "" && langs.Contains(supported, normalized) {
		return normalized
	}
	fallback := langs.Normalize(preferredFallback)
	if fallback != "" && langs.Contains(supported, fallback) {
		return fallback
	}
	if len(supported) > 0 {
		return supported[0]
	}
	return "fr"
}
