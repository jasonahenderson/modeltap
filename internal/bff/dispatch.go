package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
)

// DispatchOpts is the input to TurnDispatcher.Dispatch. It aggregates
// the canonical conversation state plus the turn-specific parameters
// the provider adapter needs to format a request.
type DispatchOpts struct {
	Conversation *Conversation
	SystemPrompt string
	Model        string
	EndpointName string // which ProviderRegistry entry to use
	MaxTokens    int
	Temperature  *float64
	Tools        []protocol.ToolDefinition
	Capabilities []string
	Stream       bool
	WindowSize   int
}

// TurnDispatcher translates a canonical Conversation into a
// provider-specific HTTP request and sends it. Streaming responses are
// returned as raw *http.Response so the caller (WU-053) can relay the
// SSE chunks into protocol notifications. Non-streaming callers use
// DispatchSync.
type TurnDispatcher struct {
	endpoints  *ProviderRegistry
	adapters   *provider.Registry
	httpClient *http.Client
}

// NewTurnDispatcher returns a dispatcher rooted at the given registries.
// endpoints carries runtime state (API key, host, health); adapters
// provides per-provider-type FormatMessages translators.
func NewTurnDispatcher(endpoints *ProviderRegistry, adapters *provider.Registry) *TurnDispatcher {
	return &TurnDispatcher{
		endpoints:  endpoints,
		adapters:   adapters,
		httpClient: &http.Client{Timeout: 300 * time.Second},
	}
}

// SetHTTPClient overrides the outbound HTTP client. Used by tests with
// httptest.Server.Client().
func (d *TurnDispatcher) SetHTTPClient(c *http.Client) { d.httpClient = c }

// Dispatch translates the conversation into the endpoint's native wire
// format and sends the HTTP request, returning the response for the
// caller to stream. On connection or formatting failure, the error is
// wrapped with the MT-CONN-009 provider_unavailable diagnostic.
func (d *TurnDispatcher) Dispatch(ctx context.Context, opts DispatchOpts) (*http.Response, error) {
	ep := d.endpoints.Get(opts.EndpointName)
	if ep == nil {
		return nil, dispatchError(opts.EndpointName, fmt.Errorf("endpoint %q not registered", opts.EndpointName))
	}
	adapter := d.adapters.Get(ep.Type)
	if adapter == nil {
		return nil, dispatchError(opts.EndpointName, fmt.Errorf("no adapter registered for provider type %q", ep.Type))
	}

	fmOpts := provider.FormatMessagesOpts{
		Messages:     opts.Conversation.Messages(),
		SystemPrompt: opts.SystemPrompt,
		Model:        opts.Model,
		MaxTokens:    opts.MaxTokens,
		Temperature:  opts.Temperature,
		Stream:       opts.Stream,
		WindowSize:   opts.WindowSize,
	}
	body, err := adapter.FormatMessages(fmOpts)
	if err != nil {
		return nil, formatError(opts.EndpointName, err)
	}

	url := strings.TrimRight(ep.Host, "/") + providerEndpointPath(ep.Type)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, dispatchError(opts.EndpointName, fmt.Errorf("new request: %w", err))
	}
	setAuthHeaders(req, ep)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, dispatchError(opts.EndpointName, fmt.Errorf("http: %w", err))
	}
	if resp.StatusCode >= 400 {
		// Read a bounded slice of the body for context; don't drain
		// unbounded server responses.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		_ = resp.Body.Close()
		return nil, httpError(opts.EndpointName, resp.StatusCode, string(snippet))
	}
	return resp, nil
}

// DispatchSync reads the full response body, returning its bytes and
// the HTTP status. Callers (e.g., content.transform in WU-062) are
// responsible for unmarshalling into the canonical form via the
// adapter's response parser.
func (d *TurnDispatcher) DispatchSync(ctx context.Context, opts DispatchOpts) ([]byte, error) {
	opts.Stream = false
	resp, err := d.Dispatch(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, dispatchError(opts.EndpointName, fmt.Errorf("read body: %w", err))
	}
	return body, nil
}

// providerEndpointPath returns the canonical API endpoint path for the
// given provider type. Host is provided by the endpoint config; this
// only encodes the API method path.
func providerEndpointPath(providerType string) string {
	switch providerType {
	case ProviderTypeAnthropic:
		return "/v1/messages"
	case ProviderTypeOpenAI:
		return "/v1/chat/completions"
	case ProviderTypeOllama:
		return "/api/chat"
	case ProviderTypeMLX:
		return "/v1/chat/completions"
	}
	return ""
}

// setAuthHeaders stamps the provider-type-appropriate auth headers on
// req. No-op if API key is empty (Ollama/MLX local).
func setAuthHeaders(req *http.Request, ep *ProviderEndpoint) {
	switch ep.Type {
	case ProviderTypeAnthropic:
		if ep.APIKey != "" {
			req.Header.Set("x-api-key", ep.APIKey)
		}
		req.Header.Set("anthropic-version", "2023-06-01")
	case ProviderTypeOpenAI, ProviderTypeMLX:
		if ep.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+ep.APIKey)
		}
	case ProviderTypeOllama:
		// Local Ollama typically doesn't require auth.
	}
}

// dispatchError wraps a transport-layer error (endpoint missing,
// adapter missing, network failure) with the MT-CONN-009 diagnostic.
func dispatchError(endpointName string, cause error) error {
	diag := protocol.Diagnostic{
		Code:     protocol.DiagProviderUnavailable,
		Category: "provider",
		Cause:    cause.Error(),
	}
	raw, _ := json.Marshal(diag)
	return &TransportError{
		Code:    CodeProviderError,
		Message: fmt.Sprintf("provider %q: %s", endpointName, cause.Error()),
		Data:    json.RawMessage(raw),
	}
}

// formatError wraps FormatMessages failures. ErrWindowTooSmall is a
// caller-addressable condition; other format errors pass through.
func formatError(endpointName string, cause error) error {
	diag := protocol.Diagnostic{Code: protocol.DiagProviderUnavailable, Category: "format"}
	if errors.Is(cause, provider.ErrWindowTooSmall) {
		diag.Category = "budget"
	}
	diag.Cause = cause.Error()
	raw, _ := json.Marshal(diag)
	return &TransportError{
		Code:    CodeProviderError,
		Message: fmt.Sprintf("provider %q format: %s", endpointName, cause.Error()),
		Data:    json.RawMessage(raw),
	}
}

// httpError wraps non-2xx HTTP responses from the upstream provider.
func httpError(endpointName string, status int, snippet string) error {
	diag := protocol.Diagnostic{
		Code:     protocol.DiagProviderUnavailable,
		Category: "provider",
		Cause:    fmt.Sprintf("HTTP %d: %s", status, snippet),
	}
	raw, _ := json.Marshal(diag)
	return &TransportError{
		Code:    CodeProviderError,
		Message: fmt.Sprintf("provider %q returned HTTP %d", endpointName, status),
		Data:    json.RawMessage(raw),
	}
}
