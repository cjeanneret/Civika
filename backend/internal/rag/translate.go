package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"civika/backend/internal/debuglog"
	"civika/backend/internal/langs"
)

const (
	TranslationStatusReady   = "ready"
	TranslationStatusPending = "pending"
	TranslationStatusFailed  = "failed"
)

type TranslationRequest struct {
	Text         string
	SourceLang   string
	TargetLang   string
	ContentLabel string
}

type Translator interface {
	Name() string
	Translate(ctx context.Context, request TranslationRequest) (string, error)
}

type DisabledTranslator struct{}

func NewDisabledTranslator() *DisabledTranslator {
	return &DisabledTranslator{}
}

func (t *DisabledTranslator) Name() string {
	return "disabled"
}

func (t *DisabledTranslator) Translate(_ context.Context, _ TranslationRequest) (string, error) {
	return "", errors.New("translation is disabled in local mode")
}

type LLMTranslatorConfig struct {
	Enabled         bool
	BaseURL         string
	APIKey          string
	ModelName       string
	Timeout         time.Duration
	MaxInputChars   int
	MaxRetries      int
	MaxOutputTokens int
}

type LLMTranslator struct {
	cfg    LLMTranslatorConfig
	client *http.Client
}

func NewLLMTranslator(cfg LLMTranslatorConfig) (*LLMTranslator, error) {
	if !cfg.Enabled {
		return nil, errors.New("llm translator is disabled")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return nil, errors.New("llm translator requires base url and model name")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxInputChars <= 0 {
		cfg.MaxInputChars = 12000
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.MaxOutputTokens <= 0 {
		cfg.MaxOutputTokens = 800
	}
	return &LLMTranslator{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (t *LLMTranslator) Name() string {
	return "llm"
}

func (t *LLMTranslator) Translate(ctx context.Context, request TranslationRequest) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= t.cfg.MaxRetries; attempt++ {
		translated, retryable, err := t.translateOnce(ctx, request)
		if err == nil {
			return translated, nil
		}
		lastErr = err
		if !retryable {
			return "", err
		}
		if ctx.Err() != nil {
			return "", err
		}
		if attempt == t.cfg.MaxRetries {
			// #region agent log
			writeDebugNDJSONLog(ctx, "H6", "backend/internal/rag/translate.go:112", "translation_retry_exhausted", map[string]any{
				"source_lang":   langs.Normalize(request.SourceLang),
				"target_lang":   langs.Normalize(request.TargetLang),
				"content_label": request.ContentLabel,
				"attempt":       attempt + 1,
				"max_retries":   t.cfg.MaxRetries,
				"error":         err.Error(),
			})
			// #endregion
			return "", err
		}
		// #region agent log
		writeDebugNDJSONLog(ctx, "H6", "backend/internal/rag/translate.go:126", "translation_retry_scheduled", map[string]any{
			"source_lang":   langs.Normalize(request.SourceLang),
			"target_lang":   langs.Normalize(request.TargetLang),
			"content_label": request.ContentLabel,
			"attempt":       attempt + 1,
			"next_attempt":  attempt + 2,
			"max_retries":   t.cfg.MaxRetries,
			"error":         err.Error(),
		})
		// #endregion
	}
	if lastErr == nil {
		lastErr = errors.New("translation failed without explicit error")
	}
	return "", lastErr
}

func (t *LLMTranslator) translateOnce(ctx context.Context, request TranslationRequest) (string, bool, error) {
	text := strings.TrimSpace(request.Text)
	if text == "" {
		return "", false, errors.New("translation text is required")
	}
	sourceLang := langs.Normalize(request.SourceLang)
	targetLang := langs.Normalize(request.TargetLang)
	if sourceLang == "" || targetLang == "" {
		return "", false, errors.New("translation languages are invalid")
	}
	if sourceLang == targetLang {
		return text, false, nil
	}
	if len(text) > t.cfg.MaxInputChars {
		text = text[:t.cfg.MaxInputChars]
	}
	deadline, hasDeadline := ctx.Deadline()
	ctxRemainingMs := int64(-1)
	if hasDeadline {
		ctxRemainingMs = time.Until(deadline).Milliseconds()
	}
	// #region agent log
	writeDebugNDJSONLog(ctx, "H2", "backend/internal/rag/translate.go:118", "translation_http_request_start", map[string]any{
		"source_lang":      sourceLang,
		"target_lang":      targetLang,
		"content_label":    request.ContentLabel,
		"text_len":         len(text),
		"max_input_chars":  t.cfg.MaxInputChars,
		"client_timeoutMs": t.cfg.Timeout.Milliseconds(),
		"ctx_has_deadline": hasDeadline,
		"ctx_remaining_ms": ctxRemainingMs,
		"base_url":         t.cfg.BaseURL,
		"model_name":       t.cfg.ModelName,
	})
	// #endregion
	prompt := buildTranslationPrompt(text, sourceLang, targetLang, request.ContentLabel)
	payload := map[string]any{
		"model":      t.cfg.ModelName,
		"max_tokens": t.cfg.MaxOutputTokens,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Translate public Swiss political content. Return only translated text, no notes.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", false, fmt.Errorf("marshal translation payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(t.cfg.BaseURL, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("create translation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.cfg.APIKey)
	}

	requestStarted := time.Now()
	res, err := t.client.Do(req)
	if err != nil {
		// #region agent log
		writeDebugNDJSONLog(ctx, "H2", "backend/internal/rag/translate.go:148", "translation_http_request_error", map[string]any{
			"source_lang":      sourceLang,
			"target_lang":      targetLang,
			"content_label":    request.ContentLabel,
			"duration_ms":      time.Since(requestStarted).Milliseconds(),
			"error":            err.Error(),
			"ctx_err":          contextErrorString(ctx),
			"client_timeoutMs": t.cfg.Timeout.Milliseconds(),
		})
		// #endregion
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "translation",
			ProviderName: "llm",
			ModelName:    t.cfg.ModelName,
			SourceLang:   sourceLang,
			TargetLang:   targetLang,
			InputChars:   len(text),
			OutputChars:  0,
			UsageSource:  "unknown",
			Status:       "error",
			DurationMS:   time.Since(requestStarted).Milliseconds(),
			ErrorCode:    "request_failed",
		})
		return "", false, fmt.Errorf("translation request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// #region agent log
		writeDebugNDJSONLog(ctx, "H4", "backend/internal/rag/translate.go:166", "translation_http_bad_status", map[string]any{
			"source_lang":   sourceLang,
			"target_lang":   targetLang,
			"content_label": request.ContentLabel,
			"duration_ms":   time.Since(requestStarted).Milliseconds(),
			"status_code":   res.StatusCode,
		})
		// #endregion
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "translation",
			ProviderName: "llm",
			ModelName:    t.cfg.ModelName,
			SourceLang:   sourceLang,
			TargetLang:   targetLang,
			InputChars:   len(text),
			OutputChars:  0,
			UsageSource:  "unknown",
			Status:       "error",
			DurationMS:   time.Since(requestStarted).Milliseconds(),
			ErrorCode:    fmt.Sprintf("status_%d", res.StatusCode),
		})
		return "", false, fmt.Errorf("translation request returned status %d", res.StatusCode)
	}

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024))
	if err != nil {
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "translation",
			ProviderName: "llm",
			ModelName:    t.cfg.ModelName,
			SourceLang:   sourceLang,
			TargetLang:   targetLang,
			InputChars:   len(text),
			OutputChars:  0,
			UsageSource:  "unknown",
			Status:       "error",
			DurationMS:   time.Since(requestStarted).Milliseconds(),
			ErrorCode:    "read_failed",
		})
		return "", false, fmt.Errorf("read translation response: %w", err)
	}
	usage, usageErr := ParseUsageFromResponseBody(responseBody)
	if usageErr != nil {
		usage = UsageEvent{UsageSource: "unknown"}
	}
	var response struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "translation",
			ProviderName: "llm",
			ModelName:    t.cfg.ModelName,
			SourceLang:   sourceLang,
			TargetLang:   targetLang,
			InputChars:   len(text),
			OutputChars:  0,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			UsageSource:  usage.UsageSource,
			Status:       "error",
			DurationMS:   time.Since(requestStarted).Milliseconds(),
			ErrorCode:    "decode_failed",
		})
		return "", false, fmt.Errorf("decode translation response: %w", err)
	}
	firstChoiceMessageLen := 0
	firstChoiceTextLen := 0
	firstChoiceFinishReason := ""
	if len(response.Choices) > 0 {
		firstChoiceMessageLen = len(strings.TrimSpace(response.Choices[0].Message.Content))
		firstChoiceTextLen = len(strings.TrimSpace(response.Choices[0].Text))
		firstChoiceFinishReason = strings.TrimSpace(response.Choices[0].FinishReason)
	}
	// #region agent log
	writeDebugNDJSONLog(ctx, "H3", "backend/internal/rag/translate.go:214", "translation_http_response_shape", map[string]any{
		"source_lang":                sourceLang,
		"target_lang":                targetLang,
		"content_label":              request.ContentLabel,
		"duration_ms":                time.Since(requestStarted).Milliseconds(),
		"response_bytes":             len(responseBody),
		"choices_count":              len(response.Choices),
		"first_choice_message_len":   firstChoiceMessageLen,
		"first_choice_text_len":      firstChoiceTextLen,
		"first_choice_finish_reason": firstChoiceFinishReason,
	})
	// #endregion
	if len(response.Choices) == 0 {
		// #region agent log
		writeDebugNDJSONLog(ctx, "H3", "backend/internal/rag/translate.go:227", "translation_http_response_no_choices", map[string]any{
			"source_lang":   sourceLang,
			"target_lang":   targetLang,
			"content_label": request.ContentLabel,
			"duration_ms":   time.Since(requestStarted).Milliseconds(),
		})
		// #endregion
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "translation",
			ProviderName: "llm",
			ModelName:    t.cfg.ModelName,
			SourceLang:   sourceLang,
			TargetLang:   targetLang,
			InputChars:   len(text),
			OutputChars:  0,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			UsageSource:  usage.UsageSource,
			Status:       "error",
			DurationMS:   time.Since(requestStarted).Milliseconds(),
			ErrorCode:    "no_choices",
		})
		return "", true, errors.New("translation response has no choices")
	}
	translated := strings.TrimSpace(response.Choices[0].Message.Content)
	if translated == "" {
		translated = strings.TrimSpace(response.Choices[0].Text)
	}
	if translated == "" {
		// #region agent log
		writeDebugNDJSONLog(ctx, "H3", "backend/internal/rag/translate.go:246", "translation_http_response_empty_text", map[string]any{
			"source_lang":                sourceLang,
			"target_lang":                targetLang,
			"content_label":              request.ContentLabel,
			"duration_ms":                time.Since(requestStarted).Milliseconds(),
			"choices_count":              len(response.Choices),
			"first_choice_finish_reason": firstChoiceFinishReason,
		})
		// #endregion
		emitUsageEvent(ctx, UsageEvent{
			Operation:    "translation",
			ProviderName: "llm",
			ModelName:    t.cfg.ModelName,
			SourceLang:   sourceLang,
			TargetLang:   targetLang,
			InputChars:   len(text),
			OutputChars:  0,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			UsageSource:  usage.UsageSource,
			Status:       "error",
			DurationMS:   time.Since(requestStarted).Milliseconds(),
			ErrorCode:    "empty_text",
		})
		return "", true, errors.New("translation response is empty")
	}
	// #region agent log
	writeDebugNDJSONLog(ctx, "H5", "backend/internal/rag/translate.go:203", "translation_http_request_success", map[string]any{
		"source_lang":   sourceLang,
		"target_lang":   targetLang,
		"content_label": request.ContentLabel,
		"duration_ms":   time.Since(requestStarted).Milliseconds(),
		"output_len":    len(translated),
	})
	// #endregion
	emitUsageEvent(ctx, UsageEvent{
		Operation:    "translation",
		ProviderName: "llm",
		ModelName:    t.cfg.ModelName,
		SourceLang:   sourceLang,
		TargetLang:   targetLang,
		InputChars:   len(text),
		OutputChars:  len(translated),
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		UsageSource:  usage.UsageSource,
		Status:       "success",
		DurationMS:   time.Since(requestStarted).Milliseconds(),
	})
	return translated, false, nil
}

