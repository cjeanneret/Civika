package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"civika/backend/internal/debuglog"
)

func QueryRAG(ctx context.Context, store VectorStore, embedder Embedder, question string, topK int) ([]SearchHit, error) {
	if store == nil {
		return nil, errors.New("vector store is required")
	}
	if embedder == nil {
		return nil, errors.New("embedder is required")
	}
	trimmedQuestion := strings.TrimSpace(question)
	if trimmedQuestion == "" {
		return nil, errors.New("question is required")
	}
	embedStart := time.Now()
	// #region agent log
	debuglog.Log(ctx, "H2", "backend/internal/rag/query.go:QueryRAG", "embed query start", map[string]any{
		"embedder":      embedder.Name(),
		"questionChars": len(trimmedQuestion),
	})
	// #endregion
	queryVector, err := embedder.EmbedQuery(ctx, trimmedQuestion)
	// #region agent log
	debuglog.Log(ctx, "H2", "backend/internal/rag/query.go:QueryRAG", "embed query end", map[string]any{
		"durationMs":   time.Since(embedStart).Milliseconds(),
		"error":        fmt.Sprint(err),
		"vectorLength": len(queryVector),
	})
	// #endregion
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	searchStart := time.Now()
	// #region agent log
	debuglog.Log(ctx, "H3", "backend/internal/rag/query.go:QueryRAG", "search similar start", map[string]any{
		"topK":         topK,
		"vectorLength": len(queryVector),
	})
	// #endregion
	hits, err := store.SearchSimilar(ctx, queryVector, topK)
	// #region agent log
	debuglog.Log(ctx, "H3", "backend/internal/rag/query.go:QueryRAG", "search similar end", map[string]any{
		"durationMs": time.Since(searchStart).Milliseconds(),
		"error":      fmt.Sprint(err),
		"hits":       len(hits),
	})
	// #endregion
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}
	return hits, nil
}

type Summarizer interface {
	Summarize(ctx context.Context, question string, hits []SearchHit) (string, error)
	Name() string
}

type LLMSummarizerConfig struct {
	Enabled        bool
	BaseURL        string
	APIKey         string
	ModelName      string
	Timeout        time.Duration
	MaxPromptChars int
}

type LLMSummarizer struct {
	cfg    LLMSummarizerConfig
	client *http.Client
}

func NewLLMSummarizer(cfg LLMSummarizerConfig) (*LLMSummarizer, error) {
	if !cfg.Enabled {
		return nil, errors.New("llm summarizer is disabled")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return nil, errors.New("llm summarizer requires base url and model name")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxPromptChars <= 0 {
		cfg.MaxPromptChars = 4000
	}
	return &LLMSummarizer{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (s *LLMSummarizer) Name() string {
	return "llm"
}

func (s *LLMSummarizer) Summarize(ctx context.Context, question string, hits []SearchHit) (string, error) {
	if strings.TrimSpace(question) == "" {
		return "", errors.New("question is required")
	}
	if len(hits) == 0 {
		return "", errors.New("hits are required")
	}

	prompt := buildSummaryPrompt(question, hits)
	if len(prompt) > s.cfg.MaxPromptChars {
		prompt = prompt[:s.cfg.MaxPromptChars]
	}

	payload := map[string]any{
		"model": s.cfg.ModelName,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You summarize public Swiss votation sources. Do not infer personal user data.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal summarize payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.cfg.BaseURL, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create summarize request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	}

	requestStart := time.Now()
	// #region agent log
	debuglog.Log(ctx, "H4", "backend/internal/rag/query.go:LLMSummarizer.Summarize", "llm summarize request start", map[string]any{
		"baseURL": strings.TrimRight(s.cfg.BaseURL, "/"),
		"model":   s.cfg.ModelName,
		"hits":    len(hits),
	})
	// #endregion
	res, err := s.client.Do(req)
	// #region agent log
	debuglog.Log(ctx, "H4", "backend/internal/rag/query.go:LLMSummarizer.Summarize", "llm summarize request end", map[string]any{
		"durationMs": time.Since(requestStart).Milliseconds(),
		"error":      fmt.Sprint(err),
	})
	// #endregion
	if err != nil {
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "summarization",
			ProviderName: "llm",
			ModelName:    s.cfg.ModelName,
			InputChars:   len(prompt),
			OutputChars:  0,
			UsageSource:  "unknown",
			Status:       "error",
			DurationMS:   time.Since(requestStart).Milliseconds(),
			ErrorCode:    "request_failed",
		})
		return "", fmt.Errorf("summarize request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "summarization",
			ProviderName: "llm",
			ModelName:    s.cfg.ModelName,
			InputChars:   len(prompt),
			OutputChars:  0,
			UsageSource:  "unknown",
			Status:       "error",
			DurationMS:   time.Since(requestStart).Milliseconds(),
			ErrorCode:    fmt.Sprintf("status_%d", res.StatusCode),
		})
		return "", fmt.Errorf("summarize request returned status %d", res.StatusCode)
	}

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024))
	if err != nil {
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "summarization",
			ProviderName: "llm",
			ModelName:    s.cfg.ModelName,
			InputChars:   len(prompt),
			OutputChars:  0,
			UsageSource:  "unknown",
			Status:       "error",
			DurationMS:   time.Since(requestStart).Milliseconds(),
			ErrorCode:    "read_failed",
		})
		return "", fmt.Errorf("read summarize response: %w", err)
	}
	usage, usageErr := ParseUsageFromResponseBody(responseBody)
	if usageErr != nil {
		usage = UsageEvent{UsageSource: "unknown"}
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "summarization",
			ProviderName: "llm",
			ModelName:    s.cfg.ModelName,
			InputChars:   len(prompt),
			OutputChars:  0,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			UsageSource:  usage.UsageSource,
			Status:       "error",
			DurationMS:   time.Since(requestStart).Milliseconds(),
			ErrorCode:    "decode_failed",
		})
		return "", fmt.Errorf("decode summarize response: %w", err)
	}
	if len(response.Choices) == 0 {
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "summarization",
			ProviderName: "llm",
			ModelName:    s.cfg.ModelName,
			InputChars:   len(prompt),
			OutputChars:  0,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			UsageSource:  usage.UsageSource,
			Status:       "error",
			DurationMS:   time.Since(requestStart).Milliseconds(),
			ErrorCode:    "no_choices",
		})
		return "", errors.New("summarize response has no choices")
	}

	summary := strings.TrimSpace(response.Choices[0].Message.Content)
	if summary == "" {
		summary = strings.TrimSpace(response.Choices[0].Text)
	}
	if summary == "" {
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "summarization",
			ProviderName: "llm",
			ModelName:    s.cfg.ModelName,
			InputChars:   len(prompt),
			OutputChars:  0,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			UsageSource:  usage.UsageSource,
			Status:       "error",
			DurationMS:   time.Since(requestStart).Milliseconds(),
			ErrorCode:    "empty_summary",
		})
		return "", errors.New("summarize response is empty")
	}
	emitUsageEvent(ctx, UsageEvent{
		Operation:    "summarization",
		ProviderName: "llm",
		ModelName:    s.cfg.ModelName,
		InputChars:   len(prompt),
		OutputChars:  len(summary),
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		UsageSource:  usage.UsageSource,
		Status:       "success",
		DurationMS:   time.Since(requestStart).Milliseconds(),
	})
	return summary, nil
}

