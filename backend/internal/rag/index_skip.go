package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

const (
	sourceFingerprintConfidenceHigh = "high"
	sourceFingerprintConfidenceLow  = "low"
)

type IndexTranslationState struct {
	TranslationID string
	Lang          string
	Title         string
	Content       string
	Status        string
	Provider      string
	SourceHash    string
	ContentHash   string
}

type IndexDocumentState struct {
	DocumentID                  string
	SourceFingerprint           string
	SourceFingerprintConfidence string
	IndexComplete               bool
	IndexedChunkCount           int
	Translations                map[string]IndexTranslationState
}

type IndexSkipReport struct {
	GroupedDocuments int
	SkippedDocuments int
	ProcessedDocs    int
}

func PrepareDocumentsForIndex(documents []Document) []Document {
	out := make([]Document, 0, len(documents))
	for _, doc := range documents {
		out = append(out, withIndexMetadata(doc))
	}
	return out
}

func BuildTranslationSourceHash(sourceLang, title, content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceLang) + "|" + strings.TrimSpace(title) + "|" + strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func FilterDocumentsForIndex(documents []Document, supportedLanguages []string, mode string, chunkCfg ChunkConfig, existing map[string]IndexDocumentState) ([]Document, IndexSkipReport) {
	prepared := PrepareDocumentsForIndex(documents)
	grouped := map[string][]Document{}
	for _, doc := range prepared {
		grouped[doc.ID] = append(grouped[doc.ID], doc)
	}

	report := IndexSkipReport{
		GroupedDocuments: len(grouped),
	}
	out := make([]Document, 0, len(prepared))
	docIDs := make([]string, 0, len(grouped))
	for docID := range grouped {
		docIDs = append(docIDs, docID)
	}
	sort.Strings(docIDs)

	for _, docID := range docIDs {
		docs := grouped[docID]
		state, exists := existing[docID]
		if !exists {
			out = append(out, docs...)
			continue
		}
		requiredLangs := requiredLanguagesForGroup(docs, supportedLanguages, mode)
		if shouldSkipDocumentGroup(docs, state, requiredLangs, chunkCfg) {
			report.SkippedDocuments++
			continue
		}
		out = append(out, docs...)
	}
	report.ProcessedDocs = report.GroupedDocuments - report.SkippedDocuments
	return out, report
}

func requiredLanguagesForGroup(docs []Document, supportedLanguages []string, mode string) []string {
	if mode == "llm" && len(supportedLanguages) > 0 {
		return supportedLanguages
	}
	langsInDocs := map[string]struct{}{}
	for _, doc := range docs {
		lang := strings.TrimSpace(doc.Language)
		if lang != "" {
			langsInDocs[lang] = struct{}{}
		}
	}
	out := make([]string, 0, len(langsInDocs))
	for lang := range langsInDocs {
		out = append(out, lang)
	}
	sort.Strings(out)
	return out
}

func shouldSkipDocumentGroup(docs []Document, state IndexDocumentState, requiredLangs []string, chunkCfg ChunkConfig) bool {
	if !state.IndexComplete {
		return false
	}
	expectedChunkCount := expectedChunkCountForGroup(docs, chunkCfg)
	if expectedChunkCount <= 0 {
		return false
	}
	if state.IndexedChunkCount != expectedChunkCount {
		return false
	}
	if !hasReadyTranslationsForLanguages(state, requiredLangs) {
		return false
	}
	if sourceFingerprintMatches(docs, state) {
		return true
	}
	return fallbackContentHashesMatch(docs, state)
}

func expectedChunkCountForGroup(docs []Document, cfg ChunkConfig) int {
	if len(docs) == 0 {
		return 0
	}
	cfg = cfg.withDefaults()
	step := cfg.ChunkSizeTokens - int(float64(cfg.ChunkSizeTokens)*cfg.OverlapRatio)
	if step < 1 {
		return 0
	}
	total := 0
	for _, doc := range docs {
		tokenCount := len(strings.Fields(doc.Content))
		if tokenCount == 0 {
			continue
		}
		total += expectedChunkCountForTokens(tokenCount, cfg.ChunkSizeTokens, step)
	}
	return total
}

func expectedChunkCountForTokens(tokenCount int, chunkSize int, step int) int {
	if tokenCount <= 0 || chunkSize <= 0 || step <= 0 {
		return 0
	}
	chunks := 0
	for start := 0; start < tokenCount; start += step {
		end := start + chunkSize
		if end > tokenCount {
			end = tokenCount
		}
		if start >= end {
			break
		}
		chunks++
		if end == tokenCount {
			break
		}
	}
	return chunks
}

