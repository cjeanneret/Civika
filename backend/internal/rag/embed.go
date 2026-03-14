package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type Embedder interface {
	Name() string
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

type LLMEmbedderConfig struct {
	Enabled       bool
	BaseURL       string
	APIKey        string
	ModelName     string
	Timeout       time.Duration
	MaxInputChars int
}

type LLMEmbedder struct {
	cfg    LLMEmbedderConfig
	client *http.Client
}

func NewLLMEmbedder(cfg LLMEmbedderConfig) (*LLMEmbedder, error) {
	if !cfg.Enabled {
		return nil, errors.New("llm embedder is disabled")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return nil, errors.New("llm embedder requires base url and model name")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxInputChars <= 0 {
		cfg.MaxInputChars = 4000
	}
	return &LLMEmbedder{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (e *LLMEmbedder) Name() string {
	return "llm"
}

func (e *LLMEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, errors.New("texts are required")
	}
	sanitized := make([]string, 0, len(texts))
	for _, text := range texts {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return nil, errors.New("embedding input cannot be empty")
		}
		if len(trimmed) > e.cfg.MaxInputChars {
			trimmed = trimmed[:e.cfg.MaxInputChars]
		}
		sanitized = append(sanitized, trimmed)
	}

	payload := map[string]any{
		"model": e.cfg.ModelName,
		"input": sanitized,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal embeddings payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(e.cfg.BaseURL, "/")+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embeddings request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}

	res, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embeddings request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings request returned status %d", res.StatusCode)
	}

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read embeddings response: %w", err)
	}

	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("decode embeddings response: %w", err)
	}

	if len(response.Embeddings) > 0 {
		return response.Embeddings, nil
	}
	out := make([][]float32, 0, len(response.Data))
	for _, item := range response.Data {
		out = append(out, item.Embedding)
	}
	if len(out) == 0 {
		return nil, errors.New("embeddings response is empty")
	}
	return out, nil
}

func (e *LLMEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	vectors, err := e.EmbedTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, errors.New("query embedding response has unexpected size")
	}
	return vectors[0], nil
}

type DeterministicEmbedder struct {
	Dimensions int
}

func NewDeterministicEmbedder(dimensions int) *DeterministicEmbedder {
	if dimensions <= 0 {
		dimensions = 128
	}
	return &DeterministicEmbedder{Dimensions: dimensions}
}

func (e *DeterministicEmbedder) Name() string {
	return "deterministic"
}

func (e *DeterministicEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, errors.New("texts are required")
	}
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return nil, errors.New("embedding input cannot be empty")
		}
		out = append(out, hashToVector(trimmed, e.Dimensions))
	}
	return out, nil
}

func (e *DeterministicEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	vectors, err := e.EmbedTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

func hashToVector(text string, dimensions int) []float32 {
	vector := make([]float32, dimensions)
	for i := 0; i < dimensions; i++ {
		seed := fmt.Sprintf("%d:%s", i, text)
		sum := sha256.Sum256([]byte(seed))
		value := binary.BigEndian.Uint32(sum[:4])
		scaled := (float64(value) / float64(math.MaxUint32) * 2.0) - 1.0
		vector[i] = float32(scaled)
	}
	return vector
}
