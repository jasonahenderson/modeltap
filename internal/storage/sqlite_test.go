package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// newTestStore creates an in-memory SQLite store for testing.
func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore(:memory:): %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// makeRequest returns a sample Request with the given overrides applied.
func makeRequest(opts ...func(*Request)) *Request {
	r := &Request{
		ID:               uuid.New().String(),
		Timestamp:        time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
		Provider:         "anthropic",
		Model:            "claude-sonnet-4-20250514",
		Method:           "POST",
		URL:              "https://api.anthropic.com/v1/messages",
		RequestHeaders:   `{"Content-Type":"application/json"}`,
		RequestBody:      `{"model":"claude-sonnet-4-20250514","messages":[]}`,
		ResponseStatus:   200,
		ResponseHeaders:  `{"Content-Type":"application/json"}`,
		ResponseBody:     `{"id":"msg_123","content":[]}`,
		InputTokens:      100,
		OutputTokens:     50,
		LatencyMs:        250,
		EstimatedCostUSD: 0.0045,
	}
	for _, fn := range opts {
		fn(r)
	}
	return r
}

func TestSaveAndGetRequest(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	req := makeRequest()
	if err := store.SaveRequest(ctx, req); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	got, err := store.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if got == nil {
		t.Fatal("GetRequest returned nil")
	}

	// Verify all fields round-trip.
	if got.ID != req.ID {
		t.Errorf("ID: got %q, want %q", got.ID, req.ID)
	}
	if !got.Timestamp.Equal(req.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, req.Timestamp)
	}
	if got.Provider != req.Provider {
		t.Errorf("Provider: got %q, want %q", got.Provider, req.Provider)
	}
	if got.Model != req.Model {
		t.Errorf("Model: got %q, want %q", got.Model, req.Model)
	}
	if got.Method != req.Method {
		t.Errorf("Method: got %q, want %q", got.Method, req.Method)
	}
	if got.URL != req.URL {
		t.Errorf("URL: got %q, want %q", got.URL, req.URL)
	}
	if got.RequestHeaders != req.RequestHeaders {
		t.Errorf("RequestHeaders: got %q, want %q", got.RequestHeaders, req.RequestHeaders)
	}
	if got.RequestBody != req.RequestBody {
		t.Errorf("RequestBody: got %q, want %q", got.RequestBody, req.RequestBody)
	}
	if got.ResponseStatus != req.ResponseStatus {
		t.Errorf("ResponseStatus: got %d, want %d", got.ResponseStatus, req.ResponseStatus)
	}
	if got.ResponseHeaders != req.ResponseHeaders {
		t.Errorf("ResponseHeaders: got %q, want %q", got.ResponseHeaders, req.ResponseHeaders)
	}
	if got.ResponseBody != req.ResponseBody {
		t.Errorf("ResponseBody: got %q, want %q", got.ResponseBody, req.ResponseBody)
	}
	if got.InputTokens != req.InputTokens {
		t.Errorf("InputTokens: got %d, want %d", got.InputTokens, req.InputTokens)
	}
	if got.OutputTokens != req.OutputTokens {
		t.Errorf("OutputTokens: got %d, want %d", got.OutputTokens, req.OutputTokens)
	}
	if got.LatencyMs != req.LatencyMs {
		t.Errorf("LatencyMs: got %d, want %d", got.LatencyMs, req.LatencyMs)
	}
	if got.EstimatedCostUSD != req.EstimatedCostUSD {
		t.Errorf("EstimatedCostUSD: got %f, want %f", got.EstimatedCostUSD, req.EstimatedCostUSD)
	}
}

func TestSaveRequest_GeneratesID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	req := makeRequest(func(r *Request) { r.ID = "" })
	if err := store.SaveRequest(ctx, req); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	if req.ID == "" {
		t.Fatal("expected ID to be generated, got empty string")
	}
	// Verify it's a valid UUID.
	if _, err := uuid.Parse(req.ID); err != nil {
		t.Errorf("generated ID %q is not a valid UUID: %v", req.ID, err)
	}
}

func TestGetRequest_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	got, err := store.GetRequest(ctx, "nonexistent-id")
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent request, got %+v", got)
	}
}

