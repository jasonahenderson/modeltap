package bff

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProviderRegistry_Add_Duplicate(t *testing.T) {
	r := NewProviderRegistry()
	if err := r.Add(&ProviderEndpoint{Name: "x", Type: ProviderTypeAnthropic, APIKey: "k"}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := r.Add(&ProviderEndpoint{Name: "x", Type: ProviderTypeAnthropic, APIKey: "k"}); err == nil {
		t.Errorf("expected error on duplicate add")
	}
}

func TestProviderRegistry_Add_InvalidType(t *testing.T) {
	r := NewProviderRegistry()
	if err := r.Add(&ProviderEndpoint{Name: "x", Type: "unknown"}); err == nil {
		t.Errorf("expected error for unknown type")
	}
}

func TestProviderRegistry_Add_DefaultHost(t *testing.T) {
	cases := map[string]string{
		ProviderTypeAnthropic: "https://api.anthropic.com",
		ProviderTypeOpenAI:    "https://api.openai.com",
		ProviderTypeOllama:    "http://localhost:11434",
		ProviderTypeMLX:       "http://localhost:8080",
	}
	for pt, host := range cases {
		t.Run(pt, func(t *testing.T) {
			r := NewProviderRegistry()
			ep := &ProviderEndpoint{Name: pt + "-ep", Type: pt, APIKey: "k"}
			if err := r.Add(ep); err != nil {
				t.Fatalf("add: %v", err)
			}
			if ep.Host != host {
				t.Errorf("default host = %q, want %q", ep.Host, host)
			}
		})
	}
}

func TestProviderRegistry_Get_All_Names(t *testing.T) {
	r := NewProviderRegistry()
	_ = r.Add(&ProviderEndpoint{Name: "a", Type: ProviderTypeAnthropic, APIKey: "k"})
	_ = r.Add(&ProviderEndpoint{Name: "b", Type: ProviderTypeOpenAI, APIKey: "k"})

	if r.Get("a") == nil || r.Get("b") == nil {
		t.Errorf("Get returned nil for known endpoints")
	}
	if r.Get("missing") != nil {
		t.Errorf("Get(missing) should be nil")
	}
	if names := r.Names(); len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("Names order = %v", names)
	}
	if all := r.All(); len(all) != 2 {
		t.Errorf("All len = %d", len(all))
	}
}

func TestCheckAnthropic_Ready(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reg := NewProviderRegistry()
	reg.SetHTTPClient(srv.Client())
	ep := &ProviderEndpoint{Name: "a", Type: ProviderTypeAnthropic, APIKey: "k", Host: srv.URL, Upstream: srv.URL}
	_ = reg.Add(ep)

	reg.CheckEndpoint(context.Background(), ep)
	if ep.Status() != ProviderStatusReady {
		t.Errorf("status = %q, want ready (err=%q)", ep.Status(), ep.ErrorMessage())
	}
}

func TestCheckAnthropic_Unauthorized_StillReady(t *testing.T) {
	// 401 indicates the endpoint is reachable and TLS/connectivity are
	// fine — just the key is bad. Health should report ready; auth
	// failure surfaces later on real dispatch.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	reg := NewProviderRegistry()
	reg.SetHTTPClient(srv.Client())
	ep := &ProviderEndpoint{Name: "a", Type: ProviderTypeAnthropic, APIKey: "k", Host: srv.URL, Upstream: srv.URL}
	_ = reg.Add(ep)

	reg.CheckEndpoint(context.Background(), ep)
	if ep.Status() != ProviderStatusReady {
		t.Errorf("status = %q, want ready (401 is reachable)", ep.Status())
	}
}

func TestCheckAnthropic_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	reg := NewProviderRegistry()
	reg.SetHTTPClient(srv.Client())
	ep := &ProviderEndpoint{Name: "a", Type: ProviderTypeAnthropic, APIKey: "k", Host: srv.URL, Upstream: srv.URL}
	_ = reg.Add(ep)

	reg.CheckEndpoint(context.Background(), ep)
	if ep.Status() != ProviderStatusError {
		t.Errorf("status = %q, want error", ep.Status())
	}
}

func TestCheckAnthropic_MissingAPIKey(t *testing.T) {
	reg := NewProviderRegistry()
	ep := &ProviderEndpoint{Name: "a", Type: ProviderTypeAnthropic, Host: "http://localhost"}
	_ = reg.Add(ep)

	reg.CheckEndpoint(context.Background(), ep)
	if ep.Status() != ProviderStatusUnavailable {
		t.Errorf("status = %q, want unavailable", ep.Status())
	}
}

