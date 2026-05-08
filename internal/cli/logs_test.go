package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
)

// seedLogsTestStore creates an in-memory SQLite store populated with test data.
func seedLogsTestStore(t *testing.T) storage.Store {
	t.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Now().UTC()
	requests := []storage.Request{
		{
			ID:               "abcdefgh1234",
			Timestamp:        now.Add(-1 * time.Hour),
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
			ID:               "bbcdefgh5678",
			Timestamp:        now.Add(-24 * time.Hour),
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
			ID:               "ccdefghi9012",
			Timestamp:        now.Add(-72 * time.Hour),
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

func executeLogs(t *testing.T, store storage.Store, args ...string) (string, error) {
	t.Helper()
	prev := logsStore
	logsStore = store
	t.Cleanup(func() { logsStore = prev })

	rootCmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"logs"}, args...))

	err := rootCmd.Execute()
	return buf.String(), err
}

func TestLogsDisplaysTable(t *testing.T) {
	store := seedLogsTestStore(t)
	output, err := executeLogs(t, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have a header line plus data lines.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (header + data), got %d", len(lines))
	}

	// Verify header contains expected columns.
	header := lines[0]
	for _, col := range []string{"ID", "TIMESTAMP", "PROVIDER", "MODEL", "STATUS", "IN TOKENS", "OUT TOKENS", "COST", "LATENCY"} {
		if !strings.Contains(header, col) {
			t.Errorf("header missing column %q, got: %s", col, header)
		}
	}

	// We should have 3 data rows.
	if len(lines) != 4 {
		t.Errorf("expected 4 lines (1 header + 3 data), got %d", len(lines))
	}
}

func TestLogsIDTruncatedTo8Chars(t *testing.T) {
	store := seedLogsTestStore(t)
	output, err := executeLogs(t, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The ID "abcdefgh1234" should be truncated to "abcdefgh".
	if !strings.Contains(output, "abcdefgh") {
		t.Error("expected truncated ID 'abcdefgh' in output")
	}
	if strings.Contains(output, "abcdefgh1234") {
		t.Error("ID should be truncated to 8 chars, but full ID found")
	}
}

func TestLogsProviderFilter(t *testing.T) {
	store := seedLogsTestStore(t)
	output, err := executeLogs(t, store, "--provider", "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Header + 1 anthropic row.
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (1 header + 1 data), got %d: %s", len(lines), output)
	}
	if !strings.Contains(lines[1], "anthropic") {
		t.Errorf("expected anthropic in output, got: %s", lines[1])
	}
}

func TestLogsModelFilter(t *testing.T) {
	store := seedLogsTestStore(t)
	output, err := executeLogs(t, store, "--model", "gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Header + 1 gpt-4 row.
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (1 header + 1 data), got %d: %s", len(lines), output)
	}
	if !strings.Contains(lines[1], "gpt-4") {
		t.Errorf("expected gpt-4 in output, got: %s", lines[1])
	}
}

func TestLogsStatusFilter(t *testing.T) {
	store := seedLogsTestStore(t)
	output, err := executeLogs(t, store, "--status", "429")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Header + 1 row with status 429.
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (1 header + 1 data), got %d: %s", len(lines), output)
	}
	if !strings.Contains(lines[1], "429") {
		t.Errorf("expected 429 in output, got: %s", lines[1])
	}
}

func TestLogsSinceFilter(t *testing.T) {
	store := seedLogsTestStore(t)
	// "2h" = last 2 hours, should get only the 1-hour-old request.
	output, err := executeLogs(t, store, "--since", "2h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (1 header + 1 data), got %d: %s", len(lines), output)
	}
	if !strings.Contains(lines[1], "abcdefgh") {
		t.Errorf("expected req abcdefgh in output, got: %s", lines[1])
	}
}

func TestLogsUntilFilter(t *testing.T) {
	store := seedLogsTestStore(t)
	// Until 48h ago should get only the 72h-old request.
	until := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	output, err := executeLogs(t, store, "--until", until)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (1 header + 1 data), got %d: %s", len(lines), output)
	}
	if !strings.Contains(lines[1], "ccdefghi") {
		t.Errorf("expected req ccdefghi in output, got: %s", lines[1])
	}
}

func TestLogsLimitFlag(t *testing.T) {
	store := seedLogsTestStore(t)
	output, err := executeLogs(t, store, "--limit", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Header + 1 data row.
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (1 header + 1 data), got %d: %s", len(lines), output)
	}
}

func TestLogsDefaultLimit(t *testing.T) {
	// Verify the default limit is 50 by checking the flag default.
	cmd := newLogsCommand()
	limitFlag := cmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Fatal("expected --limit flag to be defined")
	}
	if limitFlag.DefValue != "50" {
		t.Errorf("expected default limit 50, got %s", limitFlag.DefValue)
	}
}

func TestLogsEmptyResults(t *testing.T) {
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	output, execErr := executeLogs(t, store)
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	if !strings.Contains(output, "No log entries found.") {
		t.Errorf("expected empty results message, got: %q", output)
	}
}

// PATCH-0019 removed the "no store configured" error path. Production
// invocations now lazy-open a store via openStoreFromConfig when the
// test-injection seam is nil; the corresponding failure-mode test is
// deleted because asserting that contract no longer applies. Tests that
// exercise the success path use the injection seam directly.

func TestLogsSinceDurationDays(t *testing.T) {
	store := seedLogsTestStore(t)
	// "2d" = last 2 days, should get 2 requests (1h and 24h old).
	output, err := executeLogs(t, store, "--since", "2d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Header + 2 data rows.
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (1 header + 2 data), got %d: %s", len(lines), output)
	}
}

func TestLogsCombinedFilters(t *testing.T) {
	store := seedLogsTestStore(t)
	// Filter by provider=openai and since=2d - should get only gpt-4 (1h old).
	// The gpt-3.5-turbo is 72h old so excluded by since=2d.
	output, err := executeLogs(t, store, "--provider", "openai", "--since", "2d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (1 header + 1 data), got %d: %s", len(lines), output)
	}
	if !strings.Contains(lines[1], "gpt-4") {
		t.Errorf("expected gpt-4 in output, got: %s", lines[1])
	}
}
