package bff

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
)

// stubAdapter satisfies provider.Provider for dispatch tests. It emits
// a deterministic request body so the HTTP handler can verify the
// outbound payload.
type stubAdapter struct {
	name     string
	format   []byte
	formatEr error
}

func (a *stubAdapter) Name() string                                                       { return a.name }
func (a *stubAdapter) Detect(_ *http.Request) bool                                        { return false }
func (a *stubAdapter) ParseRequest(_ []byte, _ http.Header) (*provider.RequestMetadata, error) {
	return &provider.RequestMetadata{}, nil
}
func (a *stubAdapter) ParseResponse(_ []byte, _ http.Header, _ int) (*provider.ResponseMetadata, error) {
	return &provider.ResponseMetadata{}, nil
}
func (a *stubAdapter) ReassembleStream(_ []provider.StreamChunk) (*provider.ResponseMetadata, string, error) {
	return &provider.ResponseMetadata{}, "", nil
}
func (a *stubAdapter) FormatMessages(_ provider.FormatMessagesOpts) ([]byte, error) {
	if a.formatEr != nil {
		return nil, a.formatEr
	}
	return a.format, nil
}
func (a *stubAdapter) FormatToolDefinitions(_ []protocol.ToolDefinition) ([]byte, error) {
	return []byte("[]"), nil
}
func (a *stubAdapter) ParseStreamEvent(_ []byte) (*provider.StreamEvent, error) {
	return nil, nil
}

func newDispatchServer(t *testing.T, handler http.HandlerFunc) (*TurnDispatcher, *ProviderEndpoint, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	pr := NewProviderRegistry()
	ep := &ProviderEndpoint{Name: "ant", Type: ProviderTypeAnthropic, APIKey: "sk-test", Host: srv.URL}
	if err := pr.Add(ep); err != nil {
		t.Fatalf("Add: %v", err)
	}

	ar := provider.NewRegistry()
	ar.Register(&stubAdapter{name: ProviderTypeAnthropic, format: []byte(`{"x":1}`)})

	d := NewTurnDispatcher(pr, ar)
	d.SetHTTPClient(srv.Client())
	return d, ep, srv
}

func TestTurnDispatcher_Dispatch_Success(t *testing.T) {
	var gotBody []byte
	var gotHeaders http.Header
	d, _, _ := newDispatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	conv := NewConversation("sess")
	conv.appendMessageForTest("user", "hi")

	resp, err := d.Dispatch(context.Background(), DispatchOpts{
		Conversation: conv,
		EndpointName: "ant",
		Model:        "claude-sonnet-4-6",
		MaxTokens:    1024,
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	defer resp.Body.Close()

	if string(gotBody) != `{"x":1}` {
		t.Errorf("upstream body = %q", gotBody)
	}
	if gotHeaders.Get("x-api-key") != "sk-test" {
		t.Errorf("x-api-key header = %q", gotHeaders.Get("x-api-key"))
	}
	if gotHeaders.Get("anthropic-version") == "" {
		t.Errorf("anthropic-version header missing")
	}
	if gotHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", gotHeaders.Get("Content-Type"))
	}
}

func TestTurnDispatcher_Dispatch_UnknownEndpoint(t *testing.T) {
	d := NewTurnDispatcher(NewProviderRegistry(), provider.NewRegistry())
	_, err := d.Dispatch(context.Background(), DispatchOpts{
		Conversation: NewConversation("s"),
		EndpointName: "does-not-exist",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeProviderError {
		t.Errorf("expected CodeProviderError, got %T %v", err, err)
	}
}

func TestTurnDispatcher_Dispatch_NoAdapter(t *testing.T) {
	pr := NewProviderRegistry()
	_ = pr.Add(&ProviderEndpoint{Name: "ep", Type: ProviderTypeAnthropic, APIKey: "k"})
	d := NewTurnDispatcher(pr, provider.NewRegistry()) // empty adapter registry

	_, err := d.Dispatch(context.Background(), DispatchOpts{
		Conversation: NewConversation("s"),
		EndpointName: "ep",
	})
	if err == nil {
		t.Fatalf("expected adapter-missing error")
	}
}

func TestTurnDispatcher_Dispatch_FormatError_WindowTooSmall(t *testing.T) {
	pr := NewProviderRegistry()
	_ = pr.Add(&ProviderEndpoint{Name: "ep", Type: ProviderTypeAnthropic, APIKey: "k", Host: "http://unused"})
	ar := provider.NewRegistry()
	ar.Register(&stubAdapter{name: ProviderTypeAnthropic, formatEr: provider.ErrWindowTooSmall})

	d := NewTurnDispatcher(pr, ar)
	_, err := d.Dispatch(context.Background(), DispatchOpts{
		Conversation: NewConversation("s"),
		EndpointName: "ep",
	})
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeProviderError {
		t.Errorf("expected CodeProviderError for window-too-small, got %T %v", err, err)
	}
	var diag protocol.Diagnostic
	if te.Data != nil {
		raw, ok := te.Data.(json.RawMessage)
		if !ok {
			t.Fatalf("te.Data = %T", te.Data)
		}
		if err := json.Unmarshal(raw, &diag); err != nil {
			t.Fatalf("unmarshal diag: %v", err)
		}
		if diag.Category != "budget" {
			t.Errorf("diagnostic category = %q, want budget", diag.Category)
		}
	}
}

func TestTurnDispatcher_Dispatch_HTTPError(t *testing.T) {
	d, _, _ := newDispatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad input"}`))
	})
	_, err := d.Dispatch(context.Background(), DispatchOpts{
		Conversation: NewConversation("s"),
		EndpointName: "ant",
	})
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeProviderError {
		t.Errorf("expected CodeProviderError on HTTP 400, got %T %v", err, err)
	}
}

func TestTurnDispatcher_DispatchSync_ReadsBody(t *testing.T) {
	d, _, _ := newDispatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_01","content":"hi"}`))
	})

	body, err := d.DispatchSync(context.Background(), DispatchOpts{
		Conversation: NewConversation("s"),
		EndpointName: "ant",
	})
	if err != nil {
		t.Fatalf("DispatchSync: %v", err)
	}
	if string(body) != `{"id":"msg_01","content":"hi"}` {
		t.Errorf("body = %q", body)
	}
}

func TestProviderEndpointPath(t *testing.T) {
	cases := map[string]string{
		ProviderTypeAnthropic: "/v1/messages",
		ProviderTypeOpenAI:    "/v1/chat/completions",
		ProviderTypeOllama:    "/api/chat",
		ProviderTypeMLX:       "/v1/chat/completions",
	}
	for pt, want := range cases {
		if got := providerEndpointPath(pt); got != want {
			t.Errorf("providerEndpointPath(%q) = %q, want %q", pt, got, want)
		}
	}
}
