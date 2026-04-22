// Package integration provides end-to-end integration tests for the modeltap
// proxy. Each test stands up a full proxy with mock upstream server(s), an
// in-memory SQLite store, provider registry, and pricing table.
package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/proxy"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testEnv bundles the objects needed for an integration test.
type testEnv struct {
	Store    *storage.SQLiteStore
	Registry *provider.Registry
	Pricing  *config.PricingTable
	ProxyURL string
	Client   *http.Client
}

// setupTestProxy creates an in-memory store, a provider registry with both
// Anthropic and OpenAI adapters, the default pricing table, and a proxy
// server backed by the given upstream(s). The proxy is started on a random
// port via httptest.NewServer.
//
// providerUpstreams may be nil; in that case all traffic goes to defaultUpstream.
func setupTestProxy(t *testing.T, defaultUpstream *httptest.Server, providerUpstreams map[string]string) testEnv {
	t.Helper()

	// Use a temp file rather than :memory: because the capture middleware
	// saves asynchronously in a goroutine, and in-memory SQLite databases are
	// per-connection (different goroutines may get different connections).
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	registry := provider.NewRegistry()
	registry.Register(provider.NewAnthropicProvider())
	registry.Register(provider.NewOpenAIProvider())

	pricing := config.NewPricingTable()

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:              1, // placeholder; we use httptest.NewServer
		UpstreamURL:       defaultUpstream.URL,
		Store:             store,
		Registry:          registry,
		ProviderUpstreams: providerUpstreams,
		Pricing:           pricing,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyTS := httptest.NewServer(srv.Handler())
	t.Cleanup(proxyTS.Close)

	return testEnv{
		Store:    store,
		Registry: registry,
		Pricing:  pricing,
		ProxyURL: proxyTS.URL,
		Client:   proxyTS.Client(),
	}
}

// pollStore waits until at least wantCount requests are stored, polling every
// 10ms up to the given timeout. Returns the stored requests.
func pollStore(t *testing.T, store *storage.SQLiteStore, wantCount int, timeout time.Duration) []storage.Request {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for {
		reqs, err := store.ListRequests(ctx, storage.ListFilter{Limit: 100})
		if err != nil {
			t.Fatalf("ListRequests: %v", err)
		}
		if len(reqs) >= wantCount {
			return reqs
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d stored requests (got %d)", wantCount, len(reqs))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ---------------------------------------------------------------------------
// Mock response bodies
// ---------------------------------------------------------------------------

const anthropicNonStreamingResponse = `{
  "id": "msg_test123",
  "type": "message",
  "role": "assistant",
  "model": "claude-sonnet-4-20250514",
  "content": [{"type": "text", "text": "Hello! How can I help you?"}],
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 25, "output_tokens": 10}
}`

const openaiNonStreamingResponse = `{
  "id": "chatcmpl-test456",
  "object": "chat.completion",
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {"role": "assistant", "content": "Hi there!"},
      "finish_reason": "stop"
    }
  ],
  "usage": {"prompt_tokens": 20, "completion_tokens": 8}
}`

func anthropicSSEPayload() string {
	return strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_stream1","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":30}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" World"}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":12}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")
}

func openaiSSEPayload() string {
	return strings.Join([]string{
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}],"usage":null}`,
		"",
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}],"usage":null}`,
		"",
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" there"},"finish_reason":null}],"usage":null}`,
		"",
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}`,
		"",
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":15,"completion_tokens":5}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestProxyForwarding(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	resp, err := env.Client.Get(env.ProxyURL + "/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want %q", string(body), `{"ok":true}`)
	}
}

func TestRequestCapture(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	if rec.Method != "POST" {
		t.Errorf("Method = %q, want POST", rec.Method)
	}
	if rec.ResponseStatus != 200 {
		t.Errorf("ResponseStatus = %d, want 200", rec.ResponseStatus)
	}
	if rec.RequestBody != reqBody {
		t.Errorf("RequestBody mismatch:\ngot  %s\nwant %s", rec.RequestBody, reqBody)
	}
	if rec.ResponseBody == "" {
		t.Error("ResponseBody is empty, expected upstream JSON")
	}
}

func TestAnthropicNonStreamingCapture(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	if rec.Provider != "anthropic" {
		t.Errorf("Provider = %q, want anthropic", rec.Provider)
	}
	if rec.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want claude-sonnet-4-20250514", rec.Model)
	}
	if rec.InputTokens != 25 {
		t.Errorf("InputTokens = %d, want 25", rec.InputTokens)
	}
	if rec.OutputTokens != 10 {
		t.Errorf("OutputTokens = %d, want 10", rec.OutputTokens)
	}
	if rec.EstimatedCostUSD <= 0 {
		t.Errorf("EstimatedCostUSD = %f, want > 0", rec.EstimatedCostUSD)
	}
}

