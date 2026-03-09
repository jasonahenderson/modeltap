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

// seedMetricsTestStore creates an in-memory SQLite store populated with test
// requests that generate aggregation data across multiple providers, models,
// and days.
func seedMetricsTestStore(t *testing.T) storage.Store {
	t.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()

	// Seed requests across 3 days, 2 providers, 2 models.
	entries := []struct {
		day      time.Time
		provider string
		model    string
		in       int64
		out      int64
		latency  int64
		cost     float64
		status   int
	}{
		// Day 1: today - 2 days
		{time.Now().UTC().AddDate(0, 0, -2).Truncate(24 * time.Hour).Add(10 * time.Hour), "openai", "gpt-4", 100, 50, 200, 0.005, 200},
		{time.Now().UTC().AddDate(0, 0, -2).Truncate(24 * time.Hour).Add(11 * time.Hour), "openai", "gpt-4", 150, 75, 300, 0.008, 200},
		{time.Now().UTC().AddDate(0, 0, -2).Truncate(24 * time.Hour).Add(12 * time.Hour), "anthropic", "claude-3", 200, 100, 500, 0.01, 200},
		// Day 2: today - 1 day
		{time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour).Add(9 * time.Hour), "openai", "gpt-4", 120, 60, 250, 0.006, 200},
		{time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour).Add(14 * time.Hour), "anthropic", "claude-3", 180, 90, 400, 0.009, 429},
		// Day 3: today
		{time.Now().UTC().Truncate(24 * time.Hour).Add(8 * time.Hour), "openai", "gpt-4", 130, 65, 220, 0.007, 200},
		{time.Now().UTC().Truncate(24 * time.Hour).Add(9 * time.Hour), "anthropic", "claude-3", 210, 105, 450, 0.011, 500},
	}

	for i, e := range entries {
		req := &storage.Request{
			ID:               "metric-req-" + strings.Repeat("x", 10) + string(rune('0'+i)),
			Timestamp:        e.day,
			Provider:         e.provider,
			Model:            e.model,
			Method:           "POST",
			URL:              "https://api.example.com/v1/chat",
			ResponseStatus:   e.status,
			InputTokens:      e.in,
			OutputTokens:     e.out,
			LatencyMs:        e.latency,
			EstimatedCostUSD: e.cost,
		}
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("seeding request %d: %v", i, err)
		}
	}

	return store
}

func TestMetrics_DisplaysTable(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics command returned error: %v", err)
	}

	output := buf.String()

	// Should contain table headers.
	for _, header := range []string{"PERIOD", "PROVIDER", "MODEL", "REQUESTS", "INPUT TOKENS", "OUTPUT TOKENS", "COST", "AVG LATENCY", "ERRORS"} {
		if !strings.Contains(output, header) {
			t.Errorf("expected table header %q in output, got:\n%s", header, output)
		}
	}

	// Should contain provider and model data.
	if !strings.Contains(output, "openai") {
		t.Errorf("expected 'openai' in output")
	}
	if !strings.Contains(output, "gpt-4") {
		t.Errorf("expected 'gpt-4' in output")
	}
	if !strings.Contains(output, "anthropic") {
		t.Errorf("expected 'anthropic' in output")
	}
	if !strings.Contains(output, "claude-3") {
		t.Errorf("expected 'claude-3' in output")
	}
}

func TestMetrics_SinceFlag(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "--since", "1d"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics --since returned error: %v", err)
	}

	output := buf.String()
	// With --since 1d, we should see data from today but still have output.
	if !strings.Contains(output, "PERIOD") {
		t.Errorf("expected table header in output with --since 1d")
	}
}

func TestMetrics_UntilFlag(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	// Use an RFC3339 timestamp in the past to filter out most data.
	until := time.Now().UTC().AddDate(0, 0, -1).Format(time.RFC3339)

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "--since", "30d", "--until", until})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics --until returned error: %v", err)
	}

	output := buf.String()
	// Should have some output (data from 2 days ago).
	if len(output) == 0 {
		t.Errorf("expected non-empty output with --until flag")
	}
}

func TestMetrics_GroupByProvider(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "--group-by", "provider"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics --group-by provider returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "openai") {
		t.Errorf("expected 'openai' in grouped output")
	}
	if !strings.Contains(output, "anthropic") {
		t.Errorf("expected 'anthropic' in grouped output")
	}
}