func TestListRequests_Filters(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Insert diverse requests.
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	requests := []*Request{
		makeRequest(func(r *Request) {
			r.Provider = "anthropic"
			r.Model = "claude-sonnet-4-20250514"
			r.Timestamp = now
			r.ResponseStatus = 200
		}),
		makeRequest(func(r *Request) {
			r.Provider = "anthropic"
			r.Model = "claude-opus-4-20250514"
			r.Timestamp = now.Add(-1 * time.Hour)
			r.ResponseStatus = 200
		}),
		makeRequest(func(r *Request) {
			r.Provider = "openai"
			r.Model = "gpt-4"
			r.Timestamp = now.Add(-2 * time.Hour)
			r.ResponseStatus = 429
		}),
		makeRequest(func(r *Request) {
			r.Provider = "openai"
			r.Model = "gpt-4"
			r.Timestamp = now.Add(-3 * time.Hour)
			r.ResponseStatus = 200
		}),
	}
	for _, req := range requests {
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	status429 := 429
	status200 := 200
	since := now.Add(-90 * time.Minute)
	until := now.Add(-30 * time.Minute)

	tests := []struct {
		name      string
		filter    ListFilter
		wantCount int
	}{
		{
			name:      "no filter",
			filter:    ListFilter{},
			wantCount: 4,
		},
		{
			name:      "filter by provider anthropic",
			filter:    ListFilter{Provider: "anthropic"},
			wantCount: 2,
		},
		{
			name:      "filter by provider openai",
			filter:    ListFilter{Provider: "openai"},
			wantCount: 2,
		},
		{
			name:      "filter by model gpt-4",
			filter:    ListFilter{Model: "gpt-4"},
			wantCount: 2,
		},
		{
			name:      "filter by model claude-sonnet-4-20250514",
			filter:    ListFilter{Model: "claude-sonnet-4-20250514"},
			wantCount: 1,
		},
		{
			name:      "filter by status code 429",
			filter:    ListFilter{StatusCode: &status429},
			wantCount: 1,
		},
		{
			name:      "filter by status code 200",
			filter:    ListFilter{StatusCode: &status200},
			wantCount: 3,
		},
		{
			name:      "filter by since",
			filter:    ListFilter{Since: &since},
			wantCount: 2,
		},
		{
			name:      "filter by until",
			filter:    ListFilter{Until: &until},
			wantCount: 3,
		},
		{
			name:      "filter by time range",
			filter:    ListFilter{Since: &since, Until: &until},
			wantCount: 1,
		},
		{
			name:      "combined provider and model",
			filter:    ListFilter{Provider: "openai", Model: "gpt-4"},
			wantCount: 2,
		},
		{
			name:      "combined provider and status",
			filter:    ListFilter{Provider: "openai", StatusCode: &status429},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.ListRequests(ctx, tt.filter)
			if err != nil {
				t.Fatalf("ListRequests: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(results), tt.wantCount)
			}
		})
	}
}

func TestListRequests_OrderByTimestampDesc(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		req := makeRequest(func(r *Request) {
			r.Timestamp = now.Add(time.Duration(i) * time.Hour)
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	results, err := store.ListRequests(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// Should be newest first.
	for i := 1; i < len(results); i++ {
		if results[i].Timestamp.After(results[i-1].Timestamp) {
			t.Errorf("results not in descending order: [%d]=%v > [%d]=%v",
				i, results[i].Timestamp, i-1, results[i-1].Timestamp)
		}
	}
}

func TestListRequests_Pagination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		req := makeRequest(func(r *Request) {
			r.Timestamp = now.Add(time.Duration(i) * time.Minute)
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	tests := []struct {
		name      string
		limit     int
		offset    int
		wantCount int
	}{
		{name: "first page", limit: 3, offset: 0, wantCount: 3},
		{name: "second page", limit: 3, offset: 3, wantCount: 3},
		{name: "third page", limit: 3, offset: 6, wantCount: 3},
		{name: "last page partial", limit: 3, offset: 9, wantCount: 1},
		{name: "beyond end", limit: 3, offset: 15, wantCount: 0},
		{name: "limit only", limit: 5, offset: 0, wantCount: 5},
		{name: "no limit", limit: 0, offset: 0, wantCount: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.ListRequests(ctx, ListFilter{
				Limit:  tt.limit,
				Offset: tt.offset,
			})
			if err != nil {
				t.Fatalf("ListRequests: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(results), tt.wantCount)
			}
		})
	}

	// Verify that pages don't overlap: page 1 and page 2 have different IDs.
	page1, _ := store.ListRequests(ctx, ListFilter{Limit: 3, Offset: 0})
	page2, _ := store.ListRequests(ctx, ListFilter{Limit: 3, Offset: 3})
	for _, p1 := range page1 {
		for _, p2 := range page2 {
			if p1.ID == p2.ID {
				t.Errorf("overlapping request %s between page 1 and page 2", p1.ID)
			}
		}
	}
}

func TestCountRequests(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Empty store.
	count, err := store.CountRequests(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("CountRequests: %v", err)
	}
	if count != 0 {
		t.Errorf("got count %d, want 0", count)
	}

	// Insert some requests.
	for i := 0; i < 5; i++ {
		provider := "anthropic"
		if i%2 == 0 {
			provider = "openai"
		}
		req := makeRequest(func(r *Request) { r.Provider = provider })
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	tests := []struct {
		name   string
		filter ListFilter
		want   int64
	}{
		{name: "all", filter: ListFilter{}, want: 5},
		{name: "anthropic", filter: ListFilter{Provider: "anthropic"}, want: 2},
		{name: "openai", filter: ListFilter{Provider: "openai"}, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := store.CountRequests(ctx, tt.filter)
			if err != nil {
				t.Fatalf("CountRequests: %v", err)
			}
			if count != tt.want {
				t.Errorf("got %d, want %d", count, tt.want)
			}
		})
	}
}

func TestDeleteBefore(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		req := makeRequest(func(r *Request) {
			r.Timestamp = now.Add(time.Duration(i) * time.Hour)
		})
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("SaveRequest: %v", err)
		}
	}

	// Delete requests before now + 2h (should delete the first 2: 0h, 1h).
	cutoff := now.Add(2 * time.Hour)
	deleted, err := store.DeleteBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted %d, want 2", deleted)
	}

	// Verify remaining count.
	count, err := store.CountRequests(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("CountRequests: %v", err)
	}
	if count != 3 {
		t.Errorf("remaining count %d, want 3", count)
	}
}

func TestWALModeEnabled(t *testing.T) {
	store := newTestStore(t)

	var mode string
	err := store.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	// In-memory databases may report "memory" instead of "wal",
	// so we test with a file-based database as well.
	if mode != "memory" && mode != "wal" {
		t.Errorf("journal_mode: got %q, want 'wal' or 'memory'", mode)
	}

	// Test with a temp file-based database to confirm WAL.
	tmpDir := t.TempDir()
	fileStore, err := NewSQLiteStore(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLiteStore(file): %v", err)
	}
	defer fileStore.Close()

	err = fileStore.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode (file): %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode (file): got %q, want 'wal'", mode)
	}
}

func TestStoreInterfaceCompliance(t *testing.T) {
	// Compile-time check that SQLiteStore implements Store.
	var _ Store = (*SQLiteStore)(nil)
}
