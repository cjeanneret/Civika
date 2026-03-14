package rag

import "testing"

func TestSanitizeMetadataMapRemovesSensitiveKeys(t *testing.T) {
	input := map[string]any{
		"maintainer_email": "x@y.z",
		"contact_points": []any{
			map[string]any{
				"email": "a@b.c",
				"name":  "visible",
			},
		},
		"title": "keep",
	}

	sanitized := sanitizeMetadataMap(input)
	if _, exists := sanitized["maintainer_email"]; exists {
		t.Fatal("expected maintainer_email to be removed")
	}
	if _, exists := sanitized["contact_points"]; exists {
		t.Fatal("expected contact_points to be removed")
	}
	if sanitized["title"] != "keep" {
		t.Fatal("expected non-sensitive key to be preserved")
	}
}

