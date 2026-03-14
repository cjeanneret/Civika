package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"civika/backend/internal/langs"
	"github.com/ledongthuc/pdf"
	"golang.org/x/net/html"
)

var supportedLanguages = []string{"fr", "de", "it", "rm", "en"}

func SetSupportedLanguages(list []string) {
	parsed := langs.StableSorted(list)
	if len(parsed) == 0 {
		return
	}
	supportedLanguages = parsed
}

type Document struct {
	ID            string
	TranslationID string
	Language      string
	SourcePath    string
	Title         string
	Content       string
	ContentType   string
	Summary       string
	Source        SourceMetadata
	Intervenants  []Intervenant
	Metadata      map[string]any
}

type IngestConfig struct {
	MaxFileBytes int64
}

func (c IngestConfig) withDefaults() IngestConfig {
	if c.MaxFileBytes <= 0 {
		c.MaxFileBytes = 2 * 1024 * 1024
	}
	return c
}

func IngestCorpus(ctx context.Context, rootPath string, cfg IngestConfig) ([]Document, error) {
	cfg = cfg.withDefaults()
	if strings.TrimSpace(rootPath) == "" {
		return nil, errors.New("corpus path is required")
	}

	var documents []Document
	walkErr := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk corpus: %w", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}

		docs, handled, parseErr := ingestFile(path, cfg.MaxFileBytes)
		if parseErr != nil {
			return parseErr
		}
		if handled {
			documents = append(documents, docs...)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if len(documents) == 0 {
		return nil, errors.New("no supported files found in corpus path")
	}
	return documents, nil
}

func ingestFile(path string, maxFileBytes int64) ([]Document, bool, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown", ".txt":
		text, err := readTextFile(path, maxFileBytes)
		if err != nil {
			return nil, false, err
		}
		title, front, body := parseMarkdownBlocks(text)
		normalized := normalizeWhitespace(stripMarkdown(body))
		if normalized == "" {
			return nil, false, nil
		}
		lang := normalizeLanguage(front["lang"])
		if lang == "" {
			lang = inferLanguage(front["source_url"], path)
		}
		source := SourceMetadata{
			SourceSystem: normalizeString(front["source_system"], inferSourceSystem(front["source_url"], path)),
			SourceURI:    front["source_url"],
			ExternalID:   normalizeString(front["external_id"], inferExternalID(front["source_url"], path)),
			SourceOrg:    front["source_org"],
			ContentType:  normalizeString(front["source_content_type"], "text/markdown"),
			LicenseURI:   front["license_uri"],
		}
		if source.SourceSystem == "" {
			source.SourceSystem = "local_fixture"
		}
		if source.SourceURI == "" {
			source.SourceURI = path
		}
		if source.ExternalID == "" {
			source.ExternalID = documentID(path)
		}
		if ts, ok := parseTime(front["fetched_at_utc"]); ok {
			source.FetchedAtUTC = ts
		} else {
			source.FetchedAtUTC = time.Now().UTC()
		}
		docID := buildDocumentID(source.SourceSystem, source.ExternalID)
		translationID := buildTranslationID(docID, lang)
		meta := sanitizeMetadataMap(map[string]any{
			"source_url":          front["source_url"],
			"source_org":          front["source_org"],
			"source_content_type": front["source_content_type"],
			"license_note":        front["license_note"],
		})
		return []Document{{
			ID:            docID,
			TranslationID: translationID,
			Language:      lang,
			SourcePath:    path,
			Title:         normalizeString(title, guessTitleFromPath(path)),
			Content:       normalized,
			ContentType:   "text/markdown",
			Source:        source,
			Intervenants:  parseIntervenants(front["intervenants"]),
			Metadata:      meta,
		}}, true, nil
	case ".json":
		text, err := readTextFile(path, maxFileBytes)
		if err != nil {
			return nil, false, err
		}
		docs, parseErr := parseJSONDocuments(path, text)
		if parseErr != nil {
			return nil, false, parseErr
		}
		return docs, len(docs) > 0, nil
	case ".csv":
		text, err := readTextFile(path, maxFileBytes)
		if err != nil {
			return nil, false, err
		}
		normalized := normalizeWhitespace(text)
		if normalized == "" {
			return nil, false, nil
		}
		source := SourceMetadata{
			SourceSystem: "csv_fixture",
			SourceURI:    path,
			ExternalID:   documentID(path),
			ContentType:  "text/csv",
			FetchedAtUTC: time.Now().UTC(),
		}
		docID := buildDocumentID(source.SourceSystem, source.ExternalID)
		return []Document{{
			ID:            docID,
			TranslationID: buildTranslationID(docID, "fr"),
			Language:      "fr",
			SourcePath:    path,
			Title:         guessTitleFromPath(path),
			Content:       normalized,
			ContentType:   "text/csv",
			Source:        source,
			Metadata:      map[string]any{},
		}}, true, nil
	case ".html", ".htm":
		text, err := readTextFile(path, maxFileBytes)
		if err != nil {
			return nil, false, err
		}
		plain, err := extractHTMLText(text)
		if err != nil {
			return nil, false, fmt.Errorf("parse html %s: %w", path, err)
		}
		plain = normalizeWhitespace(plain)
		if plain == "" {
			return nil, false, nil
		}
		source := SourceMetadata{
			SourceSystem: "html_fixture",
			SourceURI:    path,
			ExternalID:   documentID(path),
			ContentType:  "text/html",
			FetchedAtUTC: time.Now().UTC(),
		}
		docID := buildDocumentID(source.SourceSystem, source.ExternalID)
		return []Document{{
			ID:            docID,
			TranslationID: buildTranslationID(docID, "fr"),
			Language:      "fr",
			SourcePath:    path,
			Title:         guessTitleFromPath(path),
			Content:       plain,
			ContentType:   "text/html",
			Source:        source,
			Metadata:      map[string]any{},
		}}, true, nil
	case ".pdf":
		plain, err := extractPDFText(path)
		if err != nil {
			return nil, false, fmt.Errorf("parse pdf %s: %w", path, err)
		}
		plain = normalizeWhitespace(plain)
		if plain == "" {
			return nil, false, nil
		}
		if int64(len(plain)) > maxFileBytes {
			return nil, false, fmt.Errorf("pdf %s exceeds size limit", path)
		}
		source := SourceMetadata{
			SourceSystem: "pdf_fixture",
			SourceURI:    path,
			ExternalID:   documentID(path),
			ContentType:  "application/pdf",
			FetchedAtUTC: time.Now().UTC(),
		}
		docID := buildDocumentID(source.SourceSystem, source.ExternalID)
		return []Document{{
			ID:            docID,
			TranslationID: buildTranslationID(docID, "fr"),
			Language:      "fr",
			SourcePath:    path,
			Title:         guessTitleFromPath(path),
			Content:       plain,
			ContentType:   "application/pdf",
			Source:        source,
			Metadata:      map[string]any{},
		}}, true, nil
	default:
		return nil, false, nil
	}
}

