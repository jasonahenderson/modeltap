package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// mockProvider implements Provider for testing purposes.
type mockProvider struct {
	name      string
	detectFn  func(r *http.Request) bool
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Detect(r *http.Request) bool {
	if m.detectFn != nil {
		return m.detectFn(r)
	}
	return false
}

func (m *mockProvider) ParseRequest(body []byte, headers http.Header) (*RequestMetadata, error) {
	return &RequestMetadata{}, nil
}

func (m *mockProvider) ParseResponse(body []byte, headers http.Header, statusCode int) (*ResponseMetadata, error) {
	return &ResponseMetadata{}, nil
}

func (m *mockProvider) ReassembleStream(chunks []StreamChunk) (*ResponseMetadata, string, error) {
	return &ResponseMetadata{}, "", nil
}

func (m *mockProvider) FormatMessages(opts FormatMessagesOpts) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (m *mockProvider) FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (m *mockProvider) ParseStreamEvent(data []byte) (*StreamEvent, error) {
	return nil, ErrNotImplemented
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if got := len(r.All()); got != 0 {
		t.Errorf("new registry should be empty, got %d providers", got)
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{name: "test-provider"}

	r.Register(p)

	got := r.Get("test-provider")
	if got == nil {
		t.Fatal("Get returned nil for registered provider")
	}
	if got.Name() != "test-provider" {
		t.Errorf("Get returned provider with name %q, want %q", got.Name(), "test-provider")
	}
}

func TestGetUnregistered(t *testing.T) {
	r := NewRegistry()
	if got := r.Get("nonexistent"); got != nil {
		t.Errorf("Get for unregistered provider should return nil, got %v", got)
	}
}

func TestRegisterReplacesExisting(t *testing.T) {
	r := NewRegistry()
	p1 := &mockProvider{name: "dup", detectFn: func(r *http.Request) bool { return false }}
	p2 := &mockProvider{name: "dup", detectFn: func(r *http.Request) bool { return true }}

	r.Register(p1)
	r.Register(p2)

	// Should still have only one provider.
	if got := len(r.All()); got != 1 {
		t.Errorf("expected 1 provider after replacement, got %d", got)
	}

	// The replaced provider should be the new one (detects everything).
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	detected := r.Detect(req)
	if detected == nil {
		t.Fatal("Detect returned nil after replacement")
	}
}

func TestDetectMatchingProvider(t *testing.T) {
	r := NewRegistry()
	anthropic := &mockProvider{
		name: "anthropic",
		detectFn: func(r *http.Request) bool {
			return r.Host == "api.anthropic.com"
		},
	}
	r.Register(anthropic)

	req := httptest.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	got := r.Detect(req)
	if got == nil {
		t.Fatal("Detect returned nil for matching request")
	}
	if got.Name() != "anthropic" {
		t.Errorf("Detect returned %q, want %q", got.Name(), "anthropic")
	}
}

func TestDetectNoMatch(t *testing.T) {
	r := NewRegistry()
	anthropic := &mockProvider{
		name: "anthropic",
		detectFn: func(r *http.Request) bool {
			return r.Host == "api.anthropic.com"
		},
	}
	r.Register(anthropic)

	req := httptest.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", nil)
	got := r.Detect(req)
	if got != nil {
		t.Errorf("Detect should return nil for non-matching request, got %q", got.Name())
	}
}

func TestDetectMultipleProviders(t *testing.T) {
	r := NewRegistry()

	anthropic := &mockProvider{
		name: "anthropic",
		detectFn: func(r *http.Request) bool {
			return r.Host == "api.anthropic.com"
		},
	}
	openai := &mockProvider{
		name: "openai",
		detectFn: func(r *http.Request) bool {
			return r.Host == "api.openai.com"
		},
	}

	r.Register(anthropic)
	r.Register(openai)

	tests := []struct {
		url      string
		wantName string
	}{
		{"https://api.anthropic.com/v1/messages", "anthropic"},
		{"https://api.openai.com/v1/chat/completions", "openai"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, tt.url, nil)
		got := r.Detect(req)
		if got == nil {
			t.Errorf("Detect(%s) returned nil, want %q", tt.url, tt.wantName)
			continue
		}
		if got.Name() != tt.wantName {
			t.Errorf("Detect(%s) = %q, want %q", tt.url, got.Name(), tt.wantName)
		}
	}
}

func TestDetectReturnsFirstMatch(t *testing.T) {
	r := NewRegistry()

	// Both providers match everything; first registered should win.
	first := &mockProvider{
		name:     "first",
		detectFn: func(r *http.Request) bool { return true },
	}
	second := &mockProvider{
		name:     "second",
		detectFn: func(r *http.Request) bool { return true },
	}

	r.Register(first)
	r.Register(second)

	req := httptest.NewRequest(http.MethodPost, "/anything", nil)
	got := r.Detect(req)
	if got == nil {
		t.Fatal("Detect returned nil")
	}
	if got.Name() != "first" {
		t.Errorf("Detect should return first matching provider, got %q", got.Name())
	}
}

func TestAllReturnsRegistrationOrder(t *testing.T) {
	r := NewRegistry()

	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		r.Register(&mockProvider{name: name})
	}

	all := r.All()
	if len(all) != len(names) {
		t.Fatalf("All returned %d providers, want %d", len(all), len(names))
	}
	for i, p := range all {
		if p.Name() != names[i] {
			t.Errorf("All()[%d] = %q, want %q", i, p.Name(), names[i])
		}
	}
}

func TestAllReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "original"})

	all := r.All()
	all[0] = &mockProvider{name: "tampered"}

	// The registry's internal slice should be unaffected.
	got := r.Get("original")
	if got == nil {
		t.Error("modifying All() result should not affect registry")
	}
}

func TestDetectEmptyRegistry(t *testing.T) {
	r := NewRegistry()
	req := httptest.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	if got := r.Detect(req); got != nil {
		t.Errorf("Detect on empty registry should return nil, got %v", got)
	}
}
