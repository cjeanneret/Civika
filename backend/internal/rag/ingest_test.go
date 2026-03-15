package rag

import (
	"strings"
	"testing"
)

func TestParseJSONDocumentsOpenParlCreatesLanguageVariants(t *testing.T) {
	raw := `{
		"source_system":"openparldata",
		"source_org":"OpenParlData.ch",
		"available_languages":["de","fr"],
		"voting":{
			"id":101,
			"external_id":"external-101",
			"url_api":"https://api.openparldata.ch/v1/votings/101",
			"title":{"de":"Abstimmung","fr":"Votation"},
			"meaning_of_yes":{"de":"Ja","fr":"Oui"}
		},
		"affair":{
			"title":{"de":"Geschaeft","fr":"Objet"},
			"title_long":{"de":"Lange Beschreibung","fr":"Description longue"}
		},
		"texts":[
			{
				"type_localized":{"de":"Bericht","fr":"Rapport"},
				"text_localized":{"de":"Deutscher Inhalt","fr":"Contenu francais"}
			}
		]
	}`

	docs, err := parseJSONDocuments("fixtures/openparldata.json", raw)
	if err != nil {
		t.Fatalf("parseJSONDocuments returned error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 language variants, got %d", len(docs))
	}

	byLang := map[string]Document{}
	for _, doc := range docs {
		byLang[doc.Language] = doc
	}
	fr, ok := byLang["fr"]
	if !ok {
		t.Fatal("expected french variant")
	}
	de, ok := byLang["de"]
	if !ok {
		t.Fatal("expected german variant")
	}
	if !strings.Contains(fr.Content, "Votation") {
		t.Fatalf("expected french content to include localized title, got: %s", fr.Content)
	}
	if strings.Contains(fr.Content, "Abstimmung") {
		t.Fatalf("did not expect german title in french content, got: %s", fr.Content)
	}
	if !strings.Contains(de.Content, "Abstimmung") {
		t.Fatalf("expected german content to include localized title, got: %s", de.Content)
	}
	if fr.TranslationID == de.TranslationID {
		t.Fatalf("translation ids must differ, fr=%s de=%s", fr.TranslationID, de.TranslationID)
	}
}