func readTextFile(path string, maxFileBytes int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > maxFileBytes {
		return "", fmt.Errorf("file %s exceeds size limit", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(raw), nil
}

func extractHTMLText(htmlInput string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlInput))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	var walker func(*html.Node)
	walker = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				buf.WriteString(text)
				buf.WriteString(" ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walker(c)
		}
	}
	walker(doc)
	return buf.String(), nil
}

func extractPDFText(path string) (string, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var out strings.Builder
	totalPages := reader.NumPage()
	for i := 1; i <= totalPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return "", err
		}
		out.WriteString(text)
		out.WriteString("\n")
	}
	return out.String(), nil
}

func stripMarkdown(input string) string {
	replacer := strings.NewReplacer(
		"#", " ",
		"*", " ",
		"_", " ",
		"`", " ",
		"[", " ",
		"]", " ",
		"(", " ",
		")", " ",
	)
	return replacer.Replace(input)
}

func normalizeWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func guessTitleFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func documentID(path string) string {
	clean := strings.ReplaceAll(path, string(os.PathSeparator), "_")
	clean = strings.ReplaceAll(clean, ".", "_")
	return clean
}

func parseMarkdownBlocks(raw string) (string, map[string]string, string) {
	lines := strings.Split(raw, "\n")
	front := make(map[string]string)
	title := ""
	bodyStart := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			bodyStart = i + 1
			continue
		}
		if strings.HasPrefix(trimmed, "# ") && title == "" {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			bodyStart = i + 1
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			entry := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if idx := strings.Index(entry, ":"); idx > 0 {
				key := strings.ToLower(strings.TrimSpace(entry[:idx]))
				value := strings.TrimSpace(entry[idx+1:])
				front[key] = value
				bodyStart = i + 1
				continue
			}
		}
		break
	}
	body := strings.Join(lines[bodyStart:], "\n")
	return title, front, body
}

