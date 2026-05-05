package proxy_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/proxy"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// sseEvent is a helper for building SSE events in tests.
type sseEvent struct {
	Event string // optional: "event: <type>\n"
	Data  string // "data: <payload>\n"
}

// writeSSE writes a sequence of SSE events to the ResponseWriter, flushing after each event.
func writeSSE(w http.ResponseWriter, events []sseEvent) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("ResponseWriter does not implement http.Flusher")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for _, evt := range events {
		if evt.Event != "" {
			fmt.Fprintf(w, "event: %s\n", evt.Event)
		}
		fmt.Fprintf(w, "data: %s\n\n", evt.Data)
		flusher.Flush()
	}
}

// anthropicSSEEvents returns a set of SSE events simulating an Anthropic streaming response.
func anthropicSSEEvents() []sseEvent {
	return []sseEvent{
		{
			Event: "message_start",
			Data:  `{"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"usage":{"input_tokens":20,"output_tokens":0}}}`,
		},
		{
			Event: "content_block_start",
			Data:  `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		},
		{
			Event: "ping",
			Data:  `{"type":"ping"}`,
		},
		{
			Event: "content_block_delta",
			Data:  `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		},
		{
			Event: "content_block_delta",
			Data:  `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world!"}}`,
		},
		{
			Event: "content_block_stop",
			Data:  `{"type":"content_block_stop","index":0}`,
		},
		{
			Event: "message_delta",
			Data:  `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":8}}`,
		},
		{
			Event: "message_stop",
			Data:  `{"type":"message_stop"}`,
		},
	}
}

// openaiSSEEvents returns a set of SSE events simulating an OpenAI streaming response.
func openaiSSEEvents() []sseEvent {
	return []sseEvent{
		{Data: `{"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`},
		{Data: `{"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`},
		{Data: `{"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world!"},"finish_reason":null}]}`},
		{Data: `{"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`},
		{Data: `{"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":15,"completion_tokens":5,"total_tokens":20}}`},
		{Data: `[DONE]`},
	}
}

func newStreamTestEnv(t *testing.T, upstreamHandler http.HandlerFunc) *captureTestEnv {
	t.Helper()

	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(upstream.Close)

	dbPath := filepath.Join(os.TempDir(), "modeltap_stream_test_"+t.Name()+".db")
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

func TestStreamCapture_Anthropic(t *testing.T) {
	env := newStreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, anthropicSSEEvents())
	})

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "sk-test")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read entire response.
	body, _ := io.ReadAll(resp.Body)

	// Verify the client received SSE content type.
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Verify the client received the SSE data (spot-check for some known content).
	if !strings.Contains(string(body), "Hello") {
		t.Errorf("response body should contain 'Hello', got: %s", string(body))
	}

	// Wait for the async save.
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
	if saved.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", saved.Model)
	}
	if saved.InputTokens != 20 {
		t.Errorf("input_tokens = %d, want 20", saved.InputTokens)
	}
	if saved.OutputTokens != 8 {
		t.Errorf("output_tokens = %d, want 8", saved.OutputTokens)
	}
	// The reassembled response body should contain the full text.
	if !strings.Contains(saved.ResponseBody, "Hello world!") {
		t.Errorf("saved response body = %q, want it to contain 'Hello world!'", saved.ResponseBody)
	}
	if saved.LatencyMs < 0 {
		t.Errorf("latency_ms = %d, want >= 0", saved.LatencyMs)
	}
}

func TestStreamCapture_OpenAI(t *testing.T) {
	env := newStreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, openaiSSEEvents())
	})

	reqBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(string(body), "Hello") {
		t.Errorf("response body should contain 'Hello', got: %s", string(body))
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

	if saved.Provider != "openai" {
		t.Errorf("provider = %q, want openai", saved.Provider)
	}
	if saved.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", saved.Model)
	}
	if saved.InputTokens != 15 {
		t.Errorf("input_tokens = %d, want 15", saved.InputTokens)
	}
	if saved.OutputTokens != 5 {
		t.Errorf("output_tokens = %d, want 5", saved.OutputTokens)
	}
	if !strings.Contains(saved.ResponseBody, "Hello world!") {
		t.Errorf("saved response body = %q, want it to contain 'Hello world!'", saved.ResponseBody)
	}
}

