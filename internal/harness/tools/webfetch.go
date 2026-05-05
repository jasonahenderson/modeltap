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
//
// SSRF defense is layered:
//  1. URL host is classified via isBlockedHost before the request is issued.
//  2. Transport.DialContext re-resolves the host at dial time and
//     validates every returned IP — closes DNS rebinding (where the
//     first resolution returned a public IP and a racing lookup
//     flips to a private one).
//  3. Client.CheckRedirect re-runs isBlockedHost on every redirect
//     target — closes the "302 → http://169.254.169.254/…" vector
//     flagged by WU-094 C-2.
func NewWebFetchTool() *WebFetchTool {
	f := &WebFetchTool{maxBytes: 100 * 1024}
	f.httpClient = &http.Client{
		Timeout:       30 * time.Second,
		Transport:     f.buildTransport(),
		CheckRedirect: f.checkRedirect,
	}
	return f
}

// buildTransport wires a DialContext that resolves the target host
// and rejects any response that includes a blocked IP. The stdlib
// Transport would otherwise dial whatever the resolver returns, which
// is where DNS rebinding lives.
func (f *WebFetchTool) buildTransport() *http.Transport {
	base := &net.Dialer{Timeout: 10 * time.Second}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		// If host is already an IP, validate it directly without a
		// resolver call. Otherwise resolve and validate every
		// returned address.
		if ip := net.ParseIP(host); ip != nil {
			if f.isBlockedIP(ip) {
				return nil, fmt.Errorf("blocked: %s resolves to a private address", host)
			}
			return base.DialContext(ctx, network, addr)
		}
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, a := range addrs {
			if f.isBlockedIP(a.IP) {
				return nil, fmt.Errorf("blocked: %s resolves to a private address", host)
			}
		}
		// Dial the first resolved IP (stdlib behavior) but via our
		// validated set. Using `addr` verbatim would re-resolve and
		// race with our check; dial the IP we already validated.
		if len(addrs) > 0 {
			return base.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
		}
		return nil, fmt.Errorf("no addresses for %s", host)
	}
	return t
}

// checkRedirect re-runs the host classification on every redirect
// hop. Returning a non-nil error aborts the redirect chain with that
// error surfaced to the caller.
func (f *WebFetchTool) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if req.URL == nil {
		return fmt.Errorf("redirect to empty URL")
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("redirect to unsupported scheme: %q", req.URL.Scheme)
	}
	if f.isBlockedHost(req.URL.Hostname()) {
		return fmt.Errorf("blocked redirect: %q resolves to a private address", req.URL.Hostname())
	}
	return nil
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
// Delegates to isBlockedIP for IP-literal handling.
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
	if ip := net.ParseIP(host); ip != nil {
		return f.isBlockedIP(ip)
	}
	return false
}

// isBlockedIP reports whether an already-parsed IP is on the SSRF
// blocklist. Covers loopback, RFC1918, link-local, multicast,
// unspecified, and IPv4-mapped IPv6 variants of all of those.
func (f *WebFetchTool) isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// Resolve IPv4-mapped IPv6 (::ffff:127.0.0.1) to the embedded
	// v4 form so IsLoopback / IsPrivate classify correctly.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if !f.allowLoopback && ip.IsLoopback() {
		return true
	}
	if ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() {
		return true
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