func parseJSONDocuments(path string, raw string) ([]Document, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("parse json %s: %w", path, err)
	}
	if isOpenParlNormalizedFixture(payload) {
		return parseOpenParlJSONDocuments(path, payload)
	}

	doc, err := parseGenericJSONDocument(path, payload)
	if err != nil {
		return nil, err
	}
	normalized := normalizeWhitespace(doc.Content)
	if normalized == "" {
		return nil, nil
	}
	doc.Content = normalized
	return []Document{doc}, nil
}

func parseGenericJSONDocument(path string, payload map[string]any) (Document, error) {
	sanitized := sanitizeMetadataMap(payload)
	metadata := sanitizeMetadataMap(map[string]any{
		"help": payload["help"],
	})

	sourceURI := toString(payload["help"])
	sourceSystem := "json_fixture"
	externalID := documentID(path)
	title := guessTitleFromPath(path)
	lang := "fr"
	var available []string
	issuedAt, modifiedAt := (*time.Time)(nil), (*time.Time)(nil)
	license := ""
	sourceOrg := ""
	intervenants := []Intervenant{}

	if result, ok := payload["result"].(map[string]any); ok {
		if arr, ok := result["results"].([]any); ok && len(arr) > 0 {
			if first, ok := arr[0].(map[string]any); ok {
				sourceSystem = "opendata_swiss"
				externalID = normalizeString(toString(first["id"]), normalizeString(toString(first["identifier"]), normalizeString(toString(first["name"]), externalID)))
				sourceURI = normalizeString(toString(first["url"]), sourceURI)
				sourceOrg = extractLocalizedString(first["organization"], "display_name", "fr")
				license = normalizeString(toString(first["license_title"]), toString(first["license_id"]))
				title = extractLocalizedFromMap(first, "title", "fr", title)
				available = extractLanguages(first["language"])
				if len(available) > 0 {
					lang = pickPreferredLanguage(available)
				}
				if v, ok := parseTime(toString(first["issued"])); ok {
					issuedAt = &v
				}
				if v, ok := parseTime(toString(first["modified"])); ok {
					modifiedAt = &v
				}
				intervenants = extractIntervenants(first)
			}
		}
	}

	source := SourceMetadata{
		SourceSystem:       sourceSystem,
		SourceURI:          sourceURI,
		ExternalID:         externalID,
		SourceOrg:          sourceOrg,
		ContentType:        "application/json",
		LicenseURI:         license,
		FetchedAtUTC:       time.Now().UTC(),
		IssuedAt:           issuedAt,
		ModifiedAt:         modifiedAt,
		AvailableLanguages: available,
		Extra:              sanitized,
	}
	docID := buildDocumentID(source.SourceSystem, source.ExternalID)
	translationID := buildTranslationID(docID, lang)

	prettySanitized, err := json.Marshal(sanitized)
	if err != nil {
		return Document{}, fmt.Errorf("marshal sanitized json: %w", err)
	}

	return Document{
		ID:            docID,
		TranslationID: translationID,
		Language:      lang,
		SourcePath:    path,
		Title:         title,
		Content:       string(prettySanitized),
		ContentType:   "application/json",
		Source:        source,
		Intervenants:  intervenants,
		Metadata:      metadata,
	}, nil
}

