package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"civika/backend/internal/langs"
	"civika/backend/internal/rag"
)

var ErrNotFound = errors.New("resource not found")

type SQLQueryService struct {
	db                 *sql.DB
	translator         rag.Translator
	supportedLanguages []string
	defaultLanguage    string
	fallbackLanguage   string
	mu                 sync.Mutex
	inFlight           map[string]struct{}
}

func NewSQLQueryService(db *sql.DB) *SQLQueryService {
	return &SQLQueryService{
		db:                 db,
		supportedLanguages: []string{"fr", "de", "it", "rm", "en"},
		defaultLanguage:    "fr",
		fallbackLanguage:   "en",
		inFlight:           map[string]struct{}{},
	}
}

type TranslationRuntimeConfig struct {
	Translator         rag.Translator
	SupportedLanguages []string
	DefaultLanguage    string
	FallbackLanguage   string
}

func NewSQLQueryServiceWithTranslations(db *sql.DB, cfg TranslationRuntimeConfig) *SQLQueryService {
	service := NewSQLQueryService(db)
	service.translator = cfg.Translator
	supported := langs.StableSorted(cfg.SupportedLanguages)
	if len(supported) > 0 {
		service.supportedLanguages = supported
	}
	service.defaultLanguage = normalizeLanguageWithFallback(cfg.DefaultLanguage, service.defaultLanguage, service.supportedLanguages)
	service.fallbackLanguage = normalizeLanguageWithFallback(cfg.FallbackLanguage, service.fallbackLanguage, service.supportedLanguages)
	return service
}

