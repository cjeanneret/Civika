package langs

import (
	"regexp"
	"sort"
	"strings"
)

var languageCodePattern = regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`)

func Normalize(code string) string {
	normalized := strings.ToLower(strings.TrimSpace(code))
	if !languageCodePattern.MatchString(normalized) {
		return ""
	}
	return normalized
}

func ParseSupported(raw string, defaults []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(defaults))

	push := func(value string) {
		normalized := Normalize(value)
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	for _, value := range defaults {
		push(value)
	}
	for _, part := range strings.Split(raw, ",") {
		push(part)
	}
	return out
}

func NormalizeOrFallback(raw, fallback string, supported []string) string {
	candidate := Normalize(raw)
	if candidate == "" {
		return fallback
	}
	if len(supported) == 0 {
		return candidate
	}
	for _, value := range supported {
		if value == candidate {
			return candidate
		}
	}
	return fallback
}

func Contains(list []string, code string) bool {
	for _, value := range list {
		if value == code {
			return true
		}
	}
	return false
}

func StableSorted(input []string) []string {
	out := make([]string, 0, len(input))
	seen := map[string]struct{}{}
	for _, value := range input {
		normalized := Normalize(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}