func buildTranslationPrompt(text, sourceLang, targetLang, contentLabel string) string {
	label := strings.TrimSpace(contentLabel)
	if label == "" {
		label = "content"
	}
	var b strings.Builder
	b.WriteString("Translate the following ")
	b.WriteString(label)
	b.WriteString(" from language code ")
	b.WriteString(sourceLang)
	b.WriteString(" to language code ")
	b.WriteString(targetLang)
	b.WriteString(". Keep factual meaning, names and numbers unchanged. Return only the translation.\n\n")
	b.WriteString(text)
	return b.String()
}

type EnsureMissingTranslationsOptions struct {
	ExistingByDocument map[string]IndexDocumentState
}

func EnsureMissingTranslations(ctx context.Context, documents []Document, supportedLanguages []string, defaultLang string, translator Translator) ([]Document, error) {
	return EnsureMissingTranslationsWithOptions(ctx, documents, supportedLanguages, defaultLang, translator, EnsureMissingTranslationsOptions{})
}

func EnsureMissingTranslationsWithOptions(
	ctx context.Context,
	documents []Document,
	supportedLanguages []string,
	defaultLang string,
	translator Translator,
	options EnsureMissingTranslationsOptions,
) ([]Document, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	if len(supportedLanguages) == 0 {
		return nil, errors.New("supported languages are required")
	}
	if translator == nil {
		return nil, errors.New("translator is required")
	}

	defaultLang = langs.NormalizeOrFallback(defaultLang, "fr", supportedLanguages)
	grouped := map[string][]Document{}
	for _, doc := range documents {
		grouped[doc.ID] = append(grouped[doc.ID], doc)
	}
	// #region agent log
	writeDebugNDJSONLog(ctx, "H1", "backend/internal/rag/translate.go:248", "ensure_missing_translations_start", map[string]any{
		"documents_total":        len(documents),
		"documents_grouped":      len(grouped),
		"supported_languages":    supportedLanguages,
		"supported_count":        len(supportedLanguages),
		"default_lang":           defaultLang,
		"translator":             translator.Name(),
		"ctx_has_deadline":       hasContextDeadline(ctx),
		"ctx_remaining_at_start": contextRemainingMs(ctx),
	})
	// #endregion

	out := make([]Document, 0, len(documents))
	for _, docs := range grouped {
		byLang := map[string]Document{}
		for _, doc := range docs {
			language := langs.NormalizeOrFallback(doc.Language, defaultLang, supportedLanguages)
			doc.Language = language
			doc.TranslationID = buildTranslationID(doc.ID, language)
			byLang[language] = doc
		}
		sourceDoc, ok := pickSourceDocument(byLang, supportedLanguages)
		if !ok {
			return nil, fmt.Errorf("no source translation found for document %s", docs[0].ID)
		}
		sourceDoc = withIndexMetadata(sourceDoc)
		translationSourceHash := BuildTranslationSourceHash(sourceDoc.Language, sourceDoc.Title, sourceDoc.Content)
		existingState := options.ExistingByDocument[sourceDoc.ID]
		for _, langCode := range supportedLanguages {
			if existing, exists := byLang[langCode]; exists {
				metadata := cloneMetadataMap(existing.Metadata)
				metadata["translation_status"] = TranslationStatusReady
				metadata["translation_provider"] = "source"
				metadata["translation_source_lang"] = existing.Language
				metadata["translation_source_hash"] = BuildTranslationSourceHash(existing.Language, existing.Title, existing.Content)
				metadata["translation_updated_at"] = time.Now().UTC().Format(time.RFC3339)
				existing.Metadata = sanitizeMetadataMap(metadata)
				out = append(out, withIndexMetadata(existing))
				continue
			}
			if reused, reusedOK := reuseExistingReadyTranslation(sourceDoc, langCode, existingState, translationSourceHash); reusedOK {
				out = append(out, reused)
				continue
			}
			translated, err := translateDocument(ctx, sourceDoc, langCode, translator)
			if err != nil {
				// #region agent log
				writeDebugNDJSONLog(ctx, "H1", "backend/internal/rag/translate.go:283", "ensure_missing_translations_failed", map[string]any{
					"document_id":       sourceDoc.ID,
					"source_lang":       sourceDoc.Language,
					"target_lang":       langCode,
					"error":             err.Error(),
					"ctx_err":           contextErrorString(ctx),
					"ctx_remaining_ms":  contextRemainingMs(ctx),
					"translations_done": len(out),
				})
				// #endregion
				return nil, fmt.Errorf("translate %s to %s: %w", sourceDoc.ID, langCode, err)
			}
			out = append(out, withIndexMetadata(translated))
		}
	}
	return out, nil
}

