package storage

import (
	"context"
	"testing"
	"time"
)

func TestSaveRequest_UpdatesAggregationTables(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	req := makeRequest(func(r *Request) {
		r.Timestamp = time.Date(2026, 3, 8, 14, 30, 0, 0, time.UTC)
		r.Provider = "anthropic"
		r.Model = "claude-sonnet-4-20250514"
		r.InputTokens = 100
		r.OutputTokens = 50
		r.LatencyMs = 250
		r.EstimatedCostUSD = 0.005
		r.ResponseStatus = 200
	})
	if err := store.SaveRequest(ctx, req); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	// Check hourly aggregation.
	hourly, err := store.QueryHourlyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics: %v", err)
	}
	if len(hourly) != 1 {
		t.Fatalf("expected 1 hourly row, got %d", len(hourly))
	}
	h := hourly[0]
	if h.Period != "2026-03-08T14:00:00Z" {
		t.Errorf("hourly period: got %q, want %q", h.Period, "2026-03-08T14:00:00Z")
	}
	if h.RequestCount != 1 {
		t.Errorf("hourly request_count: got %d, want 1", h.RequestCount)
	}
	if h.InputTokens != 100 {
		t.Errorf("hourly input_tokens: got %d, want 100", h.InputTokens)
	}
	if h.OutputTokens != 50 {
		t.Errorf("hourly output_tokens: got %d, want 50", h.OutputTokens)
	}
	if h.ErrorCount != 0 {
		t.Errorf("hourly error_count: got %d, want 0", h.ErrorCount)
	}

	// Check daily aggregation.
	daily, err := store.QueryDailyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryDailyMetrics: %v", err)
	}
	if len(daily) != 1 {
		t.Fatalf("expected 1 daily row, got %d", len(daily))
	}
	d := daily[0]
	if d.Period != "2026-03-08" {
		t.Errorf("daily period: got %q, want %q", d.Period, "2026-03-08")
	}
	if d.RequestCount != 1 {
		t.Errorf("daily request_count: got %d, want 1", d.RequestCount)
	}
}

func TestAggregation_SameHourSameModel(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 3, 8, 14, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		req := makeRequest(func(r *Request) {
			r.Timestamp = base.Add(time.Duration(i) * time.Minute)
			r.Provider = "anthropic"
			r.Model = "claude-sonnet-4-20250514"
			r.InputTokens = 100
			r.OutputTokens = 50
			r.LatencyMs = 200
			r.EstimatedCostUSD = 0.01
			r.ResponseStatus = 200
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest %d: %v", i, err)
		}
	}

	hourly, err := store.QueryHourlyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics: %v", err)
	}
	if len(hourly) != 1 {
		t.Fatalf("expected 1 hourly row, got %d", len(hourly))
	}
	h := hourly[0]
	if h.RequestCount != 3 {
		t.Errorf("request_count: got %d, want 3", h.RequestCount)
	}
	if h.InputTokens != 300 {
		t.Errorf("input_tokens: got %d, want 300", h.InputTokens)
	}
	if h.OutputTokens != 150 {
		t.Errorf("output_tokens: got %d, want 150", h.OutputTokens)
	}
	if h.AvgLatencyMs != 200 {
		t.Errorf("avg_latency_ms: got %d, want 200", h.AvgLatencyMs)
	}
}

func TestAggregation_DifferentModelsProviders(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ts := time.Date(2026, 3, 8, 14, 0, 0, 0, time.UTC)

	configs := []struct {
		provider string
		model    string
	}{
		{"anthropic", "claude-sonnet-4-20250514"},
		{"anthropic", "claude-opus-4-20250514"},
		{"openai", "gpt-4"},
	}

	for _, c := range configs {
		req := makeRequest(func(r *Request) {
			r.Timestamp = ts
			r.Provider = c.provider
			r.Model = c.model
			r.InputTokens = 100
			r.OutputTokens = 50
			r.ResponseStatus = 200
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	hourly, err := store.QueryHourlyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics: %v", err)
	}
	if len(hourly) != 3 {
		t.Errorf("expected 3 hourly rows (one per provider/model), got %d", len(hourly))
	}

	daily, err := store.QueryDailyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryDailyMetrics: %v", err)
	}
	if len(daily) != 3 {
		t.Errorf("expected 3 daily rows, got %d", len(daily))
	}
}

func TestQueryHourlyMetrics_TimeFilters(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Insert requests across 3 different hours.
	hours := []time.Time{
		time.Date(2026, 3, 8, 10, 30, 0, 0, time.UTC),
		time.Date(2026, 3, 8, 12, 30, 0, 0, time.UTC),
		time.Date(2026, 3, 8, 14, 30, 0, 0, time.UTC),
	}
	for _, ts := range hours {
		req := makeRequest(func(r *Request) {
			r.Timestamp = ts
			r.Provider = "anthropic"
			r.Model = "claude-sonnet-4-20250514"
			r.ResponseStatus = 200
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	// Query all.
	all, err := store.QueryHourlyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics (all): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 hourly rows, got %d", len(all))
	}

	// Query with Since filter.
	since := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	filtered, err := store.QueryHourlyMetrics(ctx, MetricsFilter{Since: &since})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics (since): %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 hourly rows with since filter, got %d", len(filtered))
	}

	// Query with Until filter.
	until := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	filtered, err = store.QueryHourlyMetrics(ctx, MetricsFilter{Until: &until})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics (until): %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 hourly rows with until filter, got %d", len(filtered))
	}

	// Query with Since and Until.
	filtered, err = store.QueryHourlyMetrics(ctx, MetricsFilter{Since: &since, Until: &until})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics (range): %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1 hourly row with range filter, got %d", len(filtered))
	}
}