func (s *SQLQueryService) ListVotations(ctx context.Context, filters VotationFilters) (VotationListResult, error) {
	if s.db == nil {
		return VotationListResult{}, errors.New("db is required")
	}
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	args := []any{}
	where := []string{"d.votation_id <> ''"}
	if filters.DateFrom != nil {
		where = append(where, fmt.Sprintf("d.vote_date >= $%d", len(args)+1))
		args = append(args, *filters.DateFrom)
	}
	if filters.DateTo != nil {
		where = append(where, fmt.Sprintf("d.vote_date <= $%d", len(args)+1))
		args = append(args, *filters.DateTo)
	}
	if filters.Level != "" {
		where = append(where, fmt.Sprintf("d.level = $%d", len(args)+1))
		args = append(args, filters.Level)
	}
	if filters.Canton != "" {
		where = append(where, fmt.Sprintf("d.canton = $%d", len(args)+1))
		args = append(args, filters.Canton)
	}
	if filters.Status != "" {
		where = append(where, fmt.Sprintf("d.status = $%d", len(args)+1))
		args = append(args, filters.Status)
	}

	countQuery := "SELECT COUNT(DISTINCT d.votation_id) FROM documents d WHERE " + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return VotationListResult{}, fmt.Errorf("count votations: %w", err)
	}

	dataQuery := `
SELECT
	d.id,
	d.votation_id,
	COALESCE(d.vote_date, NOW()) AS vote_date,
	d.level,
	d.canton,
	d.commune_code,
	d.commune_name,
	d.status,
	d.object_id,
	d.source_uri,
	dt.lang,
	dt.title,
	COALESCE(NULLIF(dt.metadata->>'display_title', ''), dt.title) AS display_title
FROM documents d
JOIN document_translations dt ON dt.document_id = d.id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY d.vote_date DESC, d.votation_id ASC, dt.lang ASC
LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return VotationListResult{}, fmt.Errorf("list votations: %w", err)
	}
	defer rows.Close()

	type aggregate struct {
		Item        VotationListItem
		DocumentSet map[string]struct{}
		ObjectSet   map[string]struct{}
		SourceSet   map[string]struct{}
	}
	byVotation := map[string]*aggregate{}
	orderedIDs := make([]string, 0, filters.Limit)

	for rows.Next() {
		var (
			documentID   string
			votationID   string
			date         time.Time
			level        string
			canton       string
			communeCode  string
			communeName  string
			status       string
			objectID     string
			sourceURI    string
			lang         string
			title        string
			displayTitle string
		)
		if err := rows.Scan(&documentID, &votationID, &date, &level, &canton, &communeCode, &communeName, &status, &objectID, &sourceURI, &lang, &title, &displayTitle); err != nil {
			return VotationListResult{}, fmt.Errorf("scan votation row: %w", err)
		}
		item, ok := byVotation[votationID]
		if !ok {
			item = &aggregate{
				Item: VotationListItem{
					ID:            votationID,
					DateISO:       date.UTC().Format(time.RFC3339),
					Level:         level,
					Canton:        canton,
					CommuneCode:   communeCode,
					CommuneName:   communeName,
					Status:        status,
					Language:      filters.Lang,
					Titles:        map[string]string{},
					DisplayTitles: map[string]string{},
					ObjectIDs:     []string{},
					SourceURLs:    []string{},
				},
				DocumentSet: map[string]struct{}{},
				ObjectSet:   map[string]struct{}{},
				SourceSet:   map[string]struct{}{},
			}
			byVotation[votationID] = item
			orderedIDs = append(orderedIDs, votationID)
		}
		if lang != "" && title != "" {
			item.Item.Titles[lang] = title
		}
		if lang != "" && displayTitle != "" {
			item.Item.DisplayTitles[lang] = displayTitle
		}
		if documentID != "" {
			item.DocumentSet[documentID] = struct{}{}
		}
		if objectID != "" {
			if _, exists := item.ObjectSet[objectID]; !exists {
				item.ObjectSet[objectID] = struct{}{}
				item.Item.ObjectIDs = append(item.Item.ObjectIDs, objectID)
			}
		}
		if sourceURI != "" {
			if _, exists := item.SourceSet[sourceURI]; !exists {
				item.SourceSet[sourceURI] = struct{}{}
				item.Item.SourceURLs = append(item.Item.SourceURLs, sourceURI)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return VotationListResult{}, fmt.Errorf("iterate votation rows: %w", err)
	}

	items := make([]VotationListItem, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if row, ok := byVotation[id]; ok {
			docIDs := make([]string, 0, len(row.DocumentSet))
			for docID := range row.DocumentSet {
				docIDs = append(docIDs, docID)
			}
			sort.Strings(docIDs)
			s.applyTranslationFallback(&row.Item, filters.Lang, docIDs)
			sort.Strings(row.Item.ObjectIDs)
			sort.Strings(row.Item.SourceURLs)
			items = append(items, row.Item)
		}
	}

	return VotationListResult{
		Items:  items,
		Limit:  filters.Limit,
		Offset: filters.Offset,
		Total:  total,
	}, nil
}

func (s *SQLQueryService) GetVotationByID(ctx context.Context, votationID string, preferredLang string) (VotationDetail, error) {
	if s.db == nil {
		return VotationDetail{}, errors.New("db is required")
	}
	if preferredLang == "" {
		preferredLang = "fr"
	}
	query := `
SELECT
	d.id,
	d.votation_id,
	COALESCE(d.vote_date, NOW()) AS vote_date,
	d.level,
	d.canton,
	d.commune_code,
	d.commune_name,
	d.status,
	d.object_id,
	d.source_uri,
	dt.lang,
	dt.title,
	COALESCE(NULLIF(dt.metadata->>'display_title', ''), dt.title) AS display_title
