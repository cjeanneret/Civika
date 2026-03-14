package rag

import (
	"errors"
	"fmt"
	"strings"
)

type Chunk struct {
	ID            string
	DocumentID    string
	TranslationID string
	Language      string
	SourcePath    string
	Title         string
	Text          string
	TokenCount    int
	Source        SourceMetadata
	Intervenants  []Intervenant
	Metadata      map[string]any
}

type ChunkConfig struct {
	ChunkSizeTokens int
	OverlapRatio    float64
}

func (c ChunkConfig) withDefaults() ChunkConfig {
	if c.ChunkSizeTokens == 0 {
		c.ChunkSizeTokens = 768
	}
	if c.OverlapRatio == 0 {
		c.OverlapRatio = 0.15
	}
	return c
}

func (c ChunkConfig) validate() error {
	if c.ChunkSizeTokens < 512 || c.ChunkSizeTokens > 1024 {
		return errors.New("chunk size must be between 512 and 1024 tokens")
	}
	if c.OverlapRatio < 0.10 || c.OverlapRatio > 0.20 {
		return errors.New("chunk overlap ratio must be between 0.10 and 0.20")
	}
	return nil
}

func ChunkDocuments(documents []Document, cfg ChunkConfig) ([]Chunk, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return nil, errors.New("documents are required")
	}

	var chunks []Chunk
	step := cfg.ChunkSizeTokens - int(float64(cfg.ChunkSizeTokens)*cfg.OverlapRatio)
	if step < 1 {
		return nil, errors.New("invalid chunk settings produce non-positive step")
	}

	for _, doc := range documents {
		tokens := strings.Fields(doc.Content)
		if len(tokens) == 0 {
			continue
		}
		translationKey := strings.TrimSpace(doc.TranslationID)
		if translationKey == "" {
			translationKey = fmt.Sprintf("%s:%s", doc.ID, normalizeLanguage(doc.Language))
		}
		translationKey = strings.NewReplacer(":", "_", " ", "_").Replace(translationKey)
		for start, part := 0, 0; start < len(tokens); start, part = start+step, part+1 {
			end := start + cfg.ChunkSizeTokens
			if end > len(tokens) {
				end = len(tokens)
			}
			if start >= end {
				break
			}
			text := strings.Join(tokens[start:end], " ")
			chunks = append(chunks, Chunk{
				ID:            fmt.Sprintf("%s_chunk_%04d", translationKey, part),
				DocumentID:    doc.ID,
				TranslationID: doc.TranslationID,
				Language:      doc.Language,
				SourcePath:    doc.SourcePath,
				Title:         doc.Title,
				Text:          text,
				TokenCount:    end - start,
				Source:        doc.Source,
				Intervenants:  doc.Intervenants,
				Metadata:      doc.Metadata,
			})
			if end == len(tokens) {
				break
			}
		}
	}
	if len(chunks) == 0 {
		return nil, errors.New("no chunks produced")
	}
	return chunks, nil
}
