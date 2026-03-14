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
