package rag

import (
	"testing"
	"time"
)

func TestFilterDocumentsForIndexSkipsUnchangedGroup(t *testing.T) {
	modifiedAt := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	doc := Document{
		ID:            "doc-1",
		TranslationID: "doc-1:de",
		Language:      "de",
		SourcePath:    "tests/doc-1.json",
		Title:         "Energiegesetz",
		Content:       "Dieses Gesetz betrifft die Energiepolitik.",
		Source: SourceMetadata{
			SourceSystem: "openparldata",
			ExternalID:   "doc-1",
			SourceURI:    "https://example.test/doc-1",
			ModifiedAt:   &modifiedAt,
		},
	}
	prepared := withIndexMetadata(doc)
	contentHash := prepared.Metadata["index_content_hash"].(string)
	sourceFingerprint := prepared.Metadata["index_source_fingerprint"].(string)
	chunkCfg := ChunkConfig{ChunkSizeTokens: 768, OverlapRatio: 0.15}
	expectedChunkCount := expectedChunkCountForGroup([]Document{doc}, chunkCfg)

	state := map[string]IndexDocumentState{
		"doc-1": {
			DocumentID:                  "doc-1",
			SourceFingerprint:           sourceFingerprint,
			SourceFingerprintConfidence: sourceFingerprintConfidenceHigh,
			IndexComplete:               true,
			IndexedChunkCount:           expectedChunkCount,
			Translations: map[string]IndexTranslationState{
				"de": {
					Lang:        "de",
					Status:      TranslationStatusReady,
					ContentHash: contentHash,
				},
			},
		},
	}

	toProcess, report := FilterDocumentsForIndex([]Document{doc}, []string{"de"}, "local", chunkCfg, state)
	if len(toProcess) != 0 {
		t.Fatalf("expected zero documents to process, got %d", len(toProcess))
	}
	if report.SkippedDocuments != 1 || report.ProcessedDocs != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestFilterDocumentsForIndexProcessesWhenHashDiffers(t *testing.T) {
	doc := Document{
		ID:            "doc-2",
		TranslationID: "doc-2:fr",
		Language:      "fr",
		Title:         "Titre",
		Content:       "Contenu mis a jour",
		Source: SourceMetadata{
			SourceSystem: "openparldata",
			ExternalID:   "doc-2",
			SourceURI:    "https://example.test/doc-2",
		},
	}
	state := map[string]IndexDocumentState{
		"doc-2": {
			DocumentID:        "doc-2",
			IndexComplete:     true,
			IndexedChunkCount: 1,
			Translations: map[string]IndexTranslationState{
				"fr": {
					Lang:        "fr",
					Status:      TranslationStatusReady,
					ContentHash: "outdated",
				},
			},
		},
	}

	toProcess, report := FilterDocumentsForIndex([]Document{doc}, []string{"fr"}, "local", ChunkConfig{}, state)
	if len(toProcess) != 1 {
		t.Fatalf("expected one document to process, got %d", len(toProcess))
	}
	if report.SkippedDocuments != 0 || report.ProcessedDocs != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestFilterDocumentsForIndexProcessesWhenIndexIncomplete(t *testing.T) {
	doc := Document{
		ID:            "doc-3",
		TranslationID: "doc-3:fr",
		Language:      "fr",
		Title:         "Titre",
		Content:       "Contenu stable",
		Source: SourceMetadata{
			SourceSystem: "openparldata",
			ExternalID:   "doc-3",
			SourceURI:    "https://example.test/doc-3",
		},
	}
	prepared := withIndexMetadata(doc)
	contentHash := prepared.Metadata["index_content_hash"].(string)
	state := map[string]IndexDocumentState{
		"doc-3": {
			DocumentID:        "doc-3",
			IndexComplete:     false,
			IndexedChunkCount: 1,
			Translations: map[string]IndexTranslationState{
				"fr": {
					Lang:        "fr",
					Status:      TranslationStatusReady,
					ContentHash: contentHash,
				},
			},
		},
	}

	toProcess, report := FilterDocumentsForIndex([]Document{doc}, []string{"fr"}, "local", ChunkConfig{}, state)
	if len(toProcess) != 1 {
		t.Fatalf("expected one document to process, got %d", len(toProcess))
	}
	if report.SkippedDocuments != 0 || report.ProcessedDocs != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestFilterDocumentsForIndexProcessesWhenChunkCountDiffers(t *testing.T) {
	modifiedAt := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	doc := Document{
		ID:            "doc-4",
		TranslationID: "doc-4:de",
		Language:      "de",
		SourcePath:    "tests/doc-4.json",
		Title:         "Finanzen",
		Content:       "Dieses Dokument bleibt unveraendert fuer den Test.",
		Source: SourceMetadata{
			SourceSystem: "openparldata",
			ExternalID:   "doc-4",
			SourceURI:    "https://example.test/doc-4",
			ModifiedAt:   &modifiedAt,
		},
	}
	prepared := withIndexMetadata(doc)
	contentHash := prepared.Metadata["index_content_hash"].(string)
	sourceFingerprint := prepared.Metadata["index_source_fingerprint"].(string)
	chunkCfg := ChunkConfig{ChunkSizeTokens: 768, OverlapRatio: 0.15}
	expectedChunkCount := expectedChunkCountForGroup([]Document{doc}, chunkCfg)
	state := map[string]IndexDocumentState{
		"doc-4": {
			DocumentID:                  "doc-4",
			SourceFingerprint:           sourceFingerprint,
			SourceFingerprintConfidence: sourceFingerprintConfidenceHigh,
			IndexComplete:               true,
			IndexedChunkCount:           expectedChunkCount + 1,
			Translations: map[string]IndexTranslationState{
				"de": {
					Lang:        "de",
					Status:      TranslationStatusReady,
					ContentHash: contentHash,
				},
			},
		},
	}

	toProcess, report := FilterDocumentsForIndex([]Document{doc}, []string{"de"}, "local", chunkCfg, state)
	if len(toProcess) != 1 {
		t.Fatalf("expected one document to process, got %d", len(toProcess))
	}
	if report.SkippedDocuments != 0 || report.ProcessedDocs != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}