func reuseExistingReadyTranslation(sourceDoc Document, targetLang string, state IndexDocumentState, sourceHash string) (Document, bool) {
	targetLang = langs.Normalize(targetLang)
	if targetLang == "" {
		return Document{}, false
	}
	existing, ok := state.Translations[targetLang]
	if !ok {
		return Document{}, false
	}
	if existing.Status != TranslationStatusReady {
		return Document{}, false
	}
	if strings.TrimSpace(existing.Content) == "" {
		return Document{}, false
	}
	if strings.TrimSpace(existing.SourceHash) == "" || existing.SourceHash != sourceHash {
		return Document{}, false
	}

	metadata := cloneMetadataMap(sourceDoc.Metadata)
	metadata["display_title"] = normalizeNonEmpty(existing.Title, sourceDoc.Title)
	metadata["translation_status"] = TranslationStatusReady
	metadata["translation_provider"] = normalizeNonEmpty(existing.Provider, "existing_db")
	metadata["translation_source_lang"] = sourceDoc.Language
	metadata["translation_source_hash"] = sourceHash
	metadata["translation_updated_at"] = time.Now().UTC().Format(time.RFC3339)

	reused := Document{
		ID:            sourceDoc.ID,
		TranslationID: buildTranslationID(sourceDoc.ID, targetLang),
		Language:      targetLang,
		SourcePath:    sourceDoc.SourcePath,
		Title:         normalizeNonEmpty(existing.Title, sourceDoc.Title),
		Content:       normalizeWhitespace(existing.Content),
		ContentType:   sourceDoc.ContentType,
		Summary:       sourceDoc.Summary,
		Source:        sourceDoc.Source,
		Intervenants:  sourceDoc.Intervenants,
		Metadata:      sanitizeMetadataMap(metadata),
	}
	return withIndexMetadata(reused), true
}

