package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// seedStore creates an in-memory SQLite store and populates it with test data.
func seedStore(t *testing.T) storage.Store {
	t.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		provider := "anthropic"
		model := "claude-3-opus"
		status := 200
		if i%3 == 0 {
			provider = "openai"
			model = "gpt-4"
		}
		if i == 5 {
			status = 500
		}

		req := &storage.Request{
			ID:               fmt.Sprintf("req-%03d", i),
			Timestamp:        base.Add(time.Duration(i) * time.Hour),
			Provider:         provider,
			Model:            model,
			Method:           "POST",
			URL:              fmt.Sprintf("https://api.%s.com/v1/messages", provider),
			RequestHeaders:   `{"Content-Type":"application/json"}`,
			RequestBody:      fmt.Sprintf(`{"prompt":"test %d"}`, i),
			ResponseStatus:   status,
			ResponseHeaders:  `{"Content-Type":"application/json"}`,
			ResponseBody:     fmt.Sprintf(`{"response":"answer %d"}`, i),
			InputTokens:      int64(100 + i*10),
			OutputTokens:     int64(50 + i*5),
			LatencyMs:        int64(200 + i*20),
			EstimatedCostUSD: float64(i) * 0.01,
		}
		if err := store.SaveRequest(ctx, req); err != nil {
			t.Fatalf("saving test request %d: %v", i, err)
		}
	}

	return store
}

func testConfig() *config.Config {
	return &config.Config{
		Port:          8080,
		Upstream:      "https://api.anthropic.com",
		DBPath:        ":memory:",
		RetentionDays: 30,
		Dashboard: config.DashboardConfig{
			Enabled: true,
			Port:    8081,
			Bind:    "127.0.0.1",
		},
	}
}