func parseOpenParlJSONDocuments(path string, payload map[string]any) ([]Document, error) {
	sanitized := sanitizeMetadataMap(payload)
	metadata := sanitizeMetadataMap(map[string]any{
		"selection_strategy": payload["selection_strategy"],
		"help":               payload["help"],
	})

	sourceSystem := normalizeString(toString(payload["source_system"]), "openparldata")
	sourceOrg := normalizeString(toString(payload["source_org"]), "OpenParlData.ch")
	sourceURI := normalizeString(toString(payload["help"]), path)
	externalID := documentID(path)
	titleFallback := guessTitleFromPath(path)
	intervenants := extractInitiants(payload["initiants"])

	if voting, ok := payload["voting"].(map[string]any); ok {
		sourceURI = normalizeString(toString(voting["url_api"]), sourceURI)
		externalID = normalizeString(toString(voting["external_id"]), normalizeString(toString(voting["id"]), externalID))
		enriched := buildOpenParlFiltersMetadata(payload, voting)
		for key, value := range enriched {
			metadata[key] = value
		}
	}

	available := extractLanguagesFromNormalizedFixture(payload)
	if len(available) == 0 {
		available = []string{inferLanguage(sourceURI, path)}
	}

	docID := buildDocumentID(sourceSystem, externalID)
	source := SourceMetadata{
		SourceSystem:       sourceSystem,
		SourceURI:          sourceURI,
		ExternalID:         externalID,
		SourceOrg:          sourceOrg,
		ContentType:        "application/json",
		FetchedAtUTC:       time.Now().UTC(),
		AvailableLanguages: available,
		Extra:              sanitized,
	}

	out := make([]Document, 0, len(available))
	for _, lang := range available {
		projected := projectByLanguage(payload, lang)
		if projected == nil {
			continue
		}
		serialized, err := json.Marshal(projected)
		if err != nil {
			return nil, fmt.Errorf("marshal localized json (%s): %w", lang, err)
		}
		content := normalizeWhitespace(string(serialized))
		if content == "" {
			continue
		}
		title := deriveOpenParlDisplayTitle(payload, lang, titleFallback)
		metadataByLang := cloneMetadataMap(metadata)
		metadataByLang["display_title"] = title
		out = append(out, Document{
			ID:            docID,
			TranslationID: buildTranslationID(docID, lang),
			Language:      lang,
			SourcePath:    path,
			Title:         title,
			Content:       content,
			ContentType:   "application/json",
			Source:        source,
			Intervenants:  intervenants,
			Metadata:      metadataByLang,
		})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func buildOpenParlFiltersMetadata(payload map[string]any, voting map[string]any) map[string]any {
	result := map[string]any{}
	votationID := normalizeString(toString(voting["external_id"]), toString(voting["id"]))
	if votationID != "" {
		result["votation_id"] = votationID
	}
	bodyKey := strings.ToUpper(strings.TrimSpace(toString(voting["body_key"])))
	if bodyKey != "" {
		result["canton"] = bodyKey
	}
	communeCode, communeName := extractOpenParlCommune(voting, payload)
	if communeCode != "" {
		result["commune_code"] = communeCode
	}
	if communeName != "" {
		result["commune_name"] = communeName
	}
	level := deriveLevel(bodyKey)
	if communeCode != "" || communeName != "" {
		level = "communal"
	}
	if level != "" {
		result["level"] = level
	}
	if dateValue, ok := parseTime(toString(voting["date"])); ok {
		result["vote_date"] = dateValue.Format(time.RFC3339)
		if dateValue.After(time.Now().UTC()) {
			result["status"] = "upcoming"
		} else {
			result["status"] = "past"
		}
	}
	if affair, ok := payload["affair"].(map[string]any); ok {
		objectID := normalizeString(toString(affair["external_id"]), toString(affair["id"]))
		if objectID != "" {
			result["object_id"] = objectID
		}
		result["object_type"] = pickPreferredLocalizedValue(affair["type_name"])
		result["object_theme"] = pickPreferredLocalizedValue(affair["state_name"])
	}
	result["source_type"] = "official"
	return sanitizeMetadataMap(result)
}

func deriveLevel(bodyKey string) string {
	switch bodyKey {
	case "CH", "BUND":
		return "federal"
	default:
		if regexp.MustCompile(`^[A-Z]{2}$`).MatchString(bodyKey) {
			return "cantonal"
		}
		return "communal"
	}
}

func deriveOpenParlDisplayTitle(payload map[string]any, lang string, fallback string) string {
	if voting, ok := payload["voting"].(map[string]any); ok {
		if value := normalizeString(extractLocalizedValue(voting["affair_title"], lang), ""); value != "" {
			return value
		}
	}
	if affair, ok := payload["affair"].(map[string]any); ok {
		if value := normalizeString(extractLocalizedValue(affair["title"], lang), ""); value != "" {
			return value
		}
	}
	if voting, ok := payload["voting"].(map[string]any); ok {
		if value := normalizeString(extractLocalizedValue(voting["title"], lang), ""); value != "" {
			return value
		}
	}
	return fallback
}

func extractOpenParlCommune(voting map[string]any, payload map[string]any) (string, string) {
	keysCode := []string{"commune_code", "municipality_code", "gemeinde_code", "comune_code", "vischnanca_code"}
	keysName := []string{"commune_name", "municipality_name", "gemeinde_name", "comune_name", "vischnanca_name", "commune", "municipality", "gemeinde", "comune", "vischnanca"}

	code := firstNonEmptyMapValue(voting, keysCode...)
	name := firstNonEmptyMapValue(voting, keysName...)
	if code == "" {
		code = firstNonEmptyMapValue(payload, keysCode...)
	}
	if name == "" {
		name = firstNonEmptyMapValue(payload, keysName...)
	}

	if name == "" {
		if affair, ok := payload["affair"].(map[string]any); ok {
			name = firstNonEmptyMapValue(affair, keysName...)
		}
	}
	return strings.TrimSpace(code), strings.TrimSpace(name)
}

func firstNonEmptyMapValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(toString(values[key])); value != "" {
			return value
		}
	}
	return ""
}

func cloneMetadataMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func pickPreferredLocalizedValue(raw any) string {
	values, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range supportedLanguages {
		if val := strings.TrimSpace(toString(values[key])); val != "" {
			return val
		}
	}
	for _, item := range values {
		if val := strings.TrimSpace(toString(item)); val != "" {
			return val
		}
	}
	return ""
}

func isOpenParlNormalizedFixture(payload map[string]any) bool {
	if strings.ToLower(strings.TrimSpace(toString(payload["source_system"]))) != "openparldata" {
		return false
	}
	_, hasVoting := payload["voting"].(map[string]any)
	_, hasAffair := payload["affair"].(map[string]any)
	return hasVoting || hasAffair
}

func extractInitiants(raw any) []Intervenant {
	array, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]Intervenant, 0, len(array))
	seen := map[string]struct{}{}
	for _, item := range array {
		person, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fullname := strings.TrimSpace(toString(person["fullname"]))
		if fullname == "" {
			continue
		}
		nameParts := strings.Fields(fullname)
		if len(nameParts) < 2 {
			continue
		}
		firstName := nameParts[0]
		lastName := strings.Join(nameParts[1:], " ")
		key := strings.ToLower(firstName + "|" + lastName)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, Intervenant{
			FirstName: firstName,
			LastName:  lastName,
			Role:      strings.TrimSpace(toString(person["role"])),
		})
	}
	return out
}

func extractLanguagesFromNormalizedFixture(payload map[string]any) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 5)
	push := func(raw string) {
		lang := normalizeLanguage(raw)
		if lang == "" {
			return
		}
		if _, exists := seen[lang]; exists {
			return
		}
		seen[lang] = struct{}{}
		out = append(out, lang)
	}
	if declared, ok := payload["available_languages"].([]any); ok {
		for _, item := range declared {
			push(toString(item))
		}
	}
	collectLanguageCandidates(payload, push)
	return out
}

func collectLanguageCandidates(value any, push func(string)) {
	switch typed := value.(type) {
	case map[string]any:
		if isLanguageMap(typed) {
			for key, val := range typed {
				if strings.TrimSpace(toString(val)) == "" {
					continue
				}
				push(key)
			}
			return
		}
		for _, nested := range typed {
			collectLanguageCandidates(nested, push)
		}
	case []any:
		for _, nested := range typed {
			collectLanguageCandidates(nested, push)
		}
	}
}

func projectByLanguage(value any, lang string) any {
	switch typed := value.(type) {
	case map[string]any:
		if isLanguageMap(typed) {
			return strings.TrimSpace(toString(typed[lang]))
		}
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			projected := projectByLanguage(nested, lang)
			if isEmptyProjectedValue(projected) {
				continue
			}
			out[key] = projected
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, nested := range typed {
			projected := projectByLanguage(nested, lang)
			if isEmptyProjectedValue(projected) {
				continue
			}
			out = append(out, projected)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return typed
	}
}

func isLanguageMap(values map[string]any) bool {
	if len(values) == 0 {
		return false
	}
	hasLanguageKey := false
	for key := range values {
		if normalizeLanguage(key) == "" {
			return false
		}
		hasLanguageKey = true
	}
	return hasLanguageKey
}

func isEmptyProjectedValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case map[string]any:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

func extractLocalizedValue(raw any, lang string) string {
	switch typed := raw.(type) {
	case map[string]any:
		if value := strings.TrimSpace(toString(typed[lang])); value != "" {
			return value
		}
	case map[string]string:
		if value := strings.TrimSpace(typed[lang]); value != "" {
			return value
		}
	}
	return ""
}

func parseIntervenants(raw string) []Intervenant {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ";")
	seen := map[string]struct{}{}
	result := make([]Intervenant, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) < 2 {
			continue
		}
		firstName := fields[0]
		lastName := strings.Join(fields[1:], " ")
		key := strings.ToLower(firstName + "|" + lastName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, Intervenant{
			FirstName: firstName,
			LastName:  lastName,
		})
	}
	return result
}