func TestStreamCapture_ChunksFlushedImmediately(t *testing.T) {
	// Use a channel to coordinate: upstream sends chunks with delays, and
	// we verify the client receives them promptly.
	var mu sync.Mutex
	clientReceiveTimes := make([]time.Time, 0)

	env := newStreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-sonnet-4-20250514\",\"usage\":{\"input_tokens\":5}}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"chunk1\"}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"chunk2\"}}\n\n",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}

		for _, evt := range events {
			fmt.Fprint(w, evt)
			flusher.Flush()
			time.Sleep(20 * time.Millisecond)
		}
	})

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":10,"messages":[{"role":"user","content":"test"}],"stream":true}`
	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response line by line, recording when we receive each event.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			mu.Lock()
			clientReceiveTimes = append(clientReceiveTimes, time.Now())
			mu.Unlock()
		}
	}

	// We should have received multiple data lines.
	mu.Lock()
	defer mu.Unlock()
	if len(clientReceiveTimes) < 3 {
		t.Fatalf("expected at least 3 data events, got %d", len(clientReceiveTimes))
	}

	// The time between first and last data chunk should show they were streamed
	// (not buffered until the end). Each gap should be less than 200ms.
	for i := 1; i < len(clientReceiveTimes); i++ {
		gap := clientReceiveTimes[i].Sub(clientReceiveTimes[i-1])
		if gap > 200*time.Millisecond {
			t.Errorf("gap between chunk %d and %d was %v, expected < 200ms (indicates buffering)", i-1, i, gap)
		}
	}
}

func TestStreamCapture_NonSSEStillHandled(t *testing.T) {
	// Non-SSE (regular JSON) responses should still be captured via the existing path.
	respBody := `{"id":"msg_123","type":"message","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Hello!"}],"usage":{"input_tokens":10,"output_tokens":5},"stop_reason":"end_turn"}`

	env := newStreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
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

	if saved.Provider != "anthropic" {
		t.Errorf("provider = %q, want anthropic", saved.Provider)
	}
	if saved.ResponseBody != respBody {
		t.Errorf("response body = %q, want %q", saved.ResponseBody, respBody)
	}
	if saved.InputTokens != 10 {
		t.Errorf("input_tokens = %d, want 10", saved.InputTokens)
	}
	if saved.OutputTokens != 5 {
		t.Errorf("output_tokens = %d, want 5", saved.OutputTokens)
	}
}

func TestStreamCapture_ReassembledResponseSavedWithMetadata(t *testing.T) {
	// Verify that the complete metadata is extracted from the reassembled stream.
	env := newStreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, anthropicSSEEvents())
	})

	reqBody := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"Hi"}],"stream":true}`
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

	// Verify all metadata fields are populated.
	if saved.Provider != "anthropic" {
		t.Errorf("provider = %q, want anthropic", saved.Provider)
	}
	if saved.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", saved.Model)
	}
	if saved.InputTokens != 20 {
		t.Errorf("input_tokens = %d, want 20", saved.InputTokens)
	}
	if saved.OutputTokens != 8 {
		t.Errorf("output_tokens = %d, want 8", saved.OutputTokens)
	}
	if saved.ResponseStatus != http.StatusOK {
		t.Errorf("status = %d, want %d", saved.ResponseStatus, http.StatusOK)
	}
	if saved.Method != "POST" {
		t.Errorf("method = %q, want POST", saved.Method)
	}
	if saved.RequestBody != reqBody {
		t.Errorf("request body = %q, want %q", saved.RequestBody, reqBody)
	}

	// Verify request headers were captured.
	if saved.RequestHeaders == "" {
		t.Error("request headers should not be empty")
	}
	// Verify response headers were captured.
	if saved.ResponseHeaders == "" {
		t.Error("response headers should not be empty")
	}
	// Verify latency is non-negative (may be 0 for fast local tests).
	if saved.LatencyMs < 0 {
		t.Errorf("latency_ms = %d, want >= 0", saved.LatencyMs)
	}
}

func TestStreamCapture_UnknownProviderSSE(t *testing.T) {
	// SSE from an unknown provider: should still capture raw SSE data but no metadata.
	env := newStreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, "data: {\"text\":\"hello\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"text\":\"world\"}\n\n")
		flusher.Flush()
	})

	req, _ := http.NewRequest("POST", env.proxyServer.URL+"/v1/custom/stream", strings.NewReader(`{}`))
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

	// Should still be saved with raw SSE data but no provider metadata.
	if saved.Provider != "" {
		t.Errorf("provider = %q, want empty", saved.Provider)
	}
	// The raw SSE data should be in the response body.
	if !strings.Contains(saved.ResponseBody, "hello") {
		t.Errorf("response body = %q, want it to contain 'hello'", saved.ResponseBody)
	}
}
