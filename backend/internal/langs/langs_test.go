package langs

import "testing"

func TestParseSupportedOverridesDefaultsWhenConfigured(t *testing.T) {
	got := ParseSupported(" EN,pt-BR,invalid,fr ", []string{"fr", "de"})
	if len(got) != 3 {
		t.Fatalf("expected 3 langs, got %d (%v)", len(got), got)
	}
	expected := []string{"en", "pt-br", "fr"}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	}
}

func TestParseSupportedUsesDefaultsWhenUnset(t *testing.T) {
	got := ParseSupported(" ", []string{"fr", "de"})
	if len(got) != 2 {
		t.Fatalf("expected 2 langs, got %d (%v)", len(got), got)
	}
	expected := []string{"fr", "de"}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	}
}

func TestNormalizeOrFallback(t *testing.T) {
	supported := []string{"fr", "en"}
	if got := NormalizeOrFallback("EN", "fr", supported); got != "en" {
		t.Fatalf("expected en, got %s", got)
	}
	if got := NormalizeOrFallback("it", "fr", supported); got != "fr" {
		t.Fatalf("expected fallback fr, got %s", got)
	}
}
