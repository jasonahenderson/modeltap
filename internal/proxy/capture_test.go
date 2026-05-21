package proxy_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/correlation"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/proxy"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// captureTestEnv holds the test proxy setup with a saved channel for synchronization.
type captureTestEnv struct {
	proxyServer *httptest.Server
	store       *storage.SQLiteStore
	registry    *provider.Registry
	saved       chan struct{}
}

func newCaptureTestEnv(t *testing.T, upstreamHandler http.HandlerFunc) *captureTestEnv {
	t.Helper()

	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(upstream.Close)

	dbPath := filepath.Join(os.TempDir(), "modeltap_test_"+t.Name()+".db")
	t.Cleanup(func() { os.Remove(dbPath) })

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	registry := provider.NewRegistry()
	registry.Register(provider.NewAnthropicProvider())
	registry.Register(provider.NewOpenAIProvider())

	saved := make(chan struct{}, 100)

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: upstream.URL,
		Store:       store,
		Registry:    registry,
		OnSaved:     func() { saved <- struct{}{} },
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	t.Cleanup(proxyServer.Close)

	return &captureTestEnv{
		proxyServer: proxyServer,
		store:       store,
		registry:    registry,
		saved:       saved,
	}
}

// waitForSave blocks until the capture middleware signals a save completed, with timeout.
func (env *captureTestEnv) waitForSave(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-env.saved:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for capture middleware to save request")
	}
}

func TestCaptureMiddleware_SavesRequestAndResponse(t *testing.T) {
	respBody := `{"id":"msg_123","type":"message","model":"claude-3-5-sonnet-20241022","content":[{"type":"text","text":"Hello!"}],"usage":{"input_tokens":10,"output_tokens":5},"stop_reason":"end_turn"}`

	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})

	reqBody := `{"model":"claude-3-5-sonnet-20241022","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "sk-test")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatal("no requests saved")
	}
	saved := &reqs[0]

	if saved.Method != "POST" {
		t.Errorf("method = %q, want POST", saved.Method)
	}
	if saved.RequestBody != reqBody {
		t.Errorf("request body = %q, want %q", saved.RequestBody, reqBody)
	}
	if saved.ResponseBody != respBody {
		t.Errorf("response body = %q, want %q", saved.ResponseBody, respBody)
	}
	if saved.ResponseStatus != http.StatusOK {
		t.Errorf("status = %d, want %d", saved.ResponseStatus, http.StatusOK)
	}
}

func TestCaptureMiddleware_DetectsProviderAndExtractsMetadata(t *testing.T) {
	respBody := `{"id":"msg_123","type":"message","model":"claude-3-5-sonnet-20241022","content":[{"type":"text","text":"Hi"}],"usage":{"input_tokens":15,"output_tokens":8},"stop_reason":"end_turn"}`

	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})

	reqBody := `{"model":"claude-3-5-sonnet-20241022","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", "sk-test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatal("no requests saved")
	}
	saved := &reqs[0]

	if saved.Provider != "anthropic" {
		t.Errorf("provider = %q, want anthropic", saved.Provider)
	}
	if saved.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("model = %q, want claude-3-5-sonnet-20241022", saved.Model)
	}
	if saved.InputTokens != 15 {
		t.Errorf("input_tokens = %d, want 15", saved.InputTokens)
	}
	if saved.OutputTokens != 8 {
		t.Errorf("output_tokens = %d, want 8", saved.OutputTokens)
	}
	if saved.LatencyMs < 0 {
		t.Errorf("latency_ms = %d, want >= 0", saved.LatencyMs)
	}
}

func TestCaptureMiddleware_ResponseUnchanged(t *testing.T) {
	expectedBody := `{"id":"msg_456","type":"message","content":[{"type":"text","text":"World"}]}`
	expectedHeaders := map[string]string{
		"Content-Type":    "application/json",
		"X-Custom-Header": "custom-value",
	}

	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		for k, v := range expectedHeaders {
			w.Header().Set(k, v)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedBody))
	})

	reqBody := `{"model":"claude-3","messages":[{"role":"user","content":"Hi"}]}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != expectedBody {
		t.Errorf("response body = %q, want %q", string(body), expectedBody)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if ch := resp.Header.Get("X-Custom-Header"); ch != "custom-value" {
		t.Errorf("X-Custom-Header = %q, want custom-value", ch)
	}
}

func TestCaptureMiddleware_UnknownProvider(t *testing.T) {
	respBody := `{"result":"ok"}`

	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})

	// Send a request that doesn't match any known provider.
	reqBody := `{"query":"test"}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/custom/endpoint", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatal("no requests saved")
	}
	saved := &reqs[0]

	// Should still capture raw data, just no provider/metadata.
	if saved.Provider != "" {
		t.Errorf("provider = %q, want empty string", saved.Provider)
	}
	if saved.Model != "" {
		t.Errorf("model = %q, want empty string", saved.Model)
	}
	if saved.RequestBody != reqBody {
		t.Errorf("request body = %q, want %q", saved.RequestBody, reqBody)
	}
	if saved.ResponseBody != respBody {
		t.Errorf("response body = %q, want %q", saved.ResponseBody, respBody)
	}
}