func TestParseJSONDocumentsOpenParlAddsFilterMetadata(t *testing.T) {
	raw := `{
		"source_system":"openparldata",
		"source_org":"OpenParlData.ch",
		"available_languages":["fr"],
		"voting":{
			"id":101,
			"external_id":"external-101",
			"body_key":"GE",
			"date":"2026-12-20T10:00:00Z",
			"url_api":"https://api.openparldata.ch/v1/votings/101",
			"title":{"fr":"Votation"}
		},
		"affair":{
			"id":456,
			"external_id":"obj-456",
			"type_name":{"fr":"Motion"},
			"state_name":{"fr":"Ouvert"}
		}
	}`

	docs, err := parseJSONDocuments("fixtures/openparldata.json", raw)
	if err != nil {
		t.Fatalf("parseJSONDocuments returned error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(docs))
	}
	doc := docs[0]
	if got := toString(doc.Metadata["votation_id"]); got != "external-101" {
		t.Fatalf("expected votation_id metadata, got %q", got)
	}
	if got := toString(doc.Metadata["object_id"]); got != "obj-456" {
		t.Fatalf("expected object_id metadata, got %q", got)
	}
	if got := toString(doc.Metadata["level"]); got != "cantonal" {
		t.Fatalf("expected level=cantonal, got %q", got)
	}
	if got := toString(doc.Metadata["canton"]); got != "GE" {
		t.Fatalf("expected canton=GE, got %q", got)
	}
	if got := toString(doc.Metadata["status"]); got == "" {
		t.Fatal("expected non-empty status metadata")
	}
	if got := toString(doc.Metadata["vote_date"]); got == "" {
		t.Fatal("expected vote_date metadata")
	}
}

func TestParseJSONDocumentsOpenParlUsesAffairTitleAsDisplayTitle(t *testing.T) {
	raw := `{
		"source_system":"openparldata",
		"source_org":"OpenParlData.ch",
		"available_languages":["de"],
		"voting":{
			"id":101,
			"external_id":"external-101",
			"url_api":"https://api.openparldata.ch/v1/votings/101",
			"title":{"de":"Abstimmung"},
			"affair_title":{"de":"Geschaeftstitel"}
		},
		"affair":{
			"id":456,
			"title":{"de":"Alternativer Titel"}
		}
	}`

	docs, err := parseJSONDocuments("fixtures/openparldata.json", raw)
	if err != nil {
		t.Fatalf("parseJSONDocuments returned error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(docs))
	}
	if docs[0].Title != "Geschaeftstitel" {
		t.Fatalf("expected display title from affair_title, got %q", docs[0].Title)
	}
	if got := toString(docs[0].Metadata["display_title"]); got != "Geschaeftstitel" {
		t.Fatalf("expected display_title metadata, got %q", got)
	}
}

func TestParseJSONDocumentsOpenParlFallsBackToAnyLocalizedTitle(t *testing.T) {
	raw := `{
		"source_system":"openparldata",
		"source_org":"OpenParlData.ch",
		"available_languages":["fr"],
		"voting":{
			"id":101451,
			"external_id":"101451",
			"url_api":"https://api.openparldata.ch/v1/votings/101451",
			"affair_title":{"de":"Volksschule Wankdorf"}
		},
		"affair":{
			"title":{"de":"Volksschule Wankdorf"}
		}
	}`

	docs, err := parseJSONDocuments("fixtures/01-voting-101451.json", raw)
	if err != nil {
		t.Fatalf("parseJSONDocuments returned error: %v", err)
	}
	if len(docs) < 1 {
		t.Fatalf("expected at least 1 variant, got %d", len(docs))
	}
	byLang := map[string]Document{}
	for _, doc := range docs {
		byLang[doc.Language] = doc
	}
	fr, ok := byLang["fr"]
	if !ok {
		t.Fatal("expected fr variant")
	}
	if fr.Title != "Volksschule Wankdorf" {
		t.Fatalf("expected fallback localized title, got %q", fr.Title)
	}
	if strings.Contains(fr.Title, "01-voting-101451") {
		t.Fatalf("expected title not to fallback to filename, got %q", fr.Title)
	}
}

func TestBuildOpenParlFiltersMetadataAddsCommune(t *testing.T) {
	payload := map[string]any{
		"affair": map[string]any{
			"id":          "obj-1",
			"external_id": "obj-1",
		},
	}
	voting := map[string]any{
		"id":                "v1",
		"external_id":       "v1",
		"body_key":          "ZH",
		"municipality_code": "261",
		"municipality_name": "Zurich",
	}

	meta := buildOpenParlFiltersMetadata(payload, voting)
	if got := toString(meta["commune_code"]); got != "261" {
		t.Fatalf("expected commune_code=261, got %q", got)
	}
	if got := toString(meta["commune_name"]); got != "Zurich" {
		t.Fatalf("expected commune_name=Zurich, got %q", got)
	}
	if got := toString(meta["level"]); got != "communal" {
		t.Fatalf("expected level=communal when commune is present, got %q", got)
	}
}

func TestParseJSONDocumentsOpenParlUsesDeclaredAvailableLanguagesOnly(t *testing.T) {
	raw := `{
		"source_system":"openparldata",
		"source_org":"OpenParlData.ch",
		"available_languages":["de"],
		"voting":{
			"id":101451,
			"external_id":"101451",
			"url_api":"https://api.openparldata.ch/v1/votings/101451",
			"title":{"de":"2024.PRD.0018"},
			"affair_title":{"de":"Volksschule Wankdorf"},
			"meaning_of_yes":{"de":"Annahme","fr":"Nein"}
		},
		"affair":{
			"title":{"de":"Volksschule Wankdorf"}
		}
	}`

	docs, err := parseJSONDocuments("fixtures/openparldata.json", raw)
	if err != nil {
		t.Fatalf("parseJSONDocuments returned error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 variant from declared languages, got %d", len(docs))
	}
	if docs[0].Language != "de" {
		t.Fatalf("expected de variant, got %q", docs[0].Language)
	}
}

func TestBuildOpenParlFiltersMetadataDoesNotUseNumericBodyKeyAsCanton(t *testing.T) {
	payload := map[string]any{
		"affair": map[string]any{
			"id":          "obj-2",
			"external_id": "obj-2",
		},
	}
	voting := map[string]any{
		"id":          "v2",
		"external_id": "v2",
		"body_key":    "351",
	}

	meta := buildOpenParlFiltersMetadata(payload, voting)
	if got := toString(meta["canton"]); got != "" {
		t.Fatalf("expected empty canton for numeric body_key, got %q", got)
	}
	if got := toString(meta["commune_code"]); got != "351" {
		t.Fatalf("expected commune_code=351 from numeric body_key, got %q", got)
	}
	if got := toString(meta["level"]); got != "communal" {
		t.Fatalf("expected level=communal, got %q", got)
	}
}

func TestBuildOpenParlFiltersMetadataUsesPayloadCantonFallback(t *testing.T) {
	payload := map[string]any{
		"canton": "BE",
		"affair": map[string]any{
			"id":          "obj-3",
			"external_id": "obj-3",
		},
	}
	voting := map[string]any{
		"id":          "v3",
		"external_id": "v3",
		"body_key":    "351",
	}

	meta := buildOpenParlFiltersMetadata(payload, voting)
	if got := toString(meta["canton"]); got != "BE" {
		t.Fatalf("expected canton=BE from payload fallback, got %q", got)
	}
}
