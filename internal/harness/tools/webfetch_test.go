package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func wfInput(t *testing.T, urlStr string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{"url": urlStr})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestWebFetch_NameAndRisk(t *testing.T) {
	f := NewWebFetchTool()
	if f.Name() != ToolNameWebFetch {
		t.Errorf("Name = %q, want %q", f.Name(), ToolNameWebFetch)
	}
	if f.RiskLevel() != RiskExecute {
		t.Errorf("RiskLevel = %q, want execute", f.RiskLevel())
	}
}

func TestWebFetch_HTMLToText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>T</title><style>.x{}</style></head>
<body><h1>Welcome</h1><script>alert(1)</script><p>Hello <b>world</b>.</p></body></html>`))
	}))
	defer ts.Close()

	f := newWebFetchForTest()
	res, err := f.Execute(context.Background(), wfInput(t, ts.URL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "Welcome") || !strings.Contains(res.Output, "Hello world") {
		t.Errorf("expected readable text; got %q", res.Output)
	}
	for _, unwanted := range []string{"<h1>", "<p>", "alert(1)", ".x{}"} {
		if strings.Contains(res.Output, unwanted) {
			t.Errorf("output should strip %q: %q", unwanted, res.Output)
		}
	}
}

func TestWebFetch_PlainText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain text body"))
	}))
	defer ts.Close()

	f := newWebFetchForTest()
	res, err := f.Execute(context.Background(), wfInput(t, ts.URL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if res.Output != "plain text body" {
		t.Errorf("plain text body expected verbatim; got %q", res.Output)
	}
}

func TestWebFetch_SSRF_Localhost(t *testing.T) {
	f := NewWebFetchTool()
	res, err := f.Execute(context.Background(), wfInput(t, "http://localhost:99/"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error for SSRF", res.Status)
	}
}

func TestWebFetch_SSRF_Private(t *testing.T) {
	f := NewWebFetchTool()
	for _, u := range []string{
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://192.168.1.1/",
		"http://169.254.1.1/",
	} {
		res, err := f.Execute(context.Background(), wfInput(t, u))
		if err != nil {
			t.Fatalf("Execute %s: %v", u, err)
		}
		if res.Status != StatusError {
			t.Errorf("Status for %s = %q, want error", u, res.Status)
		}
	}
}

func TestWebFetch_SSRF_FileScheme(t *testing.T) {
	f := NewWebFetchTool()
	res, err := f.Execute(context.Background(), wfInput(t, "file:///etc/passwd"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestWebFetch_Truncation(t *testing.T) {
	big := strings.Repeat("A", 50_000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(big))
	}))
	defer ts.Close()

	f := newWebFetchForTest()
	f.SetMaxBytes(4096)
	res, err := f.Execute(context.Background(), wfInput(t, ts.URL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "truncated") {
		t.Errorf("expected truncation marker: len=%d", len(res.Output))
	}
	if len(res.Output) > 5000 {
		t.Errorf("output too large: %d", len(res.Output))
	}
}

func TestWebFetch_MissingURL(t *testing.T) {
	f := NewWebFetchTool()
	res, err := f.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

// TestWebFetch_RedirectToPrivateBlocked pins WU-094 C-2: the
// previous build used the default CheckRedirect (follow up to 10),
// so a public endpoint returning a 302 to http://169.254.169.254/...
// was followed transparently and the internal response leaked back
// to the model. Now CheckRedirect re-runs isBlockedHost on every hop.
func TestWebFetch_RedirectToPrivateBlocked(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://169.254.169.254/latest/meta-data/")
		w.WriteHeader(http.StatusFound)
	}))
	defer ts.Close()

	// The tool must block even though the initial URL is httptest
	// (loopback, allowed in test builds) — the redirect target is
	// link-local and must be refused regardless of the test flag.
	f := newWebFetchForTest()
	res, err := f.Execute(context.Background(), wfInput(t, ts.URL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Fatalf("Status = %q, want error (redirect to link-local must be blocked)", res.Status)
	}
	if !strings.Contains(res.Error, "169.254") && !strings.Contains(res.Error, "blocked") {
		t.Errorf("error should name the blocked redirect; got %q", res.Error)
	}
}

func TestWebFetch_RedirectLoopBounded(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", ts.URL)
		w.WriteHeader(http.StatusFound)
	}))
	defer ts.Close()

	f := newWebFetchForTest()
	res, err := f.Execute(context.Background(), wfInput(t, ts.URL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error on redirect loop", res.Status)
	}
}

func TestWebFetch_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	f := newWebFetchForTest()
	res, err := f.Execute(context.Background(), wfInput(t, ts.URL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	if !strings.Contains(res.Error, "404") {
		t.Errorf("error should mention status: %q", res.Error)
	}
}

// newWebFetchForTest returns a WebFetchTool with the loopback-block
// disabled so httptest servers on 127.0.0.1 can be hit from test code.
func newWebFetchForTest() *WebFetchTool {
	f := NewWebFetchTool()
	f.allowLoopback = true
	return f
}
