package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WebFetchTool fetches a URL and returns its text representation. HTML
// responses are converted to plain text; plain-text/JSON responses pass
// through. The permission layer already blocks internal hosts (SSRF
// list in permission.go) but WebFetch re-checks so the tool stands on
// its own without an Executor in front of it.
type WebFetchTool struct {
	httpClient    *http.Client
	maxBytes      int
	allowLoopback bool // tests flip this true to hit httptest.Server
}

// NewWebFetchTool constructs a WebFetch tool with production defaults:
// 30s timeout, 100 KB output cap, loopback blocked for SSRF.
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		maxBytes:   100 * 1024,
	}
}

// SetMaxBytes overrides the output-size cap.
func (f *WebFetchTool) SetMaxBytes(n int) { f.maxBytes = n }

func (f *WebFetchTool) Name() string        { return ToolNameWebFetch }
func (f *WebFetchTool) Description() string { return "Fetch a URL and return its text content" }
func (f *WebFetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url":     {"type":"string","description":"URL to fetch (http or https only)"},
    "headers": {"type":"object","description":"Optional HTTP headers to send"}
  },
  "required": ["url"]
}`)
}
func (f *WebFetchTool) OutputEnvelope() string { return "text" }
func (f *WebFetchTool) RiskLevel() RiskLevel   { return RiskExecute }

type wfArgs struct {
	URL     *string           `json:"url"`
	Headers map[string]string `json:"headers"`
}

func (f *WebFetchTool) Execute(ctx context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in wfArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.URL == nil || *in.URL == "" {
		return ErrorResult("url is required"), nil
	}

	u, err := url.Parse(*in.URL)
	if err != nil {
		return ErrorResult("invalid url: %v", err), nil
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrorResult("unsupported url scheme: %q", u.Scheme), nil
	}
	if f.isBlockedHost(u.Hostname()) {
		return ErrorResult("blocked: host %q resolves to a private address", u.Hostname()), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ErrorResult("request: %v", err), nil
	}
	req.Header.Set("User-Agent", "modeltap-harness/1.0")
	for k, v := range in.Headers {
		req.Header.Set(k, v)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return ErrorResult("fetch: %v", err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ErrorResult("http %d", resp.StatusCode), nil
	}

	limit := f.maxBytes
	if limit <= 0 {
		limit = 100 * 1024
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if readErr != nil {
		return ErrorResult("read body: %v", readErr), nil
	}
	truncated := false
	if len(body) > limit {
		body = body[:limit]
		truncated = true
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	var out string
	switch {
	case strings.Contains(ct, "text/html"), strings.Contains(ct, "application/xhtml"):
		out = htmlToText(string(body))
	default:
		out = string(body)
	}

	if truncated {
		out = fmt.Sprintf("[output truncated at %d bytes]\n%s", limit, out)
	}
	return SuccessResult(out, "text"), nil
}

// isBlockedHost reports whether the host should be refused outright
// regardless of the permission layer (SSRF defense-in-depth).
func (f *WebFetchTool) isBlockedHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return true
	}
	if !f.allowLoopback {
		switch host {
		case "localhost", "127.0.0.1", "0.0.0.0", "::1":
			return true
		}
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if !f.allowLoopback && ip.IsLoopback() {
			return true
		}
		if ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}

var (
	htmlScriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	htmlStyleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlTagRe    = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlSpaceRe  = regexp.MustCompile(`[ \t]+`)
	htmlBlankRe  = regexp.MustCompile(`\n{3,}`)
)

// htmlToText strips HTML tags and decodes a small set of common
// entities, then collapses whitespace. It is deliberately simple —
// good enough for tool output; deeper parsing is not this tool's job.
func htmlToText(s string) string {
	s = htmlScriptRe.ReplaceAllString(s, "")
	s = htmlStyleRe.ReplaceAllString(s, "")
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = htmlSpaceRe.ReplaceAllString(s, " ")

	lines := strings.Split(s, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		trimmed = append(trimmed, t)
	}
	joined := strings.Join(trimmed, "\n")
	joined = htmlBlankRe.ReplaceAllString(joined, "\n\n")
	return strings.TrimSpace(joined)
}