FROM documents d
JOIN document_translations dt ON dt.document_id = d.id
WHERE d.votation_id = $1
ORDER BY CASE WHEN dt.lang = $2 THEN 0 ELSE 1 END, dt.lang ASC`
	rows, err := s.db.QueryContext(ctx, query, votationID, preferredLang)
	if err != nil {
		return VotationDetail{}, fmt.Errorf("get votation: %w", err)
	}
	defer rows.Close()

	var (
		documentID   string
		out          VotationDetail
		found        bool
		objectID     string
		source       string
		lang         string
		title        string
		date         time.Time
		level        string
		canton       string
		communeCode  string
		communeName  string
		status       string
		displayTitle string
	)
	objectSet := map[string]struct{}{}
	sourceSet := map[string]struct{}{}
	documentSet := map[string]struct{}{}
	titles := map[string]string{}
	displayTitles := map[string]string{}
	selectedLang := ""

	for rows.Next() {
		var id string
		if err := rows.Scan(&documentID, &id, &date, &level, &canton, &communeCode, &communeName, &status, &objectID, &source, &lang, &title, &displayTitle); err != nil {
			return VotationDetail{}, fmt.Errorf("scan votation detail: %w", err)
		}
		if !found {
			out.ID = id
			out.DateISO = date.UTC().Format(time.RFC3339)
			out.Level = level
			out.Canton = canton
			out.CommuneCode = communeCode
			out.CommuneName = communeName
			out.Status = status
			selectedLang = lang
			found = true
		}
		if lang != "" && title != "" {
			titles[lang] = title
		}
		if lang != "" && displayTitle != "" {
			displayTitles[lang] = displayTitle
		}
		if documentID != "" {
			documentSet[documentID] = struct{}{}
		}
		if objectID != "" {
			objectSet[objectID] = struct{}{}
		}
		if source != "" {
			sourceSet[source] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return VotationDetail{}, fmt.Errorf("iterate votation detail: %w", err)
	}
	if !found {
		return VotationDetail{}, ErrNotFound
	}
	out.Language = normalizeLanguageWithFallback(preferredLang, selectedLang, s.supportedLanguages)
	out.Titles = titles
	out.DisplayTitles = displayTitles
	for id := range objectSet {
		out.ObjectIDs = append(out.ObjectIDs, id)
	}
	for uri := range sourceSet {
		out.SourceURLs = append(out.SourceURLs, uri)
	}
	sort.Strings(out.ObjectIDs)
	sort.Strings(out.SourceURLs)
	docIDs := make([]string, 0, len(documentSet))
	for docID := range documentSet {
		docIDs = append(docIDs, docID)
	}
	sort.Strings(docIDs)
	s.applyTranslationFallbackDetail(&out, preferredLang, docIDs)
	return out, nil
}

func (s *SQLQueryService) ListObjectsByVotation(ctx context.Context, votationID string, lang string) ([]ObjectSummary, error) {
	if s.db == nil {
		return nil, errors.New("db is required")
	}
	if lang == "" {
		lang = "fr"
	}
	query := `
SELECT DISTINCT ON (d.object_id)
	d.object_id,
	COALESCE(dt.title, '') AS title,
	d.object_type,
	d.object_theme,
	d.status
FROM documents d
LEFT JOIN document_translations dt ON dt.document_id = d.id AND dt.lang = $2
WHERE d.votation_id = $1 AND d.object_id <> ''
ORDER BY d.object_id`
	rows, err := s.db.QueryContext(ctx, query, votationID, lang)
	if err != nil {
		return nil, fmt.Errorf("list objects by votation: %w", err)
	}
	defer rows.Close()

	out := make([]ObjectSummary, 0)
	for rows.Next() {
		var item ObjectSummary
		if err := rows.Scan(&item.ID, &item.Title, &item.Type, &item.Theme, &item.Status); err != nil {
			return nil, fmt.Errorf("scan object summary: %w", err)
		}
		item.Slug = slugify(item.ID)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate object summary rows: %w", err)
	}
	return out, nil
}

func (s *SQLQueryService) GetObjectByID(ctx context.Context, objectID string, lang string) (ObjectDetail, error) {
	if s.db == nil {
		return ObjectDetail{}, errors.New("db is required")
	}
	if lang == "" {
		lang = "fr"
	}
	query := `
SELECT
	d.object_id,
	d.votation_id,
	d.object_type,
	d.object_theme,
	d.source_system,
	dt.lang,
	dt.title,
	dt.content_normalized,
	d.source_metadata