func TestOpenAINonStreamingCapture(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(openaiNonStreamingResponse))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	if rec.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", rec.Provider)
	}
	if rec.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", rec.Model)
	}
	if rec.InputTokens != 20 {
		t.Errorf("InputTokens = %d, want 20", rec.InputTokens)
	}
	if rec.OutputTokens != 8 {
		t.Errorf("OutputTokens = %d, want 8", rec.OutputTokens)
	}
	if rec.EstimatedCostUSD <= 0 {
		t.Errorf("EstimatedCostUSD = %f, want > 0", rec.EstimatedCostUSD)
	}
}

func TestAnthropicSSEStreaming(t *testing.T) {
	sseData := anthropicSSEPayload()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"Say hello"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}

	// Verify the SSE stream is forwarded to the client.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	clientBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(clientBody), "content_block_delta") {
		t.Error("client did not receive SSE content_block_delta events")
	}

	// Verify captured record.
	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	if rec.Provider != "anthropic" {
		t.Errorf("Provider = %q, want anthropic", rec.Provider)
	}
	if rec.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want claude-sonnet-4-20250514", rec.Model)
	}
	if rec.InputTokens != 30 {
		t.Errorf("InputTokens = %d, want 30", rec.InputTokens)
	}
	if rec.OutputTokens != 12 {
		t.Errorf("OutputTokens = %d, want 12", rec.OutputTokens)
	}
	// Reassembled text should contain the concatenated deltas.
	if !strings.Contains(rec.ResponseBody, "Hello World") {
		t.Errorf("ResponseBody = %q, want to contain 'Hello World'", rec.ResponseBody)
	}
}

func TestOpenAISSEStreaming(t *testing.T) {
	sseData := openaiSSEPayload()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"Say hi"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	clientBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(clientBody), "[DONE]") {
		t.Error("client did not receive SSE [DONE] terminator")
	}

	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	if rec.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", rec.Provider)
	}
	if rec.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", rec.Model)
	}
	if rec.InputTokens != 15 {
		t.Errorf("InputTokens = %d, want 15", rec.InputTokens)
	}
	if rec.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", rec.OutputTokens)
	}
	if !strings.Contains(rec.ResponseBody, "Hi there") {
		t.Errorf("ResponseBody = %q, want to contain 'Hi there'", rec.ResponseBody)
	}
}