func sourceFingerprintMatches(docs []Document, state IndexDocumentState) bool {
	if strings.TrimSpace(state.SourceFingerprint) == "" {
		return false
	}
	if strings.TrimSpace(state.SourceFingerprintConfidence) != sourceFingerprintConfidenceHigh {
		return false
	}
	sourceDoc, ok := pickSourceFromDocs(docs)
	if !ok {
		return false
	}
	sourceFingerprint, confidence := sourceFingerprintForDocument(sourceDoc)
	if confidence != sourceFingerprintConfidenceHigh {
		return false
	}
	return sourceFingerprint == state.SourceFingerprint
}

func fallbackContentHashesMatch(docs []Document, state IndexDocumentState) bool {
	for _, doc := range docs {
		lang := strings.TrimSpace(doc.Language)
		existingTranslation, ok := state.Translations[lang]
		if !ok {
			return false
		}
		expectedHash := documentContentHash(doc)
		if expectedHash == "" || strings.TrimSpace(existingTranslation.ContentHash) == "" {
			return false
		}
		if expectedHash != existingTranslation.ContentHash {
			return false
		}
	}
	return true
}

func hasReadyTranslationsForLanguages(state IndexDocumentState, langs []string) bool {
	for _, lang := range langs {
		translation, ok := state.Translations[lang]
		if !ok {
			return false
		}
		if strings.TrimSpace(translation.Status) != TranslationStatusReady {
			return false
		}
	}
	return true
}

func pickSourceFromDocs(docs []Document) (Document, bool) {
	if len(docs) == 0 {
		return Document{}, false
	}
	byLang := map[string]Document{}
	langs := make([]string, 0, len(docs))
	for _, doc := range docs {
		lang := strings.TrimSpace(doc.Language)
		if lang == "" {
			continue
		}
		if _, exists := byLang[lang]; !exists {
			langs = append(langs, lang)
		}
		byLang[lang] = doc
	}
	if len(byLang) == 0 {
		return docs[0], true
	}
	sourceDoc, ok := pickSourceDocument(byLang, langs)
	if ok {
		return sourceDoc, true
	}
	return docs[0], true
}

func withIndexMetadata(doc Document) Document {
	contentHash := documentContentHash(doc)
	sourceFingerprint, sourceFingerprintConfidence := sourceFingerprintForDocument(doc)
	metadata := cloneMetadataMap(doc.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	if contentHash != "" {
		metadata["index_content_hash"] = contentHash
	}
	if sourceFingerprint != "" {
		metadata["index_source_fingerprint"] = sourceFingerprint
		metadata["index_source_fingerprint_confidence"] = sourceFingerprintConfidence
	}
	doc.Metadata = sanitizeMetadataMap(metadata)

	sourceExtra := cloneMetadataMap(doc.Source.Extra)
	if sourceExtra == nil {
		sourceExtra = map[string]any{}
	}
	if sourceFingerprint != "" {
		sourceExtra["index_source_fingerprint"] = sourceFingerprint
		sourceExtra["index_source_fingerprint_confidence"] = sourceFingerprintConfidence
	}
	doc.Source.Extra = sanitizeMetadataMap(sourceExtra)
	return doc
}

func documentContentHash(doc Document) string {
	lang := strings.TrimSpace(doc.Language)
	title := strings.TrimSpace(doc.Title)
	content := normalizeWhitespace(doc.Content)
	if title == "" && content == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(lang + "|" + title + "|" + content))
	return hex.EncodeToString(sum[:])
}

func sourceFingerprintForDocument(doc Document) (string, string) {
	sourceSystem := strings.TrimSpace(doc.Source.SourceSystem)
	externalID := strings.TrimSpace(doc.Source.ExternalID)
	sourceURI := strings.TrimSpace(doc.Source.SourceURI)
	issuedAt := ""
	if doc.Source.IssuedAt != nil {
		issuedAt = doc.Source.IssuedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	modifiedAt := ""
	if doc.Source.ModifiedAt != nil {
		modifiedAt = doc.Source.ModifiedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	base := strings.Join([]string{
		sourceSystem,
		externalID,
		sourceURI,
		issuedAt,
		modifiedAt,
	}, "|")
	if strings.Trim(base, "|") == "" {
		return "", sourceFingerprintConfidenceLow
	}
	confidence := sourceFingerprintConfidenceLow
	if issuedAt != "" || modifiedAt != "" {
		confidence = sourceFingerprintConfidenceHigh
	}
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:]), confidence
}