func TestLogsEndpoint(t *testing.T) {
	store := seedStore(t)
	handler := NewAPIHandler(store, testConfig())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	t.Run("returns JSON with data and total", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json, got %s", ct)
		}

		var resp struct {
			Data  []json.RawMessage `json:"data"`
			Total int64             `json:"total"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if resp.Total != 10 {
			t.Fatalf("expected total 10, got %d", resp.Total)
		}
		if len(resp.Data) != 10 {
			t.Fatalf("expected 10 items, got %d", len(resp.Data))
		}
	})

	t.Run("pagination with limit and offset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs?limit=3&offset=2", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data  []json.RawMessage `json:"data"`
			Total int64             `json:"total"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		// Total should still be the full count, not the page count
		if resp.Total != 10 {
			t.Fatalf("expected total 10, got %d", resp.Total)
		}
		if len(resp.Data) != 3 {
			t.Fatalf("expected 3 items, got %d", len(resp.Data))
		}
	})

	t.Run("default limit is 50", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		// With only 10 records, we should get all 10 even with default limit of 50
		var resp struct {
			Data  []json.RawMessage `json:"data"`
			Total int64             `json:"total"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(resp.Data) != 10 {
			t.Fatalf("expected 10 items (all), got %d", len(resp.Data))
		}
	})

	t.Run("filter by provider", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs?provider=openai", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		var resp struct {
			Data  []map[string]any `json:"data"`
			Total int64            `json:"total"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		// indices 0,3,6,9 are openai (i%3==0)
		if resp.Total != 4 {
			t.Fatalf("expected total 4 openai requests, got %d", resp.Total)
		}
		for _, item := range resp.Data {
			if item["provider"] != "openai" {
				t.Fatalf("expected provider openai, got %v", item["provider"])
			}
		}
	})

	t.Run("filter by model", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs?model=gpt-4", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		var resp struct {
			Data  []map[string]any `json:"data"`
			Total int64            `json:"total"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if resp.Total != 4 {
			t.Fatalf("expected total 4 gpt-4 requests, got %d", resp.Total)
		}
	})

	t.Run("filter by time range", func(t *testing.T) {
		since := "2026-03-01T12:00:00Z"
		until := "2026-03-01T16:00:00Z"
		req := httptest.NewRequest("GET", "/api/logs?since="+since+"&until="+until, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		var resp struct {
			Data  []map[string]any `json:"data"`
			Total int64            `json:"total"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		// hours 12,13,14,15,16 => indices 2,3,4,5,6
		if resp.Total != 5 {
			t.Fatalf("expected total 5, got %d", resp.Total)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs?status=500", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		var resp struct {
			Data  []map[string]any `json:"data"`
			Total int64            `json:"total"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if resp.Total != 1 {
			t.Fatalf("expected total 1 error request, got %d", resp.Total)
		}
	})

	t.Run("invalid limit returns 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs?limit=abc", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid since returns 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs?since=not-a-date", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestLogsDetailEndpoint(t *testing.T) {
	store := seedStore(t)
	handler := NewAPIHandler(store, testConfig())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	t.Run("returns full request detail", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs/req-001", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json, got %s", ct)
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		if resp["id"] != "req-001" {
			t.Fatalf("expected id req-001, got %v", resp["id"])
		}
		if resp["provider"] != "anthropic" {
			t.Fatalf("expected provider anthropic, got %v", resp["provider"])
		}
		// Verify full body fields are present
		if _, ok := resp["request_body"]; !ok {
			t.Fatal("expected request_body in response")
		}
		if _, ok := resp["response_body"]; !ok {
			t.Fatal("expected response_body in response")
		}
		if _, ok := resp["request_headers"]; !ok {
			t.Fatal("expected request_headers in response")
		}
		if _, ok := resp["response_headers"]; !ok {
			t.Fatal("expected response_headers in response")
		}
	})

	t.Run("returns 404 for unknown ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs/nonexistent-id", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json, got %s", ct)
		}
	})
}

func TestMetricsEndpoint(t *testing.T) {
	store := seedStore(t)
	handler := NewAPIHandler(store, testConfig())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	t.Run("returns metrics with default group_by", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/metrics", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json, got %s", ct)
		}

		var resp struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(resp.Data) == 0 {
			t.Fatal("expected non-empty metrics data")
		}
	})

	t.Run("group_by hour", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/metrics?group_by=hour", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(resp.Data) == 0 {
			t.Fatal("expected non-empty hourly metrics")
		}
	})

	t.Run("group_by day", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/metrics?group_by=day", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(resp.Data) == 0 {
			t.Fatal("expected non-empty daily metrics")
		}
	})

	t.Run("with time filter", func(t *testing.T) {
		since := "2026-03-01T12:00:00Z"
		until := "2026-03-01T16:00:00Z"
		req := httptest.NewRequest("GET", "/api/metrics?group_by=hour&since="+since+"&until="+until, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(resp.Data) == 0 {
			t.Fatal("expected non-empty filtered metrics")
		}
	})

	t.Run("invalid group_by returns 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/metrics?group_by=invalid", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid since returns 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/metrics?since=bad-date", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestStatusEndpoint(t *testing.T) {
	store := seedStore(t)
	cfg := testConfig()
	handler := NewAPIHandler(store, cfg)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var resp struct {
		Proxy struct {
			Port     int    `json:"port"`
			Upstream string `json:"upstream"`
		} `json:"proxy"`
		Database struct {
			Records int64 `json:"records"`
		} `json:"database"`
		Retention struct {
			Days int `json:"days"`
		} `json:"retention"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.Proxy.Port != 8080 {
		t.Fatalf("expected proxy port 8080, got %d", resp.Proxy.Port)
	}
	if resp.Proxy.Upstream != "https://api.anthropic.com" {
		t.Fatalf("expected upstream https://api.anthropic.com, got %s", resp.Proxy.Upstream)
	}
	if resp.Database.Records != 10 {
		t.Fatalf("expected 10 records, got %d", resp.Database.Records)
	}
	if resp.Retention.Days != 30 {
		t.Fatalf("expected retention 30 days, got %d", resp.Retention.Days)
	}
}
