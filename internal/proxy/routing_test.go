package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/proxy"
)

func TestRoutingAnthropicRequestsToAnthropicUpstream(t *testing.T) {
	// Create per-provider mock upstreams.
	anthropicUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"anthropic"}`))
	}))
	defer anthropicUpstream.Close()

	openaiUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"openai"}`))
	}))
	defer openaiUpstream.Close()

	defaultUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"default"}`))
	}))
	defer defaultUpstream.Close()

	registry := provider.NewRegistry()
	registry.Register(provider.NewAnthropicProvider())
	registry.Register(provider.NewOpenAIProvider())

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: defaultUpstream.URL,
		Registry:    registry,
		ProviderUpstreams: map[string]string{
			"anthropic": anthropicUpstream.URL,
			"openai":    openaiUpstream.URL,
		},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	// Send an Anthropic-style request (anthropic-version header triggers detection).
	req, _ := http.NewRequest("POST", proxyServer.URL+"/v1/messages",
		strings.NewReader(`{"model":"claude-3","messages":[{"role":"user","content":"Hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"provider":"anthropic"}` {
		t.Errorf("expected anthropic upstream, got body: %s", body)
	}
}

func TestRoutingOpenAIRequestsToOpenAIUpstream(t *testing.T) {
	anthropicUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"anthropic"}`))
	}))
	defer anthropicUpstream.Close()

	openaiUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"openai"}`))
	}))
	defer openaiUpstream.Close()

	defaultUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"default"}`))
	}))
	defer defaultUpstream.Close()

	registry := provider.NewRegistry()
	registry.Register(provider.NewAnthropicProvider())
	registry.Register(provider.NewOpenAIProvider())

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: defaultUpstream.URL,
		Registry:    registry,
		ProviderUpstreams: map[string]string{
			"anthropic": anthropicUpstream.URL,
			"openai":    openaiUpstream.URL,
		},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	// Send an OpenAI-style request (path triggers detection).
	req, _ := http.NewRequest("POST", proxyServer.URL+"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4","messages":[{"role":"user","content":"Hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"provider":"openai"}` {
		t.Errorf("expected openai upstream, got body: %s", body)
	}
}

func TestRoutingFallbackToDefaultUpstream(t *testing.T) {
	anthropicUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"anthropic"}`))
	}))
	defer anthropicUpstream.Close()

	openaiUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"openai"}`))
	}))
	defer openaiUpstream.Close()

	defaultUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"default"}`))
	}))
	defer defaultUpstream.Close()

	registry := provider.NewRegistry()
	registry.Register(provider.NewAnthropicProvider())
	registry.Register(provider.NewOpenAIProvider())

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: defaultUpstream.URL,
		Registry:    registry,
		ProviderUpstreams: map[string]string{
			"anthropic": anthropicUpstream.URL,
			"openai":    openaiUpstream.URL,
		},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	// Send a request that matches no provider (no special headers, generic path).
	req, _ := http.NewRequest("GET", proxyServer.URL+"/v1/some-other-endpoint", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"provider":"default"}` {
		t.Errorf("expected default upstream, got body: %s", body)
	}
}

func TestRoutingWithNoProviderUpstreams(t *testing.T) {
	// When no provider upstreams are configured, all requests go to default.
	defaultUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"default"}`))
	}))
	defer defaultUpstream.Close()

	registry := provider.NewRegistry()
	registry.Register(provider.NewAnthropicProvider())
	registry.Register(provider.NewOpenAIProvider())

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: defaultUpstream.URL,
		Registry:    registry,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	// Even an Anthropic request should go to default when no provider upstreams set.
	req, _ := http.NewRequest("POST", proxyServer.URL+"/v1/messages",
		strings.NewReader(`{"model":"claude-3","messages":[{"role":"user","content":"Hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"provider":"default"}` {
		t.Errorf("expected default upstream, got body: %s", body)
	}
}

func TestRoutingWithNilRegistry(t *testing.T) {
	// When no registry is set, all requests go to default.
	defaultUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"default"}`))
	}))
	defer defaultUpstream.Close()

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: defaultUpstream.URL,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	req, _ := http.NewRequest("POST", proxyServer.URL+"/v1/messages",
		strings.NewReader(`{"model":"claude-3","messages":[{"role":"user","content":"Hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"provider":"default"}` {
		t.Errorf("expected default upstream, got body: %s", body)
	}
}