func TestQueryDailyMetrics_ProviderModelFilters(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ts := time.Date(2026, 3, 8, 14, 0, 0, 0, time.UTC)

	configs := []struct {
		provider string
		model    string
	}{
		{"anthropic", "claude-sonnet-4-20250514"},
		{"anthropic", "claude-opus-4-20250514"},
		{"openai", "gpt-4"},
	}

	for _, c := range configs {
		req := makeRequest(func(r *Request) {
			r.Timestamp = ts
			r.Provider = c.provider
			r.Model = c.model
			r.ResponseStatus = 200
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	// Filter by provider.
	results, err := store.QueryDailyMetrics(ctx, MetricsFilter{Provider: "anthropic"})
	if err != nil {
		t.Fatalf("QueryDailyMetrics (provider): %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 daily rows for anthropic, got %d", len(results))
	}

	// Filter by model.
	results, err = store.QueryDailyMetrics(ctx, MetricsFilter{Model: "gpt-4"})
	if err != nil {
		t.Fatalf("QueryDailyMetrics (model): %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 daily row for gpt-4, got %d", len(results))
	}

	// Filter by both.
	results, err = store.QueryDailyMetrics(ctx, MetricsFilter{Provider: "anthropic", Model: "claude-sonnet-4-20250514"})
	if err != nil {
		t.Fatalf("QueryDailyMetrics (provider+model): %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 daily row, got %d", len(results))
	}
}

func TestRebuildMetrics(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ts := time.Date(2026, 3, 8, 14, 30, 0, 0, time.UTC)

	// Insert 3 requests.
	for i := 0; i < 3; i++ {
		req := makeRequest(func(r *Request) {
			r.Timestamp = ts.Add(time.Duration(i) * time.Minute)
			r.Provider = "anthropic"
			r.Model = "claude-sonnet-4-20250514"
			r.InputTokens = 100
			r.OutputTokens = 50
			r.LatencyMs = 200
			r.EstimatedCostUSD = 0.01
			r.ResponseStatus = 200
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	// Verify initial state.
	hourly, err := store.QueryHourlyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics: %v", err)
	}
	if len(hourly) != 1 || hourly[0].RequestCount != 3 {
		t.Fatalf("expected 1 row with count 3, got %d rows", len(hourly))
	}

	// Corrupt the aggregation by manually updating.
	_, err = store.db.ExecContext(ctx, "UPDATE hourly_usage SET request_count = 999")
	if err != nil {
		t.Fatalf("manual update: %v", err)
	}

	// Rebuild.
	if err := store.RebuildMetrics(ctx); err != nil {
		t.Fatalf("RebuildMetrics: %v", err)
	}

	// Verify rebuilt data.
	hourly, err = store.QueryHourlyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics after rebuild: %v", err)
	}
	if len(hourly) != 1 {
		t.Fatalf("expected 1 hourly row after rebuild, got %d", len(hourly))
	}
	if hourly[0].RequestCount != 3 {
		t.Errorf("request_count after rebuild: got %d, want 3", hourly[0].RequestCount)
	}
	if hourly[0].InputTokens != 300 {
		t.Errorf("input_tokens after rebuild: got %d, want 300", hourly[0].InputTokens)
	}

	// Verify daily was also rebuilt.
	daily, err := store.QueryDailyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryDailyMetrics after rebuild: %v", err)
	}
	if len(daily) != 1 {
		t.Fatalf("expected 1 daily row after rebuild, got %d", len(daily))
	}
	if daily[0].RequestCount != 3 {
		t.Errorf("daily request_count after rebuild: got %d, want 3", daily[0].RequestCount)
	}
}

func TestErrorCount_IncrementForNon2xx(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ts := time.Date(2026, 3, 8, 14, 0, 0, 0, time.UTC)

	statuses := []int{200, 201, 400, 429, 500, 200}
	for i, status := range statuses {
		req := makeRequest(func(r *Request) {
			r.Timestamp = ts.Add(time.Duration(i) * time.Minute)
			r.Provider = "anthropic"
			r.Model = "claude-sonnet-4-20250514"
			r.ResponseStatus = status
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest (status %d): %v", status, err)
		}
	}

	hourly, err := store.QueryHourlyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics: %v", err)
	}
	if len(hourly) != 1 {
		t.Fatalf("expected 1 hourly row, got %d", len(hourly))
	}
	if hourly[0].ErrorCount != 3 {
		t.Errorf("error_count: got %d, want 3 (for status 400, 429, 500)", hourly[0].ErrorCount)
	}
	if hourly[0].RequestCount != 6 {
		t.Errorf("request_count: got %d, want 6", hourly[0].RequestCount)
	}

	// Verify daily too.
	daily, err := store.QueryDailyMetrics(ctx, MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryDailyMetrics: %v", err)
	}
	if len(daily) != 1 {
		t.Fatalf("expected 1 daily row, got %d", len(daily))
	}
	if daily[0].ErrorCount != 3 {
		t.Errorf("daily error_count: got %d, want 3", daily[0].ErrorCount)
	}
}

func TestStoreInterfaceCompliance_Metrics(t *testing.T) {
	// Compile-time check that SQLiteStore still implements Store.
	var _ Store = (*SQLiteStore)(nil)
}
