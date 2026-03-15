package services

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"civika/backend/config"
)

type qaCacheContext struct {
	Language       string
	VotationID     string
	ObjectID       string
	Canton         string
	RAGMode        string
	EmbedderName   string
	SummarizerName string
	TopK           int
}

type qaCacheEntry struct {
	Key            string
	QuestionHash   string
	ContextHash    string
	QuestionVector []float32
	Output         QAQueryOutput
	CreatedAt      time.Time
	ExpiresAt      time.Time
	LastHitAt      time.Time
	HitCount       int
}

type QACache struct {
	mu  sync.Mutex
	cfg config.QACacheConfig
	now func() time.Time

	exactEntries    map[string]qaCacheEntry
	semanticEntries map[string]qaCacheEntry
}

func NewQACache(cfg config.QACacheConfig) *QACache {
	if !cfg.Enabled {
		return nil
	}
	return &QACache{
		cfg:             cfg,
		now:             time.Now,
		exactEntries:    make(map[string]qaCacheEntry),
		semanticEntries: make(map[string]qaCacheEntry),
	}
}

func (c *QACache) IsSemanticEnabledForQuestion(question string) bool {
	if c == nil || !c.cfg.SemanticEnabled {
		return false
	}
	return len(strings.TrimSpace(question)) >= c.cfg.MinSemanticQuestionChars
}