func hasContextDeadline(ctx context.Context) bool {
	_, ok := ctx.Deadline()
	return ok
}

func contextRemainingMs(ctx context.Context) int64 {
	deadline, ok := ctx.Deadline()
	if !ok {
		return -1
	}
	return time.Until(deadline).Milliseconds()
}

func contextErrorString(ctx context.Context) string {
	if err := ctx.Err(); err != nil {
		return err.Error()
	}
	return ""
}

func writeDebugNDJSONLog(ctx context.Context, hypothesisID, location, message string, data map[string]any) {
	debuglog.Log(ctx, hypothesisID, location, message, data)
}

func pickSourceDocument(byLang map[string]Document, supported []string) (Document, bool) {
	preferred := append([]string{"en", "fr", "de", "it", "rm"}, supported...)
	seen := map[string]struct{}{}
	for _, langCode := range preferred {
		if _, exists := seen[langCode]; exists {
			continue
		}
		seen[langCode] = struct{}{}
		if doc, ok := byLang[langCode]; ok {
			return doc, true
		}
	}
	for _, doc := range byLang {
		return doc, true
	}
	return Document{}, false
}

func translateDocument(ctx context.Context, source Document, targetLang string, translator Translator) (Document, error) {
	targetLang = langs.Normalize(targetLang)
	if targetLang == "" {
		return Document{}, errors.New("target language is invalid")
	}
	sourceLang := langs.Normalize(source.Language)
	if sourceLang == "" {
		sourceLang = "en"
	}
	ctx = WithUsageDocumentID(ctx, source.ID)
	translationTask := startTranslateTask("translate_document", map[string]any{
		"document_id": source.ID,
		"source_path": source.SourcePath,
		"provider":    translator.Name(),
		"source_lang": sourceLang,
		"target_lang": targetLang,
	})

	titleStarted := time.Now()
	translatedTitle, err := translator.Translate(ctx, TranslationRequest{
		Text:         source.Title,
		SourceLang:   sourceLang,
		TargetLang:   targetLang,
		ContentLabel: "title",
	})
	if err != nil {
		translationTask.Fail(err, map[string]any{
			"stage":        "title",
			"title_ms":     time.Since(titleStarted).Milliseconds(),
			"content_ms":   0,
			"document_id":  source.ID,
			"target_lang":  targetLang,
			"source_lang":  sourceLang,
			"source_path":  source.SourcePath,
			"provider":     translator.Name(),
			"stage_status": "failed",
		})
		return Document{}, err
	}
	contentStarted := time.Now()
	translatedContent, err := translator.Translate(ctx, TranslationRequest{
		Text:         source.Content,
		SourceLang:   sourceLang,
		TargetLang:   targetLang,
		ContentLabel: "document content",
	})
	if err != nil {
		translationTask.Fail(err, map[string]any{
			"stage":        "content",
			"title_ms":     time.Since(titleStarted).Milliseconds(),
			"content_ms":   time.Since(contentStarted).Milliseconds(),
			"document_id":  source.ID,
			"target_lang":  targetLang,
			"source_lang":  sourceLang,
			"source_path":  source.SourcePath,
			"provider":     translator.Name(),
			"stage_status": "failed",
		})
		return Document{}, err
	}

	metadata := cloneMetadataMap(source.Metadata)
	metadata["display_title"] = translatedTitle
	metadata["translation_status"] = TranslationStatusReady
	metadata["translation_provider"] = translator.Name()
	metadata["translation_source_lang"] = sourceLang
	metadata["translation_source_hash"] = BuildTranslationSourceHash(sourceLang, source.Title, source.Content)
	metadata["translation_updated_at"] = time.Now().UTC().Format(time.RFC3339)
	translationTask.Done(map[string]any{
		"title_ms":   time.Since(titleStarted).Milliseconds(),
		"content_ms": time.Since(contentStarted).Milliseconds(),
		"status":     TranslationStatusReady,
	})

	return withIndexMetadata(Document{
		ID:            source.ID,
		TranslationID: buildTranslationID(source.ID, targetLang),
		Language:      targetLang,
		SourcePath:    source.SourcePath,
		Title:         translatedTitle,
		Content:       normalizeWhitespace(translatedContent),
		ContentType:   source.ContentType,
		Summary:       source.Summary,
		Source:        source.Source,
		Intervenants:  source.Intervenants,
		Metadata:      sanitizeMetadataMap(metadata),
	}), nil
}