FROM documents d
JOIN document_translations dt ON dt.document_id = d.id
WHERE d.object_id = $1
ORDER BY CASE WHEN dt.lang = $2 THEN 0 ELSE 1 END, dt.lang ASC
LIMIT 1`
	var (
		out          ObjectDetail
		selectedLang string
		content      string
		rawMeta      []byte
		sourceSystem string
	)
	err := s.db.QueryRowContext(ctx, query, objectID, lang).Scan(
		&out.ID,
		&out.VotationID,
		&out.Type,
		&out.Theme,
		&sourceSystem,
		&selectedLang,
		&out.Title,
		&content,
		&rawMeta,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ObjectDetail{}, ErrNotFound
	}
	if err != nil {
		return ObjectDetail{}, fmt.Errorf("get object detail: %w", err)
	}

	out.Language = selectedLang
	out.Section = map[string]string{
		"context":   content,
		"arguments": extractArguments(rawMeta),
	}
	if out.Type != "" {
		out.Tags = append(out.Tags, out.Type)
	}
	if out.Theme != "" {
		out.Tags = append(out.Tags, out.Theme)
	}
	out.SourceSystems = []string{sourceSystem}
	return out, nil
}

func (s *SQLQueryService) ListObjectSources(ctx context.Context, objectID string) ([]ObjectSource, error) {
	if s.db == nil {
		return nil, errors.New("db is required")
	}
	query := `
