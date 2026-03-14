package services

import "testing"

func TestPickFallbackTitlePrefersEnglish(t *testing.T) {
	display := map[string]string{"fr": "Titre FR", "en": "Title EN"}
	titles := map[string]string{"de": "Titel DE"}
	lang, value := pickFallbackTitle(display, titles, "en")
	if lang != "en" {
		t.Fatalf("expected en, got %s", lang)
	}
	if value != "Title EN" {
		t.Fatalf("unexpected value: %s", value)
	}
}

func TestNormalizeLanguageWithFallback(t *testing.T) {
	supported := []string{"fr", "de", "en"}
	if got := normalizeLanguageWithFallback("de", "fr", supported); got != "de" {
		t.Fatalf("expected de, got %s", got)
	}
	if got := normalizeLanguageWithFallback("pt", "fr", supported); got != "fr" {
		t.Fatalf("expected fallback fr, got %s", got)
	}
}
