package rag

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeTranslator struct{}

func (f fakeTranslator) Name() string { return "fake" }

func (f fakeTranslator) Translate(_ context.Context, request TranslationRequest) (string, error) {
	return request.TargetLang + ":" + strings.TrimSpace(request.Text), nil
}

type failingTranslator struct{}

func (f failingTranslator) Name() string { return "failing" }

func (f failingTranslator) Translate(_ context.Context, _ TranslationRequest) (string, error) {
	return "", errors.New("translator should not be called")
}

func TestEnsureMissingTranslations(t *testing.T) {
	docs := []Document{
		{
			ID:            "doc-1",
			TranslationID: "doc-1:en",
			Language:      "en",
			Title:         "Energy vote",
			Content:       "This vote affects taxes.",
			Metadata:      map[string]any{"display_title": "Energy vote"},
		},
	}

	got, err := EnsureMissingTranslations(context.Background(), docs, []string{"fr", "en"}, "fr", fakeTranslator{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 translations, got %d", len(got))
	}
	byLang := map[string]Document{}
	for _, item := range got {
		byLang[item.Language] = item
	}
	fr, ok := byLang["fr"]
	if !ok {
		t.Fatalf("missing generated fr translation")
	}
	if fr.TranslationID != "doc-1:fr" {
		t.Fatalf("unexpected translation id: %s", fr.TranslationID)
	}
	if !strings.Contains(fr.Title, "fr:Energy vote") {
		t.Fatalf("unexpected translated title: %s", fr.Title)
	}
	if status, _ := fr.Metadata["translation_status"].(string); status != TranslationStatusReady {
		t.Fatalf("expected ready status, got %v", fr.Metadata["translation_status"])
	}
}

func TestEnsureMissingTranslationsReusesExistingReadyTranslation(t *testing.T) {
	sourceDoc := Document{
		ID:            "doc-2",
		TranslationID: "doc-2:de",
		Language:      "de",
		Title:         "Energie Vorlage",
		Content:       "Das ist ein Testinhalt.",
		Metadata:      map[string]any{},
	}
	sourceDoc = withIndexMetadata(sourceDoc)
	sourceHash := BuildTranslationSourceHash(sourceDoc.Language, sourceDoc.Title, sourceDoc.Content)

	existingState := map[string]IndexDocumentState{
		"doc-2": {
			DocumentID: "doc-2",
			Translations: map[string]IndexTranslationState{
				"rm": {
					Lang:       "rm",
					Title:      "Model da energia",
					Content:    "Quei ei in cuntegn da test.",
					Status:     TranslationStatusReady,
					Provider:   "llm",
					SourceHash: sourceHash,
				},
			},
		},
	}

	got, err := EnsureMissingTranslationsWithOptions(
		context.Background(),
		[]Document{sourceDoc},
		[]string{"de", "rm"},
		"de",
		failingTranslator{},
		EnsureMissingTranslationsOptions{
			ExistingByDocument: existingState,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 translations, got %d", len(got))
	}
	byLang := map[string]Document{}
	for _, item := range got {
		byLang[item.Language] = item
	}
	rmDoc, ok := byLang["rm"]
	if !ok {
		t.Fatalf("missing reused rm translation")
	}
	if rmDoc.Title != "Model da energia" {
		t.Fatalf("unexpected title for reused translation: %s", rmDoc.Title)
	}
	if provider, _ := rmDoc.Metadata["translation_provider"].(string); provider == "" {
		t.Fatalf("expected provider metadata on reused translation")
	}
}

func TestLLMTranslatorTranslateRetriesOnEmptyResponse(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if current == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"content": "",
						},
						"finish_reason": "stop",
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "Texte traduit",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer server.Close()

	translator, err := NewLLMTranslator(LLMTranslatorConfig{
		Enabled:       true,
		BaseURL:       server.URL,
		ModelName:     "test-model",
		Timeout:       2 * time.Second,
		MaxInputChars: 4000,
		MaxRetries:    1,
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	got, err := translator.Translate(context.Background(), TranslationRequest{
		Text:         "Original",
		SourceLang:   "de",
		TargetLang:   "fr",
		ContentLabel: "document content",
	})
	if err != nil {
		t.Fatalf("unexpected translate error: %v", err)
	}
	if got != "Texte traduit" {
		t.Fatalf("unexpected translation: %q", got)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("expected 2 calls, got %d", atomic.LoadInt32(&callCount))
	}
}

func TestLLMTranslatorTranslateFailsAfterRetryExhausted(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer server.Close()

	translator, err := NewLLMTranslator(LLMTranslatorConfig{
		Enabled:       true,
		BaseURL:       server.URL,
		ModelName:     "test-model",
		Timeout:       2 * time.Second,
		MaxInputChars: 4000,
		MaxRetries:    1,
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	_, err = translator.Translate(context.Background(), TranslationRequest{
		Text:         "Original",
		SourceLang:   "de",
		TargetLang:   "fr",
		ContentLabel: "document content",
	})
	if err == nil {
		t.Fatal("expected an error but got nil")
	}
	if !strings.Contains(err.Error(), "translation response is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("expected 2 calls, got %d", atomic.LoadInt32(&callCount))
	}
}
