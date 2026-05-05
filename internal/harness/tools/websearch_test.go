package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func wsInput(t *testing.T, m map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestWebSearch_NameAndRisk(t *testing.T) {
	s := NewWebSearchTool(WebSearchConfig{APIKey: "x", Engine: "brave"})
	if s.Name() != ToolNameWebSearch {
		t.Errorf("Name = %q, want %q", s.Name(), ToolNameWebSearch)
	}
	if s.RiskLevel() != RiskExecute {
		t.Errorf("RiskLevel = %q, want execute", s.RiskLevel())
	}
}

func TestWebSearch_NoAPIKey(t *testing.T) {
	s := NewWebSearchTool(WebSearchConfig{Engine: "brave"})
	res, err := s.Execute(context.Background(), wsInput(t, map[string]any{"query": "hello"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	if !strings.Contains(strings.ToLower(res.Error), "api key") {
		t.Errorf("error should mention missing api key: %q", res.Error)
	}
}

func TestWebSearch_UnknownEngine(t *testing.T) {
	s := NewWebSearchTool(WebSearchConfig{APIKey: "x", Engine: "bogus"})
	res, err := s.Execute(context.Background(), wsInput(t, map[string]any{"query": "hello"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestWebSearch_MissingQuery(t *testing.T) {
	s := NewWebSearchTool(WebSearchConfig{APIKey: "x", Engine: "brave"})
	res, err := s.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestWebSearch_BraveSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Subscription-Token"); got != "brave-key" {
			t.Errorf("expected auth header; got %q", got)
		}
		if q := r.URL.Query().Get("q"); q != "bubble tea" {
			t.Errorf("query = %q", q)
		}
		_, _ = w.Write([]byte(`{"web":{"results":[
			{"title":"Bubble Tea","url":"https://example.com/1","description":"A tasty drink"},
			{"title":"Bubbletea Go","url":"https://example.com/2","description":"TUI framework"}
		]}}`))
	}))
	defer ts.Close()

	s := NewWebSearchTool(WebSearchConfig{APIKey: "brave-key", Engine: "brave", BraveBaseURL: ts.URL})
	res, err := s.Execute(context.Background(), wsInput(t, map[string]any{"query": "bubble tea"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	for _, want := range []string{"Bubble Tea", "https://example.com/1", "A tasty drink", "Bubbletea Go"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("missing %q in output:\n%s", want, res.Output)
		}
	}
}

func TestWebSearch_BraveEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"web":{"results":[]}}`))
	}))
	defer ts.Close()

	s := NewWebSearchTool(WebSearchConfig{APIKey: "k", Engine: "brave", BraveBaseURL: ts.URL})
	res, err := s.Execute(context.Background(), wsInput(t, map[string]any{"query": "nothing"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "no results") {
		t.Errorf("expected empty-results marker: %q", res.Output)
	}
}

func TestWebSearch_BraveHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer ts.Close()

	s := NewWebSearchTool(WebSearchConfig{APIKey: "k", Engine: "brave", BraveBaseURL: ts.URL})
	res, err := s.Execute(context.Background(), wsInput(t, map[string]any{"query": "x"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

// TestWebSearch_SerpAPIKeyRedactedOnError pins WU-094 C-3: the
// SerpAPI key rides in the query string, so any url.Error surfaces
// the full URL (key included) in its Error() string. That used to
// flow straight into the tool output. Now error paths redact the
// key before returning.
func TestWebSearch_SerpAPIKeyRedactedOnError(t *testing.T) {
	secretKey := "sk-serp-test-12345-DONOTLEAK"
	// Point at a closed TCP port so the transport-level error fires
	// with a url.Error that embeds the request URL (and key).
	s := NewWebSearchTool(WebSearchConfig{
		APIKey:         secretKey,
		Engine:         "serpapi",
		SerpAPIBaseURL: "http://127.0.0.1:1/search",
	})
	res, err := s.Execute(context.Background(), wsInput(t, map[string]any{"query": "test"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Fatalf("Status = %q, want error", res.Status)
	}
	if strings.Contains(res.Error, secretKey) {
		t.Errorf("API key leaked into error: %q", res.Error)
	}
	if !strings.Contains(res.Error, "REDACTED") {
		t.Errorf("error should show redaction marker; got %q", res.Error)
	}
}

func TestWebSearch_SerpAPISuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key := r.URL.Query().Get("api_key"); key != "serp-key" {
			t.Errorf("expected api_key query param; got %q", key)
		}
		_, _ = w.Write([]byte(`{"organic_results":[
			{"title":"Serp One","link":"https://example.com/a","snippet":"First hit"}
		]}`))
	}))
	defer ts.Close()

	s := NewWebSearchTool(WebSearchConfig{APIKey: "serp-key", Engine: "serpapi", SerpAPIBaseURL: ts.URL})
	res, err := s.Execute(context.Background(), wsInput(t, map[string]any{"query": "serpapi test"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	for _, want := range []string{"Serp One", "https://example.com/a", "First hit"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("missing %q: %q", want, res.Output)
		}
	}
}