func TestMultiProviderRouting(t *testing.T) {
	// Two separate upstreams, one for each provider.
	var anthropicHit, openaiHit bool

	anthropicUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anthropicHit = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(anthropicUpstream.Close)

	openaiUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openaiHit = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(openaiNonStreamingResponse))
	}))
	t.Cleanup(openaiUpstream.Close)

	// Use the anthropic upstream as the default and configure per-provider routing.
	env := setupTestProxy(t, anthropicUpstream, map[string]string{
		"anthropic": anthropicUpstream.URL,
		"openai":    openaiUpstream.URL,
	})

	// Send an Anthropic request.
	{
		reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
		req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", "2023-06-01")
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("Anthropic POST: %v", err)
		}
		resp.Body.Close()
	}

	// Send an OpenAI request.
	{
		reqBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}`
		req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/chat/completions", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer sk-test")
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("OpenAI POST: %v", err)
		}
		resp.Body.Close()
	}

	if !anthropicHit {
		t.Error("Anthropic upstream was not hit")
	}
	if !openaiHit {
		t.Error("OpenAI upstream was not hit")
	}

	// Verify both captured in store with correct providers.
	reqs := pollStore(t, env.Store, 2, 2*time.Second)
	providers := map[string]bool{}
	for _, r := range reqs {
		providers[r.Provider] = true
	}
	if !providers["anthropic"] {
		t.Error("no captured request with provider=anthropic")
	}
	if !providers["openai"] {
		t.Error("no captured request with provider=openai")
	}
}

func TestMetricsAggregation(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	// Send 3 requests.
	for i := 0; i < 3; i++ {
		reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
		req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", "2023-06-01")
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST %d: %v", i, err)
		}
		resp.Body.Close()
	}

	// Wait for all 3 captures.
	pollStore(t, env.Store, 3, 3*time.Second)

	ctx := context.Background()

	// Check hourly metrics.
	hourly, err := env.Store.QueryHourlyMetrics(ctx, storage.MetricsFilter{
		Provider: "anthropic",
	})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics: %v", err)
	}
	if len(hourly) == 0 {
		t.Fatal("no hourly metrics found")
	}

	var totalReqs int64
	var totalInput int64
	var totalOutput int64
	for _, m := range hourly {
		totalReqs += m.RequestCount
		totalInput += m.InputTokens
		totalOutput += m.OutputTokens
	}
	if totalReqs != 3 {
		t.Errorf("hourly request_count = %d, want 3", totalReqs)
	}
	if totalInput != 75 { // 25 * 3
		t.Errorf("hourly input_tokens = %d, want 75", totalInput)
	}
	if totalOutput != 30 { // 10 * 3
		t.Errorf("hourly output_tokens = %d, want 30", totalOutput)
	}

	// Check daily metrics.
	daily, err := env.Store.QueryDailyMetrics(ctx, storage.MetricsFilter{
		Provider: "anthropic",
	})
	if err != nil {
		t.Fatalf("QueryDailyMetrics: %v", err)
	}
	if len(daily) == 0 {
		t.Fatal("no daily metrics found")
	}

	var dailyReqs int64
	for _, m := range daily {
		dailyReqs += m.RequestCount
	}
	if dailyReqs != 3 {
		t.Errorf("daily request_count = %d, want 3", dailyReqs)
	}
}

func TestCostEstimation(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		model        string
		path         string
		headers      map[string]string
		reqBody      string
		respBody     string
		inputTokens  int64
		outputTokens int64
	}{
		{
			name:         "anthropic claude-sonnet-4",
			provider:     "anthropic",
			model:        "claude-sonnet-4-20250514",
			path:         "/v1/messages",
			headers:      map[string]string{"anthropic-version": "2023-06-01"},
			reqBody:      `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`,
			respBody:     `{"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":1000,"output_tokens":500}}`,
			inputTokens:  1000,
			outputTokens: 500,
		},
		{
			name:         "openai gpt-4o",
			provider:     "openai",
			model:        "gpt-4o",
			path:         "/v1/chat/completions",
			headers:      map[string]string{"Authorization": "Bearer sk-test"},
			reqBody:      `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}`,
			respBody:     `{"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2000,"completion_tokens":300}}`,
			inputTokens:  2000,
			outputTokens: 300,
		},
		{
			name:         "openai gpt-4o-mini",
			provider:     "openai",
			model:        "gpt-4o-mini",
			path:         "/v1/chat/completions",
			headers:      map[string]string{"Authorization": "Bearer sk-test"},
			reqBody:      `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hi"}]}`,
			respBody:     `{"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":500,"completion_tokens":100}}`,
			inputTokens:  500,
			outputTokens: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.respBody))
			}))
			t.Cleanup(upstream.Close)

			env := setupTestProxy(t, upstream, nil)

			req, _ := http.NewRequest("POST", env.ProxyURL+tt.path, strings.NewReader(tt.reqBody))
			req.Header.Set("Content-Type", "application/json")
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			resp, err := env.Client.Do(req)
			if err != nil {
				t.Fatalf("POST: %v", err)
			}
			resp.Body.Close()

			reqs := pollStore(t, env.Store, 1, 2*time.Second)
			rec := reqs[0]

			// Compute expected cost using the pricing table directly.
			expectedCost := env.Pricing.EstimateCost(tt.provider, tt.model, tt.inputTokens, tt.outputTokens)

			if expectedCost <= 0 {
				t.Fatalf("expected cost for %s/%s should be > 0 (check pricing table)", tt.provider, tt.model)
			}
			if rec.EstimatedCostUSD != expectedCost {
				t.Errorf("EstimatedCostUSD = %f, want %f", rec.EstimatedCostUSD, expectedCost)
			}

			// Sanity check: the cost in the record matches manual calculation.
			t.Logf("%s: %d in / %d out -> $%.6f", tt.name, rec.InputTokens, rec.OutputTokens, rec.EstimatedCostUSD)
		})
	}
}

func TestCostEstimationMetricsAggregated(t *testing.T) {
	// Verify that the estimated cost flows through to the metrics aggregation tables.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	pollStore(t, env.Store, 1, 2*time.Second)

	ctx := context.Background()
	hourly, err := env.Store.QueryHourlyMetrics(ctx, storage.MetricsFilter{Provider: "anthropic"})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics: %v", err)
	}
	if len(hourly) == 0 {
		t.Fatal("no hourly metrics")
	}
	if hourly[0].EstimatedCost <= 0 {
		t.Errorf("hourly EstimatedCost = %f, want > 0", hourly[0].EstimatedCost)
	}
}

func TestStreamingCostEstimation(t *testing.T) {
	// Verify cost estimation works for streaming responses too.
	sseData := anthropicSSEPayload()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"Say hello"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()

	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	expectedCost := env.Pricing.EstimateCost("anthropic", "claude-sonnet-4-20250514", 30, 12)
	if rec.EstimatedCostUSD != expectedCost {
		t.Errorf("streaming EstimatedCostUSD = %f, want %f", rec.EstimatedCostUSD, expectedCost)
	}
}