SELECT source_uri, source_system, source_org, source_metadata
FROM documents
WHERE object_id = $1
ORDER BY vote_date DESC NULLS LAST`
	rows, err := s.db.QueryContext(ctx, query, objectID)
	if err != nil {
		return nil, fmt.Errorf("list object sources: %w", err)
	}
	defer rows.Close()

	sources := make([]ObjectSource, 0)
	seen := map[string]struct{}{}
	for rows.Next() {
		var sourceURI, sourceSystem, sourceOrg string
		var rawMeta []byte
		if err := rows.Scan(&sourceURI, &sourceSystem, &sourceOrg, &rawMeta); err != nil {
			return nil, fmt.Errorf("scan object source: %w", err)
		}
		if sourceURI == "" {
			continue
		}
		if _, exists := seen[sourceURI]; exists {
			continue
		}
		seen[sourceURI] = struct{}{}
		sourceType := "other"
		if strings.Contains(strings.ToLower(sourceSystem), "openparl") || strings.Contains(strings.ToLower(sourceURI), "admin.ch") {
			sourceType = "official"
		}
		sources = append(sources, ObjectSource{
			Type:   sourceType,
			Title:  normalizeNonEmpty(sourceOrg, sourceSystem),
			URL:    sourceURI,
			Origin: detectOrigin(rawMeta),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate object sources: %w", err)
	}
	return sources, nil
}

func (s *SQLQueryService) GetTaxonomies(ctx context.Context) (Taxonomies, error) {
	if s.db == nil {
		return Taxonomies{}, errors.New("db is required")
	}
	readDistinct := func(query string) ([]string, error) {
		rows, err := s.db.QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := make([]string, 0)
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				return nil, err
			}
			if value != "" {
				out = append(out, value)
			}
		}
		return out, rows.Err()
	}

	levels, err := readDistinct(`SELECT DISTINCT level FROM documents WHERE level <> '' ORDER BY level`)
	if err != nil {
		return Taxonomies{}, fmt.Errorf("levels taxonomies: %w", err)
	}
	cantons, err := readDistinct(`SELECT DISTINCT canton FROM documents WHERE canton <> '' ORDER BY canton`)
	if err != nil {
		return Taxonomies{}, fmt.Errorf("cantons taxonomies: %w", err)
	}
	statuses, err := readDistinct(`SELECT DISTINCT status FROM documents WHERE status <> '' ORDER BY status`)
	if err != nil {
		return Taxonomies{}, fmt.Errorf("status taxonomies: %w", err)
	}
	languages, err := readDistinct(`SELECT DISTINCT lang FROM document_translations WHERE lang <> '' ORDER BY lang`)
	if err != nil {
		return Taxonomies{}, fmt.Errorf("language taxonomies: %w", err)
	}
	objectTypes, err := readDistinct(`SELECT DISTINCT object_type FROM documents WHERE object_type <> '' ORDER BY object_type`)
	if err != nil {
		return Taxonomies{}, fmt.Errorf("type taxonomies: %w", err)
	}
	themes, err := readDistinct(`SELECT DISTINCT object_theme FROM documents WHERE object_theme <> '' ORDER BY object_theme`)
	if err != nil {
		return Taxonomies{}, fmt.Errorf("theme taxonomies: %w", err)
	}

	return Taxonomies{
		Levels:      levels,
		Cantons:     cantons,
		Statuses:    statuses,
		Languages:   languages,
		ObjectTypes: objectTypes,
		Themes:      themes,
	}, nil
}

func extractArguments(rawMetadata []byte) string {
	if len(rawMetadata) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(rawMetadata, &payload); err != nil {
		return ""
	}
	arguments, ok := payload["arguments"].(map[string]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, 2)
	if pro, ok := arguments["pro"].(string); ok && strings.TrimSpace(pro) != "" {
		parts = append(parts, "pro: "+strings.TrimSpace(pro))
	}
	if contra, ok := arguments["contra"].(string); ok && strings.TrimSpace(contra) != "" {
		parts = append(parts, "contra: "+strings.TrimSpace(contra))
	}
	return strings.Join(parts, "\n")
}

func detectOrigin(rawMetadata []byte) string {
	if len(rawMetadata) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(rawMetadata, &payload); err != nil {
		return ""
	}
	if _, ok := payload["selection_strategy"]; ok {
		return "dataset"
	}
	return ""
}

func normalizeNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func normalizeLanguageWithFallback(raw, fallback string, supported []string) string {
	normalized := langs.Normalize(raw)
	if normalized != "" && (len(supported) == 0 || langs.Contains(supported, normalized)) {
		return normalized
	}
	return normalizeNonEmpty(fallback, "fr")
}

func (s *SQLQueryService) applyTranslationFallback(item *VotationListItem, requestedLang string, documentIDs []string) {
	if item == nil {
		return
	}
	requested := normalizeLanguageWithFallback(requestedLang, s.defaultLanguage, s.supportedLanguages)
	item.Language = requested
	if item.DisplayTitles == nil {
		item.DisplayTitles = map[string]string{}
	}
	if item.Titles == nil {
		item.Titles = map[string]string{}
	}
	if value := strings.TrimSpace(item.DisplayTitles[requested]); value != "" {
		item.Translation = &TranslationState{
			State:             rag.TranslationStatusReady,
			RequestedLanguage: requested,
		}
		return
	}

	fallbackLang, fallbackTitle := pickFallbackTitle(item.DisplayTitles, item.Titles, s.fallbackLanguage)
	if strings.TrimSpace(fallbackTitle) == "" {
		return
	}
	item.DisplayTitles[requested] = fallbackTitle
	item.Translation = &TranslationState{
		State:             rag.TranslationStatusPending,
		RequestedLanguage: requested,
		FallbackLanguage:  fallbackLang,
		Message:           "translation in progress",
	}
	s.enqueueTranslationJobs(documentIDs, requested)
}

func (s *SQLQueryService) applyTranslationFallbackDetail(item *VotationDetail, requestedLang string, documentIDs []string) {
	if item == nil {
		return
	}
	requested := normalizeLanguageWithFallback(requestedLang, s.defaultLanguage, s.supportedLanguages)
	item.Language = requested
	if item.DisplayTitles == nil {
		item.DisplayTitles = map[string]string{}
	}
	if item.Titles == nil {
		item.Titles = map[string]string{}
	}
	if value := strings.TrimSpace(item.DisplayTitles[requested]); value != "" {
		item.Translation = &TranslationState{
			State:             rag.TranslationStatusReady,
			RequestedLanguage: requested,
		}
		return
	}
	fallbackLang, fallbackTitle := pickFallbackTitle(item.DisplayTitles, item.Titles, s.fallbackLanguage)
	if strings.TrimSpace(fallbackTitle) == "" {
		return
	}
	item.DisplayTitles[requested] = fallbackTitle
	item.Translation = &TranslationState{
		State:             rag.TranslationStatusPending,
		RequestedLanguage: requested,
		FallbackLanguage:  fallbackLang,
		Message:           "translation in progress",
	}
	s.enqueueTranslationJobs(documentIDs, requested)
}

func pickFallbackTitle(displayTitles map[string]string, titles map[string]string, preferredFallback string) (string, string) {
	fallback := langs.Normalize(preferredFallback)
	if fallback != "" {
		if value := strings.TrimSpace(displayTitles[fallback]); value != "" {
			return fallback, value
		}
		if value := strings.TrimSpace(titles[fallback]); value != "" {
			return fallback, value
		}
	}
	for langCode, value := range displayTitles {
		if strings.TrimSpace(value) != "" {
			return langCode, value
		}
	}
	for langCode, value := range titles {
		if strings.TrimSpace(value) != "" {
			return langCode, value
		}
	}
	return "", ""
}

func (s *SQLQueryService) enqueueTranslationJobs(documentIDs []string, targetLang string) {
	if s.translator == nil || len(documentIDs) == 0 {
		return
	}
	for _, documentID := range documentIDs {
		documentID = strings.TrimSpace(documentID)
		if documentID == "" {
			continue
		}
		key := documentID + "|" + targetLang
		s.mu.Lock()
		if _, exists := s.inFlight[key]; exists {
			s.mu.Unlock()
			continue
		}
		s.inFlight[key] = struct{}{}
		s.mu.Unlock()
		go s.translateDocumentAsync(documentID, targetLang)
	}
}

func (s *SQLQueryService) translateDocumentAsync(documentID, targetLang string) {
	defer func() {
		key := documentID + "|" + targetLang
		s.mu.Lock()
		delete(s.inFlight, key)
		s.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	source, err := s.loadBestSourceTranslation(ctx, documentID, targetLang)
	if err != nil {
		return
	}
	contentHash := hashTranslationContent(source.Lang, source.Title, source.Content)
	if s.translationAlreadyReady(ctx, documentID, targetLang, contentHash) {
		return
	}
	_ = s.upsertTranslation(ctx, translationUpsert{
		DocumentID:     documentID,
		Lang:           targetLang,
		Title:          source.Title,
		Content:        source.Content,
		DisplayTitle:   source.Title,
		Status:         rag.TranslationStatusPending,
		Provider:       s.translator.Name(),
		SourceLang:     source.Lang,
		SourceHash:     contentHash,
		AvailableLangs: source.AvailableLanguages,
	})

	translatedTitle, err := s.translator.Translate(ctx, rag.TranslationRequest{
		Text:         source.Title,
		SourceLang:   source.Lang,
		TargetLang:   targetLang,
		ContentLabel: "title",
	})
	if err != nil {
		_ = s.markTranslationFailed(ctx, documentID, targetLang, source, contentHash)
		return
	}
	translatedContent, err := s.translator.Translate(ctx, rag.TranslationRequest{
		Text:         source.Content,
		SourceLang:   source.Lang,
		TargetLang:   targetLang,
		ContentLabel: "document content",
	})
	if err != nil {
		_ = s.markTranslationFailed(ctx, documentID, targetLang, source, contentHash)
		return
	}
	_ = s.upsertTranslation(ctx, translationUpsert{
		DocumentID:     documentID,
		Lang:           targetLang,
		Title:          translatedTitle,
		Content:        translatedContent,
		DisplayTitle:   translatedTitle,
		Status:         rag.TranslationStatusReady,
		Provider:       s.translator.Name(),
		SourceLang:     source.Lang,
		SourceHash:     contentHash,
		AvailableLangs: source.AvailableLanguages,
	})
}

type sourceTranslation struct {
	Lang               string
	Title              string
	Content            string
	AvailableLanguages []string
}

func (s *SQLQueryService) loadBestSourceTranslation(ctx context.Context, documentID, targetLang string) (sourceTranslation, error) {
	query := `
