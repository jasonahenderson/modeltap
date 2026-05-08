package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
)

// seedShowTestStore creates an in-memory SQLite store with a single detailed
// request for show command testing.
func seedShowTestStore(t *testing.T) storage.Store {
	t.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	req := &storage.Request{
		ID:               "show-req-1",
		Timestamp:        time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
		Provider:         "openai",
		Model:            "gpt-4",
		Method:           "POST",
		URL:              "https://api.openai.com/v1/chat/completions",
		RequestHeaders:   `{"Content-Type":"application/json","Authorization":"Bearer sk-***"}`,
		RequestBody:      `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`,
		ResponseStatus:   200,
		ResponseHeaders:  `{"Content-Type":"application/json"}`,
		ResponseBody:     `{"id":"chatcmpl-123","choices":[{"message":{"content":"Hi there!"}}]}`,
		InputTokens:      100,
		OutputTokens:     50,
		LatencyMs:        250,
		EstimatedCostUSD: 0.0045,
	}

	ctx := context.Background()
	if err := store.SaveRequest(ctx, req); err != nil {
		t.Fatalf("seeding request: %v", err)
	}
	return store
}

func executeShow(t *testing.T, store storage.Store, args ...string) (string, error) {
	t.Helper()
	prev := showStore
	showStore = store
	t.Cleanup(func() { showStore = prev })

	rootCmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"show"}, args...))

	err := rootCmd.Execute()
	return buf.String(), err
}

func TestShowDisplaysFullDetail(t *testing.T) {
	store := seedShowTestStore(t)
	output, err := executeShow(t, store, "show-req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify header section fields.
	expected := []string{
		"show-req-1",
		"openai",
		"gpt-4",
		"200",
		"250ms",
		"100 input / 50 output",
		"$0.0045",
	}
	for _, s := range expected {
		if !strings.Contains(output, s) {
			t.Errorf("output missing expected string %q\nOutput:\n%s", s, output)
		}
	}

	// Verify request section.
	if !strings.Contains(output, "POST") {
		t.Errorf("output missing method POST")
	}
	if !strings.Contains(output, "https://api.openai.com/v1/chat/completions") {
		t.Errorf("output missing URL")
	}

	// Verify section headers.
	for _, section := range []string{"=== Request Detail ===", "--- Request ---", "--- Response ---"} {
		if !strings.Contains(output, section) {
			t.Errorf("output missing section header %q", section)
		}
	}
}

func TestShowPrettyPrintsJSON(t *testing.T) {
	store := seedShowTestStore(t)
	output, err := executeShow(t, store, "show-req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pretty-printed JSON has indentation. Check for indented keys
	// that would only appear in pretty-printed output.
	if !strings.Contains(output, "\"model\": \"gpt-4\"") {
		t.Errorf("request body does not appear pretty-printed\nOutput:\n%s", output)
	}
	if !strings.Contains(output, "\"choices\": [") {
		t.Errorf("response body does not appear pretty-printed\nOutput:\n%s", output)
	}
}

func TestShowNotFound(t *testing.T) {
	store := seedShowTestStore(t)
	output, err := executeShow(t, store, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for missing ID, got nil")
	}
	if !strings.Contains(output, "Request nonexistent-id not found") {
		t.Errorf("expected 'not found' message, got: %s", output)
	}
}

func TestShowDisplaysMetadata(t *testing.T) {
	store := seedShowTestStore(t)
	output, err := executeShow(t, store, "show-req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all metadata fields are present.
	metadata := map[string]string{
		"Tokens":    "100 input / 50 output",
		"Cost":      "$0.0045",
		"Latency":   "250ms",
		"Provider":  "openai",
		"Model":     "gpt-4",
		"Timestamp": "2026-03-08T12:00:00Z",
	}
	for label, value := range metadata {
		if !strings.Contains(output, label) {
			t.Errorf("output missing metadata label %q", label)
		}
		if !strings.Contains(output, value) {
			t.Errorf("output missing metadata value %q for %s", value, label)
		}
	}
}

func TestShowDisplaysHeaders(t *testing.T) {
	store := seedShowTestStore(t)
	output, err := executeShow(t, store, "show-req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Content-Type: application/json") {
		t.Errorf("output missing formatted header\nOutput:\n%s", output)
	}
}

// PATCH-0019 removed the "no store configured" error path. Production
// invocations now lazy-open a store via openStoreFromConfig when the
// test-injection seam is nil; the corresponding failure-mode test is
// deleted because asserting that contract no longer applies.
