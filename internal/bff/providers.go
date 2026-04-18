package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ProviderStatus is the health state of a configured provider endpoint.
type ProviderStatus string

const (
	ProviderStatusReady       ProviderStatus = "ready"
	ProviderStatusUnavailable ProviderStatus = "unavailable"
	ProviderStatusError       ProviderStatus = "error"
)

// Well-known provider type strings accepted in config.
const (
	ProviderTypeAnthropic = "anthropic"
	ProviderTypeOpenAI    = "openai"
	ProviderTypeOllama    = "ollama"
	ProviderTypeMLX       = "mlx"
)

// Default health-check timings per FEAT-0008 design D2.3.
const (
	defaultHealthCheckTimeout  = 5 * time.Second
	defaultHealthCheckInterval = 60 * time.Second
)

// ProviderEndpoint is a configured provider instance with runtime
// health/discovery state. Name is the user-assigned identifier
// (e.g. "anthropic-prod"); Type selects the adapter behavior.
type ProviderEndpoint struct {
	Name     string
	Type     string
	APIKey   string
	Host     string
	Discover bool

	mu        sync.RWMutex
	status    ProviderStatus
	errMsg    string
	models    []string
	lastCheck time.Time
}

// Status returns the current health status.
func (e *ProviderEndpoint) Status() ProviderStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.status == "" {
		return ProviderStatusUnavailable
	}
	return e.status
}

// ErrorMessage returns the last error message, if any.
func (e *ProviderEndpoint) ErrorMessage() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.errMsg
}

// Models returns a copy of the discovered model list.
func (e *ProviderEndpoint) Models() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]string, len(e.models))
	copy(out, e.models)
	return out
}

// LastCheck returns the timestamp of the last health check.
func (e *ProviderEndpoint) LastCheck() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastCheck
}

func (e *ProviderEndpoint) setStatus(s ProviderStatus, errMsg string, models []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.status = s
	e.errMsg = errMsg
	if models != nil {
		e.models = models
	}
	e.lastCheck = time.Now().UTC()
}

// ProviderRegistry manages configured provider endpoints and their
// health. Safe for concurrent use.
type ProviderRegistry struct {
	mu        sync.RWMutex
	endpoints map[string]*ProviderEndpoint
	order     []string // insertion order — used for duplicate-resolution

	httpClient *http.Client

	pollCtx    context.Context
	pollCancel context.CancelFunc
	pollWG     sync.WaitGroup
}

// NewProviderRegistry returns an empty ProviderRegistry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		endpoints:  make(map[string]*ProviderEndpoint),
		httpClient: &http.Client{Timeout: defaultHealthCheckTimeout},
	}
}

// SetHTTPClient overrides the default health-check HTTP client. Used by
// tests that hand in an httptest.Server.Client().
func (r *ProviderRegistry) SetHTTPClient(c *http.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.httpClient = c
}

// Add registers a new endpoint. Duplicate names are rejected.
func (r *ProviderRegistry) Add(ep *ProviderEndpoint) error {
	if ep.Name == "" {
		return errors.New("provider endpoint: name is required")
	}
	if !isValidProviderType(ep.Type) {
		return fmt.Errorf("provider endpoint %q: unsupported type %q", ep.Name, ep.Type)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.endpoints[ep.Name]; exists {
		return fmt.Errorf("provider endpoint %q already registered", ep.Name)
	}
	// Apply defaults for well-known hosts.
	if ep.Host == "" {
		ep.Host = defaultHostFor(ep.Type)
	}
	// Expand ${ENV_VAR} in API key.
	ep.APIKey = ExpandEnvAPIKey(ep.APIKey)

	r.endpoints[ep.Name] = ep
	r.order = append(r.order, ep.Name)
	return nil
}

// Get returns the endpoint with the given name, or nil if not found.
func (r *ProviderRegistry) Get(name string) *ProviderEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.endpoints[name]
}

// All returns all registered endpoints in insertion order.
func (r *ProviderRegistry) All() []*ProviderEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ProviderEndpoint, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.endpoints[name])
	}
	return out
}

// Names returns the registered endpoint names in insertion order.
func (r *ProviderRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// CheckEndpoint probes a single endpoint and updates its status. Safe
// for concurrent calls; each call does one round of HTTP probing.
func (r *ProviderRegistry) CheckEndpoint(ctx context.Context, ep *ProviderEndpoint) {
	switch ep.Type {
	case ProviderTypeAnthropic, ProviderTypeOpenAI:
		r.checkCloudEndpoint(ctx, ep)
	case ProviderTypeOllama:
		r.checkOllama(ctx, ep)
	case ProviderTypeMLX:
		r.checkMLX(ctx, ep)
	default:
		ep.setStatus(ProviderStatusError, fmt.Sprintf("unknown provider type %q", ep.Type), nil)
	}
}

// CheckAll probes every endpoint concurrently.
func (r *ProviderRegistry) CheckAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, ep := range r.All() {
		wg.Add(1)
		go func(e *ProviderEndpoint) {
			defer wg.Done()
			r.CheckEndpoint(ctx, e)
		}(ep)
	}
	wg.Wait()
}

