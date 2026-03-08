package proxy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/proxy"
)

// newTestUpstream creates a mock upstream server that echoes back request info.
func newTestUpstream(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestProxyForwardsRequests(t *testing.T) {
	upstream := newTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello from upstream"}`))
	})

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: upstream.URL,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Use httptest to test the handler directly
	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	resp, err := http.Get(proxyServer.URL + "/v1/messages")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"message":"hello from upstream"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestProxyForwardsHeaders(t *testing.T) {
	var receivedHeaders http.Header
	var receivedPath string

	upstream := newTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: upstream.URL,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	req, _ := http.NewRequest("POST", proxyServer.URL+"/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer sk-test-key")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "custom-value")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if receivedPath != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", receivedPath)
	}
	if got := receivedHeaders.Get("Authorization"); got != "Bearer sk-test-key" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer sk-test-key")
	}
	if got := receivedHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if got := receivedHeaders.Get("X-Custom-Header"); got != "custom-value" {
		t.Errorf("X-Custom-Header = %q, want %q", got, "custom-value")
	}
}

func TestProxyPreservesStatusCodes(t *testing.T) {
	codes := []int{
		http.StatusOK,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusTooManyRequests,
	}

	for _, code := range codes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			upstream := newTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			})

			srv, err := proxy.NewServer(proxy.ServerConfig{
				Port:        9999,
				UpstreamURL: upstream.URL,
			})
			if err != nil {
				t.Fatalf("NewServer: %v", err)
			}

			proxyServer := httptest.NewServer(srv.Handler())
			defer proxyServer.Close()

			resp, err := http.Get(proxyServer.URL + "/test")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != code {
				t.Errorf("status = %d, want %d", resp.StatusCode, code)
			}
		})
	}
}

func TestProxyReturnsResponseBody(t *testing.T) {
	expectedBody := `{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hello!"}]}`

	upstream := newTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	})

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: upstream.URL,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	reqBody := `{"model":"claude-3","messages":[{"role":"user","content":"Hi"}]}`
	resp, err := http.Post(proxyServer.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != expectedBody {
		t.Errorf("body = %q, want %q", string(body), expectedBody)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestNewServerValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     proxy.ServerConfig
		wantErr bool
	}{
		{
			name:    "empty upstream",
			cfg:     proxy.ServerConfig{Port: 8080, UpstreamURL: ""},
			wantErr: true,
		},
		{
			name:    "invalid port",
			cfg:     proxy.ServerConfig{Port: 0, UpstreamURL: "http://localhost:1234"},
			wantErr: true,
		},
		{
			name:    "negative port",
			cfg:     proxy.ServerConfig{Port: -1, UpstreamURL: "http://localhost:1234"},
			wantErr: true,
		},
		{
			name:    "missing scheme",
			cfg:     proxy.ServerConfig{Port: 8080, UpstreamURL: "localhost:1234"},
			wantErr: true,
		},
		{
			name:    "valid config",
			cfg:     proxy.ServerConfig{Port: 8080, UpstreamURL: "http://localhost:1234"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := proxy.NewServer(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	upstream := newTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: upstream.URL,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Test shutdown via context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown should succeed even if server hasn't started listening
	// (http.Server.Shutdown is safe to call in this case)
	err = srv.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestProxyForwardsRequestBody(t *testing.T) {
	var receivedBody string

	upstream := newTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv, err := proxy.NewServer(proxy.ServerConfig{
		Port:        9999,
		UpstreamURL: upstream.URL,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	proxyServer := httptest.NewServer(srv.Handler())
	defer proxyServer.Close()

	reqBody := `{"model":"claude-3","messages":[{"role":"user","content":"Hello"}]}`
	resp, err := http.Post(proxyServer.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if receivedBody != reqBody {
		t.Errorf("request body = %q, want %q", receivedBody, reqBody)
	}
}
