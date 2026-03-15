package rag

import "testing"

func TestParseUsageFromResponseBody_OpenAIStyle(t *testing.T) {
	raw := []byte(`{"usage":{"prompt_tokens":12,"completion_tokens":7,"total_tokens":19}}`)
	usage, err := ParseUsageFromResponseBody(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.InputTokens != 12 {
		t.Fatalf("input tokens mismatch: got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 7 {
		t.Fatalf("output tokens mismatch: got %d", usage.OutputTokens)
	}
	if usage.TotalTokens != 19 {
		t.Fatalf("total tokens mismatch: got %d", usage.TotalTokens)
	}
	if usage.UsageSource != "provider" {
		t.Fatalf("usage source mismatch: got %s", usage.UsageSource)
	}
}

func TestParseUsageFromResponseBody_InputOutputStyle(t *testing.T) {
	raw := []byte(`{"usage":{"input_tokens":33,"output_tokens":9}}`)
	usage, err := ParseUsageFromResponseBody(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.InputTokens != 33 {
		t.Fatalf("input tokens mismatch: got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 9 {
		t.Fatalf("output tokens mismatch: got %d", usage.OutputTokens)
	}
	if usage.TotalTokens != 42 {
		t.Fatalf("total tokens mismatch: got %d", usage.TotalTokens)
	}
}

func TestParseUsageFromResponseBody_UnknownWhenMissing(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"content":"ok"}}]}`)
	usage, err := ParseUsageFromResponseBody(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.UsageSource != "unknown" {
		t.Fatalf("usage source mismatch: got %s", usage.UsageSource)
	}
	if usage.TotalTokens != 0 {
		t.Fatalf("total tokens mismatch: got %d", usage.TotalTokens)
	}
}

func TestNormalizeUsagePagination(t *testing.T) {
	tests := []struct {
		name         string
		filter       UsageListFilter
		wantLimit    int
		wantOffset   int
	}{
		{
			name:       "default limit when non positive",
			filter:     UsageListFilter{Limit: 0, Offset: 5},
			wantLimit:  UsageListDefaultLimit,
			wantOffset: 5,
		},
		{
			name:       "clamp limit when too high",
			filter:     UsageListFilter{Limit: UsageListMaxLimit + 1, Offset: 0},
			wantLimit:  UsageListMaxLimit,
			wantOffset: 0,
		},
		{
			name:       "keep limit inside bounds",
			filter:     UsageListFilter{Limit: 42, Offset: 7},
			wantLimit:  42,
			wantOffset: 7,
		},
		{
			name:       "default offset when negative",
			filter:     UsageListFilter{Limit: 20, Offset: -10},
			wantLimit:  20,
			wantOffset: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLimit, gotOffset := normalizeUsagePagination(tc.filter)
			if gotLimit != tc.wantLimit {
				t.Fatalf("limit mismatch: got %d want %d", gotLimit, tc.wantLimit)
			}
			if gotOffset != tc.wantOffset {
				t.Fatalf("offset mismatch: got %d want %d", gotOffset, tc.wantOffset)
			}
		})
	}
}