type DeterministicSummarizer struct{}

func NewDeterministicSummarizer() *DeterministicSummarizer {
	return &DeterministicSummarizer{}
}

func (s *DeterministicSummarizer) Name() string {
	return "deterministic"
}

func (s *DeterministicSummarizer) Summarize(_ context.Context, question string, hits []SearchHit) (string, error) {
	if len(hits) == 0 {
		return "", errors.New("hits are required")
	}
	var b strings.Builder
	b.WriteString("Resume deterministe (LLM desactive). ")
	b.WriteString("Question: ")
	b.WriteString(strings.TrimSpace(question))
	b.WriteString(". Sources principales: ")
	for i, hit := range hits {
		if i >= 3 {
			break
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(hit.Chunk.Title)
	}
	return b.String(), nil
}

func ExplainVotation(ctx context.Context, summarizer Summarizer, question string, hits []SearchHit) (string, error) {
	if summarizer == nil {
		return "", errors.New("summarizer is required")
	}
	return summarizer.Summarize(ctx, question, hits)
}

func buildSummaryPrompt(question string, hits []SearchHit) string {
	var b strings.Builder
	b.WriteString("Question utilisateur:\n")
	b.WriteString(question)
	b.WriteString("\n\nExtraits de documents publics indexes:\n")
	for i, hit := range hits {
		if i >= 8 {
			break
		}
		intervenants := formatIntervenants(hit.Chunk.Intervenants)
		b.WriteString(fmt.Sprintf(
			"\n[%d] source=%s title=%s lang=%s score=%.4f source_uri=%s intervenants=%s\n",
			i+1,
			hit.Chunk.SourcePath,
			hit.Chunk.Title,
			normalizeNonEmpty(hit.Chunk.Language, "fr"),
			hit.Score,
			hit.Chunk.Source.SourceURI,
			intervenants,
		))
		b.WriteString(hit.Chunk.Text)
		b.WriteString("\n")
	}
	b.WriteString("\nProduis un resume factuel et concis en francais, uniquement base sur ces sources.")
	return b.String()
}

func formatIntervenants(intervenants []Intervenant) string {
	if len(intervenants) == 0 {
		return "none"
	}
	var parts []string
	for _, intervenant := range intervenants {
		fullName := strings.TrimSpace(intervenant.FirstName + " " + intervenant.LastName)
		if fullName == "" {
			continue
		}
		if intervenant.Role != "" {
			fullName = fullName + " (" + intervenant.Role + ")"
		}
		parts = append(parts, fullName)
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