type translateTaskScope struct {
	taskName  string
	startedAt time.Time
}

func startTranslateTask(taskName string, fields map[string]any) translateTaskScope {
	scope := translateTaskScope{
		taskName:  taskName,
		startedAt: time.Now(),
	}
	logTranslateTaskEvent("start", taskName, fields)
	return scope
}

func (s translateTaskScope) Done(fields map[string]any) {
	withDuration := mergeTranslateTaskFields(fields, map[string]any{
		"duration_ms": time.Since(s.startedAt).Milliseconds(),
	})
	logTranslateTaskEvent("done", s.taskName, withDuration)
}

func (s translateTaskScope) Fail(err error, fields map[string]any) {
	withFailure := mergeTranslateTaskFields(fields, map[string]any{
		"duration_ms": time.Since(s.startedAt).Milliseconds(),
		"error":       err,
	})
	logTranslateTaskEvent("failed", s.taskName, withFailure)
}

func logTranslateTaskEvent(taskStatus string, taskName string, fields map[string]any) {
	log.Printf("rag translate task=%s task_name=%s%s", taskStatus, taskName, formatTranslateTaskFields(fields))
}

func formatTranslateTaskFields(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		value := fields[key]
		if value == nil {
			continue
		}
		b.WriteString(" ")
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(strings.ReplaceAll(fmt.Sprint(value), " ", "_"))
	}
	return b.String()
}

func mergeTranslateTaskFields(base map[string]any, additional map[string]any) map[string]any {
	if len(base) == 0 && len(additional) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range additional {
		out[key] = value
	}
	return out
}