func inferSourceSystem(sourceURL, path string) string {
	uri := strings.ToLower(sourceURL)
	switch {
	case strings.Contains(uri, "abstimmungen.admin.ch"):
		return "abstimmungen_admin"
	case strings.Contains(uri, "opendata.swiss"):
		return "opendata_swiss"
	default:
		if strings.HasSuffix(strings.ToLower(path), ".json") {
			return "json_fixture"
		}
		return "local_fixture"
	}
}

func inferExternalID(sourceURL, path string) string {
	if sourceURL != "" {
		u, err := url.Parse(sourceURL)
		if err == nil {
			if proposalID := u.Query().Get("proposalId"); proposalID != "" {
				return proposalID
			}
		}
	}
	return documentID(path)
}

func inferLanguage(sourceURL, path string) string {
	uri := strings.ToLower(sourceURL + " " + path)
	switch {
	case strings.Contains(uri, "/de/"):
		return "de"
	case strings.Contains(uri, "/it/"):
		return "it"
	case strings.Contains(uri, "/rm/"):
		return "rm"
	case strings.Contains(uri, "/en/"):
		return "en"
	default:
		return "fr"
	}
}

func normalizeLanguage(lang string) string {
	l := langs.Normalize(lang)
	if l == "" {
		return ""
	}
	if !langs.Contains(supportedLanguages, l) {
		return ""
	}
	return l
}

func buildDocumentID(sourceSystem, externalID string) string {
	safeSystem := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(sourceSystem)), " ", "_")
	safeID := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(externalID)), " ", "_")
	if safeSystem == "" {
		safeSystem = "source"
	}
	if safeID == "" {
		safeID = "document"
	}
	return fmt.Sprintf("%s:%s", safeSystem, safeID)
}

func buildTranslationID(documentID, lang string) string {
	l := normalizeLanguage(lang)
	if l == "" {
		l = "fr"
	}
	return fmt.Sprintf("%s:%s", documentID, l)
}

func parseTime(raw string) (time.Time, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, v); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

func normalizeString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func extractLocalizedFromMap(parent map[string]any, key, lang, fallback string) string {
	raw, ok := parent[key]
	if !ok {
		return fallback
	}
	if v, ok := raw.(string); ok {
		return normalizeString(v, fallback)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return fallback
	}
	if val := toString(m[lang]); val != "" {
		return val
	}
	for _, code := range supportedLanguages {
		if val := toString(m[code]); val != "" {
			return val
		}
	}
	return fallback
}

func extractLocalizedString(parent any, key, lang string) string {
	m, ok := parent.(map[string]any)
	if !ok {
		return ""
	}
	return extractLocalizedFromMap(m, key, lang, "")
}

func extractLanguages(raw any) []string {
	array, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(array))
	seen := map[string]struct{}{}
	for _, item := range array {
		lang := normalizeLanguage(toString(item))
		if lang == "" {
			continue
		}
		if _, exists := seen[lang]; exists {
			continue
		}
		seen[lang] = struct{}{}
		out = append(out, lang)
	}
	return out
}

func pickPreferredLanguage(langs []string) string {
	if len(langs) == 0 {
		if len(supportedLanguages) > 0 {
			return supportedLanguages[0]
		}
		return "fr"
	}
	for _, preferred := range supportedLanguages {
		for _, candidate := range langs {
			if candidate == preferred {
				return candidate
			}
		}
	}
	return langs[0]
}

func extractIntervenants(dataset map[string]any) []Intervenant {
	author := strings.TrimSpace(toString(dataset["author"]))
	if author == "" {
		return nil
	}
	parts := strings.Split(author, ",")
	seen := map[string]struct{}{}
	var out []Intervenant
	for _, part := range parts {
		tokens := strings.Fields(strings.TrimSpace(part))
		if len(tokens) < 2 {
			continue
		}
		firstName := tokens[0]
		lastName := strings.Join(tokens[1:], " ")
		key := strings.ToLower(firstName + "|" + lastName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, Intervenant{
			FirstName: firstName,
			LastName:  lastName,
			Role:      "source_contributor",
		})
	}
	return out
}