func (c *QACache) GetExact(questionSanitized string, ctx qaCacheContext) (QAQueryOutput, bool) {
	if c == nil {
		return QAQueryOutput{}, false
	}
	now := c.now().UTC()
	exactKey := buildExactCacheKey(questionSanitized, ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeExpiredLocked(now)
	entry, ok := c.exactEntries[exactKey]
	if !ok {
		return QAQueryOutput{}, false
	}
	entry.LastHitAt = now
	entry.HitCount++
	c.exactEntries[exactKey] = entry
	return cloneQAOutput(entry.Output), true
}

func (c *QACache) GetSemantic(questionVector []float32, questionSanitized string, ctx qaCacheContext) (QAQueryOutput, float64, bool) {
	if c == nil || !c.cfg.SemanticEnabled {
		return QAQueryOutput{}, 0, false
	}
	if len(questionVector) == 0 || len(strings.TrimSpace(questionSanitized)) < c.cfg.MinSemanticQuestionChars {
		return QAQueryOutput{}, 0, false
	}
	now := c.now().UTC()
	contextHash := buildContextHash(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeExpiredLocked(now)

	bestKey := ""
	bestScore := -1.0
	for key, entry := range c.semanticEntries {
		if entry.ContextHash != contextHash {
			continue
		}
		score := cosineSimilarity(questionVector, entry.QuestionVector)
		if score > bestScore {
			bestScore = score
			bestKey = key
		}
	}
	if bestKey == "" || bestScore < c.cfg.SimilarityThreshold {
		return QAQueryOutput{}, bestScore, false
	}

	entry := c.semanticEntries[bestKey]
	entry.LastHitAt = now
	entry.HitCount++
	c.semanticEntries[bestKey] = entry
	return cloneQAOutput(entry.Output), bestScore, true
}

func (c *QACache) Set(questionSanitized string, questionVector []float32, ctx qaCacheContext, output QAQueryOutput) {
	if c == nil {
		return
	}
	now := c.now().UTC()
	contextHash := buildContextHash(ctx)
	questionHash := hashString(strings.TrimSpace(questionSanitized))
	exactKey := buildExactCacheKey(questionSanitized, ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeExpiredLocked(now)

	c.exactEntries[exactKey] = qaCacheEntry{
		Key:          exactKey,
		QuestionHash: questionHash,
		ContextHash:  contextHash,
		Output:       cloneQAOutput(output),
		CreatedAt:    now,
		ExpiresAt:    now.Add(c.cfg.ExactTTL),
		LastHitAt:    now,
		HitCount:     0,
	}
	c.evictOldestLocked(c.exactEntries, c.cfg.ExactMaxEntries)

	if c.cfg.SemanticEnabled && len(questionVector) > 0 && len(strings.TrimSpace(questionSanitized)) >= c.cfg.MinSemanticQuestionChars {
		semanticKey := buildSemanticCacheKey(questionHash, contextHash)
		c.semanticEntries[semanticKey] = qaCacheEntry{
			Key:            semanticKey,
			QuestionHash:   questionHash,
			ContextHash:    contextHash,
			QuestionVector: cloneVector(questionVector),
			Output:         cloneQAOutput(output),
			CreatedAt:      now,
			ExpiresAt:      now.Add(c.cfg.SemanticTTL),
			LastHitAt:      now,
			HitCount:       0,
		}
		c.evictOldestLocked(c.semanticEntries, c.cfg.SemanticMaxEntries)
	}
}

func (c *QACache) removeExpiredLocked(now time.Time) {
	for key, entry := range c.exactEntries {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(c.exactEntries, key)
		}
	}
	for key, entry := range c.semanticEntries {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(c.semanticEntries, key)
		}
	}
}

func (c *QACache) evictOldestLocked(entries map[string]qaCacheEntry, maxEntries int) {
	if maxEntries <= 0 || len(entries) <= maxEntries {
		return
	}
	type pair struct {
		key       string
		createdAt time.Time
	}
	oldest := make([]pair, 0, len(entries))
	for key, entry := range entries {
		oldest = append(oldest, pair{key: key, createdAt: entry.CreatedAt})
	}
	sort.Slice(oldest, func(i, j int) bool {
		return oldest[i].createdAt.Before(oldest[j].createdAt)
	})
	toDelete := len(entries) - maxEntries
	for i := 0; i < toDelete; i++ {
		delete(entries, oldest[i].key)
	}
}

func buildExactCacheKey(questionSanitized string, ctx qaCacheContext) string {
	return hashString(strings.TrimSpace(questionSanitized) + "|" + buildContextSignature(ctx) + "|exact-v1")
}

func buildSemanticCacheKey(questionHash, contextHash string) string {
	return hashString(questionHash + "|" + contextHash + "|semantic-v1")
}

func buildContextHash(ctx qaCacheContext) string {
	return hashString(buildContextSignature(ctx))
}

func buildContextSignature(ctx qaCacheContext) string {
	parts := []string{
		"lang=" + strings.ToLower(strings.TrimSpace(ctx.Language)),
		"votation=" + strings.TrimSpace(ctx.VotationID),
		"object=" + strings.TrimSpace(ctx.ObjectID),
		"canton=" + strings.ToUpper(strings.TrimSpace(ctx.Canton)),
		"ragMode=" + strings.TrimSpace(ctx.RAGMode),
		"embedder=" + strings.TrimSpace(ctx.EmbedderName),
		"summarizer=" + strings.TrimSpace(ctx.SummarizerName),
		"topK=" + strings.TrimSpace(intToString(ctx.TopK)),
	}
	return strings.Join(parts, "|")
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func cloneQAOutput(in QAQueryOutput) QAQueryOutput {
	out := QAQueryOutput{
		Answer:   in.Answer,
		Language: in.Language,
		Meta: QAQueryMeta{
			Confidence: in.Meta.Confidence,
		},
	}
	if len(in.Citations) > 0 {
		out.Citations = make([]Citation, 0, len(in.Citations))
		out.Citations = append(out.Citations, in.Citations...)
	} else {
		out.Citations = []Citation{}
	}
	if len(in.Meta.UsedDocuments) > 0 {
		out.Meta.UsedDocuments = make([]string, 0, len(in.Meta.UsedDocuments))
		out.Meta.UsedDocuments = append(out.Meta.UsedDocuments, in.Meta.UsedDocuments...)
	} else {
		out.Meta.UsedDocuments = []string{}
	}
	return out
}

func cloneVector(in []float32) []float32 {
	if len(in) == 0 {
		return nil
	}
	out := make([]float32, len(in))
	copy(out, in)
	return out
}

func cosineSimilarity(a []float32, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return -1
	}
	var dot float64
	var normA float64
	var normB float64
	for i := 0; i < len(a); i++ {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return -1
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func intToString(value int) string {
	return strconv.Itoa(value)
}
