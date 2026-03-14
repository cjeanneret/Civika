package rag

import (
	"encoding/json"
	"strings"
	"time"
)

type Intervenant struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role,omitempty"`
}

type SourceMetadata struct {
	SourceSystem       string         `json:"source_system"`
	SourceURI          string         `json:"source_uri"`
	ExternalID         string         `json:"external_id"`
	SourceOrg          string         `json:"source_org"`
	ContentType        string         `json:"content_type"`
	LicenseURI         string         `json:"license_uri"`
	FetchedAtUTC       time.Time      `json:"fetched_at_utc"`
	IssuedAt           *time.Time     `json:"issued_at,omitempty"`
	ModifiedAt         *time.Time     `json:"modified_at,omitempty"`
	AvailableLanguages []string       `json:"available_languages,omitempty"`
	Extra              map[string]any `json:"extra,omitempty"`
}

func sanitizeMetadataValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return sanitizeMetadataMap(t)
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			clean := sanitizeMetadataValue(item)
			if clean != nil {
				out = append(out, clean)
			}
		}
		return out
	default:
		return t
	}
}

func sanitizeMetadataMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if isSensitiveMetadataKey(k) {
			continue
		}
		clean := sanitizeMetadataValue(v)
		if clean == nil {
			continue
		}
		out[k] = clean
	}
	return out
}

func isSensitiveMetadataKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(k, "email") || strings.Contains(k, "contact")
}

func marshalSanitizedMetadata(in map[string]any) []byte {
	if in == nil {
		return []byte("{}")
	}
	raw, err := json.Marshal(sanitizeMetadataMap(in))
	if err != nil {
		return []byte("{}")
	}
	return raw
}