// PATCH-0025: cloud-probe targets Upstream, not Host. Host may point at
// the local capture proxy per PATCH-0005; the probe must bypass that
// and verify the credentialed cloud API itself is reachable.
func TestCheckAnthropic_ProbesUpstreamNotHost(t *testing.T) {
	var hits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	// Host points at a non-existent local proxy; the test fails if
	// the probe hits Host instead of Upstream.
	reg := NewProviderRegistry()
	reg.SetHTTPClient(upstream.Client())
	ep := &ProviderEndpoint{
		Name:     "a",
		Type:     ProviderTypeAnthropic,
		APIKey:   "k",
		Host:     "http://127.0.0.1:1", // unreachable local proxy
		Upstream: upstream.URL,
	}
	_ = reg.Add(ep)

	reg.CheckEndpoint(context.Background(), ep)
	if ep.Status() != ProviderStatusReady {
		t.Errorf("status = %q (err=%q), want ready", ep.Status(), ep.ErrorMessage())
	}
	if hits != 1 {
		t.Errorf("upstream hits = %d, want 1 (probe should hit Upstream not Host)", hits)
	}
}

func TestCheckOllama_ReadyWithModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]any{
			"models": []map[string]any{
				{"name": "llama-3.1:8b"},
				{"name": "mistral:7b"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	reg := NewProviderRegistry()
	reg.SetHTTPClient(srv.Client())
	ep := &ProviderEndpoint{Name: "ol", Type: ProviderTypeOllama, Host: srv.URL, Discover: true}
	_ = reg.Add(ep)

	reg.CheckEndpoint(context.Background(), ep)
	if ep.Status() != ProviderStatusReady {
		t.Fatalf("status = %q, err=%q", ep.Status(), ep.ErrorMessage())
	}
	models := ep.Models()
	if len(models) != 2 || models[0] != "llama-3.1:8b" {
		t.Errorf("models = %v", models)
	}
}

func TestCheckMLX_ReadyWithModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "mlx-community/Llama-3-8B"}},
		})
	}))
	defer srv.Close()

	reg := NewProviderRegistry()
	reg.SetHTTPClient(srv.Client())
	ep := &ProviderEndpoint{Name: "mlx", Type: ProviderTypeMLX, Host: srv.URL, Discover: true}
	_ = reg.Add(ep)

	reg.CheckEndpoint(context.Background(), ep)
	if ep.Status() != ProviderStatusReady {
		t.Errorf("status = %q", ep.Status())
	}
	if m := ep.Models(); len(m) != 1 || m[0] != "mlx-community/Llama-3-8B" {
		t.Errorf("models = %v", m)
	}
}

func TestExpandEnvAPIKey(t *testing.T) {
	origLookup := lookupEnv
	t.Cleanup(func() { lookupEnv = origLookup })
	lookupEnv = func(name string) string {
		if name == "FOO" {
			return "secret-value"
		}
		return ""
	}

	cases := []struct {
		in, want string
	}{
		{"${FOO}", "secret-value"},
		{"${MISSING}", ""},
		{"raw-key", "raw-key"},
		{"", ""},
	}
	for _, c := range cases {
		if got := ExpandEnvAPIKey(c.in); got != c.want {
			t.Errorf("ExpandEnvAPIKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"short", "*****"},
		{"sk-ant-api-12345678ABCD", "sk-ant-****ABCD"},
	}
	for _, c := range cases {
		if got := MaskAPIKey(c.in); got != c.want {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestProviderRegistry_CheckAll_Concurrent(t *testing.T) {
	// Two endpoints, both reachable — CheckAll should update both
	// statuses within the default timeout.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reg := NewProviderRegistry()
	reg.SetHTTPClient(srv.Client())
	_ = reg.Add(&ProviderEndpoint{Name: "a", Type: ProviderTypeAnthropic, APIKey: "k", Host: srv.URL, Upstream: srv.URL})
	_ = reg.Add(&ProviderEndpoint{Name: "b", Type: ProviderTypeOpenAI, APIKey: "k", Host: srv.URL, Upstream: srv.URL})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	reg.CheckAll(ctx)

	for _, ep := range reg.All() {
		if ep.Status() != ProviderStatusReady {
			t.Errorf("endpoint %q status = %q", ep.Name, ep.Status())
		}
		if ep.LastCheck().IsZero() {
			t.Errorf("endpoint %q LastCheck not set", ep.Name)
		}
	}
}
