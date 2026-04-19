package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchTool queries an external search API. The supported engines
// are Brave Search and SerpAPI. Which engine (and its API key) is
// injected at construction time; the `query` and `limit` come from the
// tool input.
type WebSearchTool struct {
	cfg        WebSearchConfig
	httpClient *http.Client
}

// WebSearchConfig holds the per-session configuration for WebSearch.
// BraveBaseURL / SerpAPIBaseURL override the default upstream endpoints
// — tests inject httptest servers here; production leaves them empty.
type WebSearchConfig struct {
	APIKey         string
	Engine         string // "brave" or "serpapi"
	BraveBaseURL   string
	SerpAPIBaseURL string
}

// NewWebSearchTool constructs a WebSearch tool with the given config.
func NewWebSearchTool(cfg WebSearchConfig) *WebSearchTool {
	return &WebSearchTool{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ToolNameWebSearch is the canonical registered name.
const ToolNameWebSearch = "WebSearch"

const (
	engineBrave   = "brave"
	engineSerpAPI = "serpapi"

	defaultBraveURL   = "https://api.search.brave.com/res/v1/web/search"
	defaultSerpAPIURL = "https://serpapi.com/search"

	defaultWebSearchLimit = 10
	maxWebSearchLimit     = 25
)

func (s *WebSearchTool) Name() string        { return ToolNameWebSearch }
func (s *WebSearchTool) Description() string { return "Search the web via an external search API" }
func (s *WebSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {"type":"string","description":"Search query"},
    "limit": {"type":"integer","description":"Maximum results (default 10, max 25)"}
  },
  "required": ["query"]
}`)
}
func (s *WebSearchTool) OutputEnvelope() string { return "text" }
func (s *WebSearchTool) RiskLevel() RiskLevel   { return RiskExecute }

type wsArgs struct {
	Query *string `json:"query"`
	Limit int     `json:"limit"`
}

type searchHit struct {
	title   string
	url     string
	snippet string
}

func (s *WebSearchTool) Execute(ctx context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in wsArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.Query == nil || strings.TrimSpace(*in.Query) == "" {
		return ErrorResult("query is required"), nil
	}

	if s.cfg.APIKey == "" {
		return ErrorResult("no WebSearch API key configured"), nil
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultWebSearchLimit
	}
	if limit > maxWebSearchLimit {
		limit = maxWebSearchLimit
	}

	var hits []searchHit
	var err error
	switch strings.ToLower(s.cfg.Engine) {
	case engineBrave:
		hits, err = s.braveSearch(ctx, *in.Query, limit)
	case engineSerpAPI:
		hits, err = s.serpAPISearch(ctx, *in.Query, limit)
	default:
		return ErrorResult("unknown search engine: %q", s.cfg.Engine), nil
	}
	if err != nil {
		return ErrorResult("%v", err), nil
	}

	if len(hits) == 0 {
		return SuccessResult("no results", "text"), nil
	}

	var b strings.Builder
	for i, h := range hits {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d. %s\n   URL: %s\n", i+1, h.title, h.url)
		if h.snippet != "" {
			fmt.Fprintf(&b, "   Snippet: %s\n", h.snippet)
		}
	}
	return SuccessResult(strings.TrimRight(b.String(), "\n"), "text"), nil
}

func (s *WebSearchTool) braveSearch(ctx context.Context, query string, limit int) ([]searchHit, error) {
	base := s.cfg.BraveBaseURL
	if base == "" {
		base = defaultBraveURL
	}

	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("brave base url: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("count", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("brave request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", s.cfg.APIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("brave read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("brave status %d: %s", resp.StatusCode, body)
	}

	var parsed struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("brave decode: %w", err)
	}
	out := make([]searchHit, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		out = append(out, searchHit{title: r.Title, url: r.URL, snippet: r.Description})
	}
	return out, nil
}

func (s *WebSearchTool) serpAPISearch(ctx context.Context, query string, limit int) ([]searchHit, error) {
	base := s.cfg.SerpAPIBaseURL
	if base == "" {
		base = defaultSerpAPIURL
	}

	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("serpapi base url: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("num", fmt.Sprintf("%d", limit))
	q.Set("api_key", s.cfg.APIKey)
	q.Set("engine", "google")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("serpapi request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("serpapi http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("serpapi read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("serpapi status %d: %s", resp.StatusCode, body)
	}

	var parsed struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("serpapi decode: %w", err)
	}
	out := make([]searchHit, 0, len(parsed.OrganicResults))
	for _, r := range parsed.OrganicResults {
		out = append(out, searchHit{title: r.Title, url: r.Link, snippet: r.Snippet})
	}
	return out, nil
}