SELECT
	dt.lang,
	dt.title,
	dt.content_normalized,
	COALESCE(dt.metadata->'available_languages', '[]'::jsonb)
FROM document_translations dt
WHERE dt.document_id = $1 AND dt.lang <> $2
ORDER BY
	CASE
		WHEN dt.lang = 'en' THEN 0
		WHEN dt.lang = 'fr' THEN 1
		ELSE 2
	END,
	dt.lang ASC
LIMIT 1`
	var (
		source sourceTranslation
		raw    []byte
	)
	if err := s.db.QueryRowContext(ctx, query, documentID, targetLang).Scan(&source.Lang, &source.Title, &source.Content, &raw); err != nil {
		return sourceTranslation{}, err
	}
	if len(raw) > 0 {
		var values []string
		if err := json.Unmarshal(raw, &values); err == nil {
			source.AvailableLanguages = values
		}
	}
	if source.Lang == "" {
		source.Lang = s.fallbackLanguage
	}
	return source, nil
}

func (s *SQLQueryService) translationAlreadyReady(ctx context.Context, documentID, lang, sourceHash string) bool {
	query := `
SELECT COALESCE(metadata->>'translation_status', ''), COALESCE(metadata->>'translation_source_hash', '')
FROM document_translations
WHERE document_id = $1 AND lang = $2`
	var status, hash string
	if err := s.db.QueryRowContext(ctx, query, documentID, lang).Scan(&status, &hash); err != nil {
		return false
	}
	return status == rag.TranslationStatusReady && hash == sourceHash
}

type translationUpsert struct {
	DocumentID     string
	Lang           string
	Title          string
	Content        string
	DisplayTitle   string
	Status         string
	Provider       string
	SourceLang     string
	SourceHash     string
	AvailableLangs []string
}

func (s *SQLQueryService) upsertTranslation(ctx context.Context, payload translationUpsert) error {
	query := `