func TestCaptureMiddleware_StoresCorrelationAndStripsInternalHeaders(t *testing.T) {
	var upstreamRunID, upstreamTraceID string

	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		upstreamRunID = r.Header.Get(correlation.HeaderRunID)
		upstreamTraceID = r.Header.Get(correlation.HeaderTraceID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	reqBody := `{"model":"gpt-4","messages":[{"role":"user","content":"Test"}]}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")
	req.Header.Set(correlation.HeaderRunID, "run-capture-1")
	req.Header.Set(correlation.HeaderTraceID, "trace-capture-1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if upstreamRunID != "" || upstreamTraceID != "" {
		t.Fatalf("internal correlation headers leaked upstream: run=%q trace=%q", upstreamRunID, upstreamTraceID)
	}

	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{RunID: "run-capture-1"})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("got %d correlated captures, want 1", len(reqs))
	}
	if reqs[0].RunID != "run-capture-1" || reqs[0].TraceID != "trace-capture-1" {
		t.Errorf("correlation = (%q, %q), want run-capture-1/trace-capture-1", reqs[0].RunID, reqs[0].TraceID)
	}
}

func TestCaptureMiddleware_UncorrelatedTrafficStoresEmptyCorrelation(t *testing.T) {
	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("got %d captures, want 1", len(reqs))
	}
	if reqs[0].RunID != "" || reqs[0].TraceID != "" {
		t.Errorf("uncorrelated capture = (%q, %q), want empty strings", reqs[0].RunID, reqs[0].TraceID)
	}
}

func TestCaptureMiddleware_PreservesRequestBody(t *testing.T) {
	var receivedBody string

	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	reqBody := `{"model":"gpt-4","messages":[{"role":"user","content":"Test"}]}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	// Verify upstream received the full body (middleware didn't consume it).
	if receivedBody != reqBody {
		t.Errorf("upstream received body = %q, want %q", receivedBody, reqBody)
	}

	// Verify the store also captured the body.
	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatal("no requests saved")
	}
	saved := &reqs[0]
	if saved.RequestBody != reqBody {
		t.Errorf("saved request body = %q, want %q", saved.RequestBody, reqBody)
	}
}

func TestCaptureMiddleware_OpenAIProvider(t *testing.T) {
	respBody := `{"id":"chatcmpl-abc","model":"gpt-4","choices":[{"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":3}}`

	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})

	reqBody := `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatal("no requests saved")
	}
	saved := &reqs[0]

	if saved.Provider != "openai" {
		t.Errorf("provider = %q, want openai", saved.Provider)
	}
	if saved.Model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", saved.Model)
	}
	if saved.InputTokens != 12 {
		t.Errorf("input_tokens = %d, want 12", saved.InputTokens)
	}
	if saved.OutputTokens != 3 {
		t.Errorf("output_tokens = %d, want 3", saved.OutputTokens)
	}
}

func TestCaptureMiddleware_HeadersCaptured(t *testing.T) {
	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-789")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	req, _ := http.NewRequest("GET", env.proxyServer.URL+"/health", nil)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatal("no requests saved")
	}
	saved := &reqs[0]

	// Verify request headers were captured.
	var reqHeaders map[string]string
	if err := json.Unmarshal([]byte(saved.RequestHeaders), &reqHeaders); err != nil {
		t.Fatalf("failed to parse request headers: %v", err)
	}
	if reqHeaders["Accept"] != "application/json" {
		t.Errorf("request Accept header = %q, want application/json", reqHeaders["Accept"])
	}

	// Verify response headers were captured.
	var respHeaders map[string]string
	if err := json.Unmarshal([]byte(saved.ResponseHeaders), &respHeaders); err != nil {
		t.Fatalf("failed to parse response headers: %v", err)
	}
	if respHeaders["X-Request-Id"] != "req-789" {
		t.Errorf("response X-Request-Id header = %q, want req-789", respHeaders["X-Request-Id"])
	}
}

func TestCaptureMiddleware_NonSuccessStatusCode(t *testing.T) {
	env := newCaptureTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"too many requests"}}`))
	})

	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}

	env.waitForSave(t, 2*time.Second)

	reqs, err := env.store.ListRequests(context.Background(), storage.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatal("no requests saved")
	}
	saved := &reqs[0]
	if saved.ResponseStatus != http.StatusTooManyRequests {
		t.Errorf("saved status = %d, want %d", saved.ResponseStatus, http.StatusTooManyRequests)
	}
}
