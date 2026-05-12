package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
)

// seedTestStore creates an in-memory SQLite store and populates it with test
// data spanning several days.
func seedTestStore(t *testing.T) storage.Store {
	t.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Now().UTC()
	requests := []storage.Request{
		{
			ID:               "req-1",
			Timestamp:        now.Add(-1 * time.Hour),
			RunID:            "run-a",
			TraceID:          "trace-a",
			Provider:         "openai",
			Model:            "gpt-4",
			Method:           "POST",
			URL:              "https://api.openai.com/v1/chat/completions",
			ResponseStatus:   200,
			InputTokens:      100,
			OutputTokens:     50,
			LatencyMs:        250,
			EstimatedCostUSD: 0.0045,
		},
		{
			ID:               "req-2",
			Timestamp:        now.Add(-24 * time.Hour),
			RunID:            "run-a",
			TraceID:          "trace-b",
			Provider:         "anthropic",
			Model:            "claude-3",
			Method:           "POST",
			URL:              "https://api.anthropic.com/v1/messages",
			ResponseStatus:   200,
			InputTokens:      200,
			OutputTokens:     100,
			LatencyMs:        500,
			EstimatedCostUSD: 0.009,
		},
		{
			ID:               "req-3",
			Timestamp:        now.Add(-72 * time.Hour),
			RunID:            "run-b",
			TraceID:          "trace-a",
			Provider:         "openai",
			Model:            "gpt-3.5-turbo",
			Method:           "POST",
			URL:              "https://api.openai.com/v1/chat/completions",
			ResponseStatus:   429,
			InputTokens:      50,
			OutputTokens:     0,
			LatencyMs:        100,
			EstimatedCostUSD: 0,
		},
	}

	ctx := context.Background()
	for i := range requests {
		if err := store.SaveRequest(ctx, &requests[i]); err != nil {
			t.Fatalf("seeding request %s: %v", requests[i].ID, err)
		}
	}
	return store
}

func executeExport(t *testing.T, store storage.Store, args ...string) (string, error) {
	t.Helper()
	prev := requestsStore
	requestsStore = store
	t.Cleanup(func() { requestsStore = prev })

	rootCmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"requests", "export"}, args...))

	err := rootCmd.Execute()
	return buf.String(), err
}

func TestExportJSONLDefault(t *testing.T) {
	store := seedTestStore(t)
	output, err := executeExport(t, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}

	for i, line := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i+1, err)
		}
		// Verify expected fields exist.
		for _, field := range []string{"id", "timestamp", "run_id", "trace_id", "provider", "model", "status", "input_tokens", "output_tokens", "latency_ms", "cost"} {
			if _, ok := obj[field]; !ok {
				t.Errorf("line %d missing field %q", i+1, field)
			}
		}
	}
}

func TestExportJSONLExplicit(t *testing.T) {
	store := seedTestStore(t)
	output, err := executeExport(t, store, "--format", "jsonl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}

	// Verify each line parses as JSON.
	for i, line := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i+1, err)
		}
	}
}

func TestExportCSV(t *testing.T) {
	store := seedTestStore(t)
	output, err := executeExport(t, store, "--format", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := csv.NewReader(strings.NewReader(output))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV output: %v", err)
	}

	// Header + 3 data rows.
	if len(records) != 4 {
		t.Fatalf("expected 4 CSV rows (1 header + 3 data), got %d", len(records))
	}

	// Verify header row.
	expectedHeader := []string{"id", "timestamp", "run_id", "trace_id", "provider", "model", "status", "input_tokens", "output_tokens", "latency_ms", "cost"}
	for i, col := range expectedHeader {
		if records[0][i] != col {
			t.Errorf("header column %d: expected %q, got %q", i, col, records[0][i])
		}
	}

	// Verify each data row has correct number of columns.
	for i, row := range records[1:] {
		if len(row) != len(expectedHeader) {
			t.Errorf("data row %d: expected %d columns, got %d", i+1, len(expectedHeader), len(row))
		}
	}
}

func TestExportRunFilter(t *testing.T) {
	store := seedTestStore(t)
	output, err := executeExport(t, store, "--run", "run-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 results with --run run-a, got %d", len(lines))
	}
	for _, line := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if obj["run_id"] != "run-a" {
			t.Errorf("run_id = %v, want run-a", obj["run_id"])
		}
	}
}

func TestExportTraceFilter(t *testing.T) {
	store := seedTestStore(t)
	output, err := executeExport(t, store, "--trace", "trace-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 results with --trace trace-a, got %d", len(lines))
	}
}

func TestExportSinceFilter(t *testing.T) {
	store := seedTestStore(t)
	// "2h" means requests from the last 2 hours - should only get req-1.
	output, err := executeExport(t, store, "--since", "2h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 result with --since 2h, got %d", len(lines))
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if obj["id"] != "req-1" {
		t.Errorf("expected req-1, got %v", obj["id"])
	}
}

func TestExportUntilFilter(t *testing.T) {
	store := seedTestStore(t)
	// "48h" ago means only requests older than 48h - should get req-3.
	until := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	output, err := executeExport(t, store, "--until", until)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 result with --until 48h ago, got %d", len(lines))
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if obj["id"] != "req-3" {
		t.Errorf("expected req-3, got %v", obj["id"])
	}
}

func TestExportSinceAndUntilFilter(t *testing.T) {
	store := seedTestStore(t)
	// Between 48h ago and 2h ago - should get req-2 only.
	since := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	until := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	output, err := executeExport(t, store, "--since", since, "--until", until)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 result with --since/--until range, got %d", len(lines))
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if obj["id"] != "req-2" {
		t.Errorf("expected req-2, got %v", obj["id"])
	}
}

func TestExportSinceDurationDays(t *testing.T) {
	store := seedTestStore(t)
	// "2d" = last 2 days - should get req-1 and req-2.
	output, err := executeExport(t, store, "--since", "2d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 results with --since 2d, got %d", len(lines))
	}
}

func TestExportInvalidFormat(t *testing.T) {
	store := seedTestStore(t)
	_, err := executeExport(t, store, "--format", "xml")
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error message should mention invalid format, got: %v", err)
	}
}

func TestExportDefaultFormatIsJSONL(t *testing.T) {
	store := seedTestStore(t)
	// Run without --format flag, verify output is valid JSONL.
	output, err := executeExport(t, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON (default format should be jsonl): %v", i+1, err)
		}
	}
}

func TestExportEmptyStore(t *testing.T) {
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	output, execErr := executeExport(t, store)
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	if strings.TrimSpace(output) != "" {
		t.Errorf("expected empty output for empty store, got: %q", output)
	}
}

func TestExportCSVEmptyStore(t *testing.T) {
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	output, execErr := executeExport(t, store, "--format", "csv")
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	// CSV should still have header row.
	r := csv.NewReader(strings.NewReader(output))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 row (header only), got %d", len(records))
	}
}
