package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestContentTransform_HappyPath_AnthropicShape(t *testing.T) {
	srv := newServerWithRealStore(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"Summary: lorem ipsum"}]}`))
	}))
	t.Cleanup(upstream.Close)

	_ = srv.providers.Add(&ProviderEndpoint{Name: "ant", Type: ProviderTypeAnthropic, APIKey: "k", Host: upstream.URL})
	srv.adapters.Register(&turnAdapter{body: []byte(`{}`)})
	srv.dispatch.SetHTTPClient(upstream.Client())
	srv.models.Refresh()
	srv.routing.Replace(protocol.RoutingPolicy{
		"cheap":   rawJSON(t, "claude-haiku-4-5"),
		"default": rawJSON(t, "claude-sonnet-4-6"),
	})

	c, _ := newRelayConnection(t, srv)
	params, _ := json.Marshal(&protocol.ContentTransform{
		Transform:       "summarize",
		RawContent:      "the original long content here",
		ContentType:     "text/plain",
		MaxOutputTokens: 256,
	})
	resp, err := handleContentTransform(context.Background(), c, params)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	tr := resp.(*protocol.ContentTransformResponse)
	if tr.Content != "Summary: lorem ipsum" {
		t.Errorf("content = %q", tr.Content)
	}
	if tr.ModelUsed != "claude-haiku-4-5" {
		t.Errorf("model = %q", tr.ModelUsed)
	}
}

func TestContentTransform_OpenAIShape(t *testing.T) {
	srv := newServerWithRealStore(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"oai summary"}}]}`))
	}))
	t.Cleanup(upstream.Close)

	_ = srv.providers.Add(&ProviderEndpoint{Name: "oai", Type: ProviderTypeOpenAI, APIKey: "k", Host: upstream.URL})
	srv.adapters.Register(&turnAdapter{body: []byte(`{}`)}) // Anthropic-typed adapter; we override below.
	// Actually the adapter is selected by Endpoint.Type via Server.adapterFor;
	// here we register a dedicated OpenAI-typed stub.
	srv.adapters.Register(&openaiTransformAdapter{})
	srv.dispatch.SetHTTPClient(upstream.Client())
	srv.models.Refresh()
	srv.routing.Replace(protocol.RoutingPolicy{
		"default": rawJSON(t, "gpt-5"),
	})

	c, _ := newRelayConnection(t, srv)
	params, _ := json.Marshal(&protocol.ContentTransform{
		Transform: "summarize", RawContent: "in",
	})
	resp, err := handleContentTransform(context.Background(), c, params)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	tr := resp.(*protocol.ContentTransformResponse)
	if tr.Content != "oai summary" {
		t.Errorf("content = %q", tr.Content)
	}
}

func TestContentTransform_MissingTransform(t *testing.T) {
	srv := newServerWithRealStore(t)
	c, _ := newRelayConnection(t, srv)
	params, _ := json.Marshal(&protocol.ContentTransform{RawContent: "x"})
	_, err := handleContentTransform(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestContentTransform_NoRoutingTarget(t *testing.T) {
	srv := newServerWithRealStore(t)
	c, _ := newRelayConnection(t, srv)
	params, _ := json.Marshal(&protocol.ContentTransform{
		Transform: "summarize", RawContent: "x",
	})
	_, err := handleContentTransform(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeModelUnavailable {
		t.Errorf("expected CodeModelUnavailable, got %T %v", err, err)
	}
}

// openaiTransformAdapter satisfies the Provider interface enough for
// the transform path: only FormatMessages and ParseStreamEvent are
// invoked, and we only need the former here (transform uses
// DispatchSync, which doesn't stream).
type openaiTransformAdapter struct{ stubAdapter }

func (a *openaiTransformAdapter) Name() string { return ProviderTypeOpenAI }