INSERT INTO document_translations (id, document_id, lang, title, summary, content_normalized, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (document_id, lang)
DO UPDATE SET
	id = EXCLUDED.id,
	title = EXCLUDED.title,
	summary = EXCLUDED.summary,
	content_normalized = EXCLUDED.content_normalized,
	metadata = EXCLUDED.metadata`

	metadata := map[string]any{
		"display_title":           strings.TrimSpace(payload.DisplayTitle),
		"translation_status":      payload.Status,
		"translation_provider":    payload.Provider,
		"translation_source_lang": payload.SourceLang,
		"translation_source_hash": payload.SourceHash,
		"translation_updated_at":  time.Now().UTC().Format(time.RFC3339),
		"available_languages":     payload.AvailableLangs,
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	translationID := fmt.Sprintf("%s:%s", payload.DocumentID, payload.Lang)
	_, err = s.db.ExecContext(
		ctx,
		query,
		translationID,
		payload.DocumentID,
		payload.Lang,
		normalizeNonEmpty(payload.Title, "Untitled"),
		"",
		normalizeNonEmpty(payload.Content, payload.Title),
		raw,
	)
	return err
}

func (s *SQLQueryService) markTranslationFailed(ctx context.Context, documentID, targetLang string, source sourceTranslation, sourceHash string) error {
	return s.upsertTranslation(ctx, translationUpsert{
		DocumentID:     documentID,
		Lang:           targetLang,
		Title:          source.Title,
		Content:        source.Content,
		DisplayTitle:   source.Title,
		Status:         rag.TranslationStatusFailed,
		Provider:       s.translator.Name(),
		SourceLang:     source.Lang,
		SourceHash:     sourceHash,
		AvailableLangs: source.AvailableLanguages,
	})
}

func hashTranslationContent(sourceLang, title, content string) string {
	sum := sha256.Sum256([]byte(sourceLang + "|" + strings.TrimSpace(title) + "|" + strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", ":", "-", ".", "-")
	return replacer.Replace(value)
}