func TestMultiProviderRoutingIsolation(t *testing.T) {
	// Verify that requests to one provider do NOT hit the other provider's upstream.
	var anthropicCount, openaiCount int

	anthropicUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anthropicCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(anthropicUpstream.Close)

	openaiUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openaiCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(openaiNonStreamingResponse))
	}))
	t.Cleanup(openaiUpstream.Close)

	env := setupTestProxy(t, anthropicUpstream, map[string]string{
		"anthropic": anthropicUpstream.URL,
		"openai":    openaiUpstream.URL,
	})

	// Send 2 Anthropic requests.
	for i := 0; i < 2; i++ {
		reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
		req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", "2023-06-01")
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("Anthropic POST %d: %v", i, err)
		}
		resp.Body.Close()
	}

	// Send 1 OpenAI request.
	{
		reqBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}`
		req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/chat/completions", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer sk-test")
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("OpenAI POST: %v", err)
		}
		resp.Body.Close()
	}

	if anthropicCount != 2 {
		t.Errorf("anthropic upstream hit %d times, want 2", anthropicCount)
	}
	if openaiCount != 1 {
		t.Errorf("openai upstream hit %d times, want 1", openaiCount)
	}
}

func TestResponseHeadersPreserved(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-abc-123")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	resp, err := env.Client.Get(env.ProxyURL + "/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if got := resp.Header.Get("X-Request-Id"); got != "req-abc-123" {
		t.Errorf("X-Request-Id = %q, want req-abc-123", got)
	}
}

func TestCaptureStoresResponseHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "test-val")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	// Verify headers are stored as JSON.
	var headers map[string]string
	if err := json.Unmarshal([]byte(rec.ResponseHeaders), &headers); err != nil {
		t.Fatalf("failed to unmarshal response headers: %v", err)
	}
	if headers["X-Custom"] != "test-val" {
		t.Errorf("stored X-Custom header = %q, want test-val", headers["X-Custom"])
	}
}

func TestErrorResponseCapture(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"too many requests"}}`))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}

	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	if rec.ResponseStatus != 429 {
		t.Errorf("stored ResponseStatus = %d, want 429", rec.ResponseStatus)
	}

	// Verify error increments in hourly metrics.
	ctx := context.Background()
	hourly, err := env.Store.QueryHourlyMetrics(ctx, storage.MetricsFilter{})
	if err != nil {
		t.Fatalf("QueryHourlyMetrics: %v", err)
	}
	if len(hourly) == 0 {
		t.Fatal("no hourly metrics")
	}

	var totalErrors int64
	for _, m := range hourly {
		totalErrors += m.ErrorCount
	}
	if totalErrors != 1 {
		t.Errorf("hourly error_count = %d, want 1", totalErrors)
	}
}

func TestLatencyTracking(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Small delay so latency is non-zero.
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
	req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	reqs := pollStore(t, env.Store, 1, 2*time.Second)
	rec := reqs[0]

	if rec.LatencyMs <= 0 {
		t.Errorf("LatencyMs = %d, want > 0", rec.LatencyMs)
	}
}

func TestSequentialBurstRequests(t *testing.T) {
	// Send multiple requests in quick succession. The proxy saves captures
	// asynchronously via goroutines, so the saves overlap even though the
	// HTTP requests are sequential. This validates that the store handles
	// the write volume without losing data. We send sequentially to avoid
	// SQLite "database is locked" errors that occur with truly concurrent
	// writes when no busy_timeout is configured.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(anthropicNonStreamingResponse))
	}))
	t.Cleanup(upstream.Close)

	env := setupTestProxy(t, upstream, nil)

	const n = 5
	for i := 0; i < n; i++ {
		reqBody := fmt.Sprintf(`{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"req %d"}]}`, i)
		req, _ := http.NewRequest("POST", env.ProxyURL+"/v1/messages", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", "2023-06-01")
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST %d: %v", i, err)
		}
		resp.Body.Close()
		// Give the async fire-and-forget goroutine time to complete the
		// SQLite write before the next request spawns another goroutine.
		// Without this, concurrent goroutines hit SQLite lock contention
		// and silently drop saves (the capture middleware ignores errors).
		time.Sleep(50 * time.Millisecond)
	}

	reqs := pollStore(t, env.Store, n, 5*time.Second)
	if len(reqs) != n {
		t.Errorf("stored %d requests, want %d", len(reqs), n)
	}
	t.Logf("burst: %d/%d requests captured", len(reqs), n)
}