func TestMetrics_GroupByModel(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "--group-by", "model"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics --group-by model returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "gpt-4") {
		t.Errorf("expected 'gpt-4' in grouped output")
	}
	if !strings.Contains(output, "claude-3") {
		t.Errorf("expected 'claude-3' in grouped output")
	}
}

func TestMetrics_FormatJSON(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics --format json returned error: %v", err)
	}

	// Verify the output is valid JSON.
	var metrics []storage.UsageMetrics
	if err := json.Unmarshal(buf.Bytes(), &metrics); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput:\n%s", err, buf.String())
	}

	if len(metrics) == 0 {
		t.Errorf("expected at least one metric in JSON output")
	}

	// Verify fields are populated.
	for _, m := range metrics {
		if m.Period == "" {
			t.Errorf("expected non-empty Period in JSON output")
		}
		if m.Provider == "" {
			t.Errorf("expected non-empty Provider in JSON output")
		}
		if m.RequestCount == 0 {
			t.Errorf("expected non-zero RequestCount in JSON output")
		}
	}
}

func TestMetrics_FormatCSV(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "--format", "csv"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics --format csv returned error: %v", err)
	}

	// Verify the output is valid CSV.
	reader := csv.NewReader(strings.NewReader(buf.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v\nOutput:\n%s", err, buf.String())
	}

	// Should have header + at least 1 data row.
	if len(records) < 2 {
		t.Fatalf("expected at least 2 CSV rows (header + data), got %d", len(records))
	}

	// Verify header.
	expectedHeaders := []string{"period", "provider", "model", "requests", "input_tokens", "output_tokens", "cost", "avg_latency_ms", "errors"}
	header := records[0]
	if len(header) != len(expectedHeaders) {
		t.Fatalf("expected %d CSV columns, got %d", len(expectedHeaders), len(header))
	}
	for i, h := range expectedHeaders {
		if header[i] != h {
			t.Errorf("CSV header[%d]: got %q, want %q", i, header[i], h)
		}
	}

	// Verify data rows have the right number of columns.
	for i, row := range records[1:] {
		if len(row) != len(expectedHeaders) {
			t.Errorf("CSV row %d has %d columns, want %d", i+1, len(row), len(expectedHeaders))
		}
	}
}

func TestMetrics_Rebuild(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "rebuild"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics rebuild returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Metrics rebuilt successfully") {
		t.Errorf("expected success message, got: %s", output)
	}

	// Verify metrics are still queryable after rebuild.
	ctx := context.Background()
	daily, err := store.QueryDailyMetrics(ctx, storage.MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryDailyMetrics after rebuild: %v", err)
	}
	if len(daily) == 0 {
		t.Errorf("expected metrics data after rebuild")
	}
}

func TestMetrics_RebuildNoStore(t *testing.T) {
	metricsStore = nil

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "rebuild"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when no store configured")
	}
	if !strings.Contains(err.Error(), "no store configured") {
		t.Errorf("expected 'no store configured' error, got: %v", err)
	}
}

func TestMetrics_NoStore(t *testing.T) {
	metricsStore = nil

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when no store configured")
	}
	if !strings.Contains(err.Error(), "no store configured") {
		t.Errorf("expected 'no store configured' error, got: %v", err)
	}
}

func TestMetrics_InvalidGroupBy(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "--group-by", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for invalid --group-by")
	}
	if !strings.Contains(err.Error(), "invalid --group-by") {
		t.Errorf("expected 'invalid --group-by' error, got: %v", err)
	}
}

func TestMetrics_EmptyResults(t *testing.T) {
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics with empty store returned error: %v", err)
	}

	if !strings.Contains(buf.String(), "No metrics found") {
		t.Errorf("expected 'No metrics found' message, got: %s", buf.String())
	}
}

func TestMetrics_GroupByHour(t *testing.T) {
	store := seedMetricsTestStore(t)
	metricsStore = store
	t.Cleanup(func() { metricsStore = nil })

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"metrics", "--group-by", "hour"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("metrics --group-by hour returned error: %v", err)
	}

	output := buf.String()
	// Hourly periods contain T and :00:00Z pattern.
	if !strings.Contains(output, "T") {
		t.Errorf("expected hourly period format (containing 'T') in output, got:\n%s", output)
	}
}