// StartHealthChecks runs an initial synchronous CheckAll and then polls
// at the given interval until Stop is called.
func (r *ProviderRegistry) StartHealthChecks(interval time.Duration) {
	if interval <= 0 {
		interval = defaultHealthCheckInterval
	}
	r.pollCtx, r.pollCancel = context.WithCancel(context.Background())
	r.CheckAll(r.pollCtx)

	r.pollWG.Add(1)
	go func() {
		defer r.pollWG.Done()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-r.pollCtx.Done():
				return
			case <-t.C:
				r.CheckAll(r.pollCtx)
			}
		}
	}()
}

// Stop halts the background health check loop.
func (r *ProviderRegistry) Stop() {
	if r.pollCancel != nil {
		r.pollCancel()
	}
	r.pollWG.Wait()
}

// -----------------------------------------------------------------------
// Per-type probes
// -----------------------------------------------------------------------

func (r *ProviderRegistry) checkCloudEndpoint(ctx context.Context, ep *ProviderEndpoint) {
	if ep.APIKey == "" {
		ep.setStatus(ProviderStatusUnavailable, "api_key is empty", nil)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, ep.Host, nil)
	if err != nil {
		ep.setStatus(ProviderStatusError, err.Error(), nil)
		return
	}
	switch ep.Type {
	case ProviderTypeAnthropic:
		req.Header.Set("x-api-key", ep.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case ProviderTypeOpenAI:
		req.Header.Set("Authorization", "Bearer "+ep.APIKey)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		ep.setStatus(ProviderStatusUnavailable, err.Error(), nil)
		return
	}
	defer resp.Body.Close()
	// 2xx or 401 both indicate the endpoint is reachable (401 means auth
	// works at the TLS level; the provider will accept /v1/messages).
	// 404/405 on HEAD also indicates reachable-but-method-rejected.
	if resp.StatusCode < 500 {
		ep.setStatus(ProviderStatusReady, "", nil)
		return
	}
	ep.setStatus(ProviderStatusError, fmt.Sprintf("HTTP %d", resp.StatusCode), nil)
}

func (r *ProviderRegistry) checkOllama(ctx context.Context, ep *ProviderEndpoint) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(ep.Host, "/")+"/api/tags", nil)
	if err != nil {
		ep.setStatus(ProviderStatusError, err.Error(), nil)
		return
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		ep.setStatus(ProviderStatusUnavailable, err.Error(), nil)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		ep.setStatus(ProviderStatusError, fmt.Sprintf("HTTP %d", resp.StatusCode), nil)
		return
	}
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		ep.setStatus(ProviderStatusError, "decode /api/tags: "+err.Error(), nil)
		return
	}
	names := make([]string, 0, len(body.Models))
	for _, m := range body.Models {
		names = append(names, m.Name)
	}
	ep.setStatus(ProviderStatusReady, "", names)
}

func (r *ProviderRegistry) checkMLX(ctx context.Context, ep *ProviderEndpoint) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(ep.Host, "/")+"/v1/models", nil)
	if err != nil {
		ep.setStatus(ProviderStatusError, err.Error(), nil)
		return
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		ep.setStatus(ProviderStatusUnavailable, err.Error(), nil)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		ep.setStatus(ProviderStatusError, fmt.Sprintf("HTTP %d", resp.StatusCode), nil)
		return
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		ep.setStatus(ProviderStatusError, "decode /v1/models: "+err.Error(), nil)
		return
	}
	names := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		names = append(names, m.ID)
	}
	ep.setStatus(ProviderStatusReady, "", names)
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

func isValidProviderType(t string) bool {
	switch t {
	case ProviderTypeAnthropic, ProviderTypeOpenAI, ProviderTypeOllama, ProviderTypeMLX:
		return true
	}
	return false
}

func defaultHostFor(t string) string {
	switch t {
	case ProviderTypeAnthropic:
		return "https://api.anthropic.com"
	case ProviderTypeOpenAI:
		return "https://api.openai.com"
	case ProviderTypeOllama:
		return "http://localhost:11434"
	case ProviderTypeMLX:
		return "http://localhost:8080"
	}
	return ""
}

// ExpandEnvAPIKey resolves ${ENV_VAR} placeholders at the start of an
// API key value. A value not wrapped in ${...} is returned unchanged.
// Missing env vars return "" so the endpoint ends up marked unavailable
// on health check.
func ExpandEnvAPIKey(v string) string {
	if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		name := v[2 : len(v)-1]
		return lookupEnv(name)
	}
	return v
}

// lookupEnv is a thin wrapper over os.Getenv extracted for test hooks.
var lookupEnv = os.Getenv

// MaskAPIKey returns a masked form of an API key suitable for logs or
// protocol events: first 7 chars and last 4 chars are retained, the
// middle is replaced with asterisks. Short keys collapse to asterisks.
func MaskAPIKey(key string) string {
	if len(key) <= 11 {
		return strings.Repeat("*", len(key))
	}
	return key[:7] + "****" + key[len(key)-4:]
}
