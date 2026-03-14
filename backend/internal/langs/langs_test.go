package langs

import "testing"

func TestParseSupportedMergesAndNormalizes(t *testing.T) {
	got := ParseSupported(" EN,pt-BR,invalid,fr ", []string{"fr", "de"})
	if len(got) != 4 {
		t.Fatalf("expected 4 langs, got %d (%v)", len(got), got)
	}
	expected := []string{"fr", "de", "en", "pt-br"}
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
