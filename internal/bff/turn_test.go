package bff

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
)

// turnAdapter is a streamingAdapter wired to behave like a real
// provider for handleTurnSubmit end-to-end coverage. It returns the
// canned SSE script set on the underlying httptest server.
type turnAdapter struct {
	stubAdapter
	body []byte
}

func (a *turnAdapter) Name() string { return ProviderTypeAnthropic }
func (a *turnAdapter) FormatMessages(_ provider.FormatMessagesOpts) ([]byte, error) {
	return a.body, nil
}
func (a *turnAdapter) FormatToolDefinitions(_ []protocol.ToolDefinition) ([]byte, error) {
	return []byte("[]"), nil
}
func (a *turnAdapter) ParseStreamEvent(data []byte) (*provider.StreamEvent, error) {
	switch string(data) {
	case "done":
		return &provider.StreamEvent{Type: provider.StreamEventDone}, nil
	case "":
		return nil, nil
	default:
		return &provider.StreamEvent{Type: provider.StreamEventText, Content: string(data)}, nil
	}
}

func setupTurnSubmitServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	srv := newServerWithRealStore(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: hello\n\ndata: world\n\ndata: done\n\n"))
	}))
	t.Cleanup(upstream.Close)

	_ = srv.providers.Add(&ProviderEndpoint{
		Name: "ant", Type: ProviderTypeAnthropic, APIKey: "k", Host: upstream.URL,
	})
	srv.adapters.Register(&turnAdapter{body: []byte(`{}`)})
	srv.dispatch.SetHTTPClient(upstream.Client())
	srv.models.Refresh()
	srv.routing.Replace(protocol.RoutingPolicy{
		"default": rawJSON(t, "claude-sonnet-4-6"),
	})
	return srv, upstream
}

func TestHandleTurnSubmit_HappyPath(t *testing.T) {
	srv, _ := setupTurnSubmitServer(t)

	c, frames := newRelayConnection(t, srv)
	c.SetSessionID("sess-1")
	srv.sessions.EnsureActive("sess-1", c)

	submit := &protocol.TurnSubmit{
		TurnID:    "turn-1",
		SessionID: "sess-1",
		Sequence:  1,
		Mode:      protocol.ModeBuild,
		Content:   "say hi",
	}
	params, _ := json.Marshal(submit)

	resp, err := handleTurnSubmit(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleTurnSubmit: %v", err)
	}
	tr := resp.(*protocol.TurnSubmitResponse)
	if tr.Status != "accepted" || tr.TurnID != "turn-1" {
		t.Errorf("response = %+v", tr)
	}
	if tr.RunID == "" {
		t.Fatalf("RunID not set in turn.submit response")
	}

	// Wait for streaming to fire token.delta and turn.complete frames.
	if started := frames.waitForFrame(t, protocol.EventRunStarted); len(started) == 0 {
		t.Errorf("no run.started")
	}
	deltas := frames.waitForFrame(t, protocol.EventTokenDelta)
	if len(deltas) == 0 {
		t.Errorf("no token.delta")
	}
	var delta protocol.TokenDelta
	if err := json.Unmarshal(deltas[0], &delta); err != nil {
		t.Fatalf("decode token.delta: %v", err)
	}
	if delta.TurnID != "turn-1" || delta.RunID != tr.RunID {
		t.Fatalf("token.delta ids = turn:%q run:%q, want turn-1/%s", delta.TurnID, delta.RunID, tr.RunID)
	}
	complete := frames.waitForFrame(t, protocol.EventTurnComplete)
	if len(complete) != 1 {
		t.Errorf("turn.complete count = %d", len(complete))
	}
	var turnComplete protocol.TurnComplete
	if err := json.Unmarshal(complete[0], &turnComplete); err != nil {
		t.Fatalf("decode turn.complete: %v", err)
	}
	if turnComplete.RunID != tr.RunID {
		t.Fatalf("turn.complete run_id = %q, want %q", turnComplete.RunID, tr.RunID)
	}
	if runComplete := frames.waitForFrame(t, protocol.EventRunCompleted); len(runComplete) != 1 {
		t.Errorf("run.completed count = %d", len(runComplete))
	}

	// User turn should be persisted in storage.
	turns, _ := srv.store.ListTurns(context.Background(), "sess-1")
	if len(turns) < 1 {
		t.Errorf("expected at least 1 persisted turn, got %d", len(turns))
	}
	run, err := srv.store.GetRun(context.Background(), tr.RunID)
	if err != nil {
		t.Fatalf("GetRun(%s): %v", tr.RunID, err)
	}
	if run.WorkflowType != "implementation" || run.Status != "completed" {
		t.Errorf("run = %+v", run)
	}
}

func TestHandleTurnSubmit_MissingSession(t *testing.T) {
	srv, _ := setupTurnSubmitServer(t)
	c, _ := newRelayConnection(t, srv)
	submit := &protocol.TurnSubmit{
		TurnID: "x", Sequence: 1, Mode: protocol.ModeBuild, Content: "hi",
	}
	params, _ := json.Marshal(submit)
	resp, err := handleTurnSubmit(context.Background(), c, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ack, ok := resp.(*protocol.TurnSubmitResponse)
	if !ok {
		t.Fatalf("expected TurnSubmitResponse, got %T", resp)
	}
	if ack.TurnID == "" {
		t.Errorf("TurnID not set")
	}
}

func TestHandleTurnSubmit_UnknownModelInRouting(t *testing.T) {
	srv := newServerWithRealStore(t)
	srv.routing.Replace(protocol.RoutingPolicy{
		"default": rawJSON(t, "no-such-model"),
	})

	c, _ := newRelayConnection(t, srv)
	c.SetSessionID("sess-1")
	srv.sessions.EnsureActive("sess-1", c)

	submit := &protocol.TurnSubmit{
		TurnID: "t1", SessionID: "sess-1", Sequence: 1, Mode: protocol.ModeBuild, Content: "hi",
	}
	params, _ := json.Marshal(submit)
	_, err := handleTurnSubmit(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected error for unknown model")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeModelUnavailable {
		t.Errorf("expected CodeModelUnavailable, got %T %v", err, err)
	}
}

func TestHandleTurnSubmit_MultiModelRejected(t *testing.T) {
	srv, _ := setupTurnSubmitServer(t)
	srv.routing.Replace(protocol.RoutingPolicy{
		"default": rawJSON(t, []string{"claude-sonnet-4-6", "gpt-5"}),
	})

	c, _ := newRelayConnection(t, srv)
	c.SetSessionID("sess-1")
	srv.sessions.EnsureActive("sess-1", c)

	submit := &protocol.TurnSubmit{
		TurnID: "t1", SessionID: "sess-1", Sequence: 1, Mode: protocol.ModeBuild, Content: "hi",
	}
	params, _ := json.Marshal(submit)
	_, err := handleTurnSubmit(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected multi-model rejection")
	}
}

func TestHandleTurnCancel_KnownTurn(t *testing.T) {
	srv, _ := setupTurnSubmitServer(t)
	srv.turns.register("t1", func() {})
	params, _ := json.Marshal(&protocol.TurnCancel{TurnID: "t1"})
	c, _ := newRelayConnection(t, srv)
	resp, err := handleTurnCancel(context.Background(), c, params)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tr := resp.(*protocol.TurnCancelResponse)
	if !tr.Accepted {
		t.Errorf("expected Accepted=true")
	}
}

func TestHandleTurnCancel_UnknownTurn(t *testing.T) {
	srv, _ := setupTurnSubmitServer(t)
	c, _ := newRelayConnection(t, srv)
	params, _ := json.Marshal(&protocol.TurnCancel{TurnID: "unknown"})
	resp, err := handleTurnCancel(context.Background(), c, params)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tr := resp.(*protocol.TurnCancelResponse)
	if tr.Accepted {
		t.Errorf("unknown turn should not be Accepted")
	}
}

func TestHandleTurnSubmit_RegistersStandardHandlers(t *testing.T) {
	srv := newServerWithRealStore(t)
	for _, m := range []string{
		protocol.MethodTurnSubmit,
		protocol.MethodTurnCancel,
		protocol.MethodToolResult,
		protocol.MethodRunList,
		protocol.MethodRunDetails,
		protocol.MethodRunAttach,
		protocol.MethodRunDetach,
		protocol.MethodRunCancel,
	} {
		if _, ok := srv.dispatcher.handlers[m]; !ok {
			t.Errorf("handler %q not registered", m)
		}
	}
}

// ensure unused-string-import flag stays clean if strings becomes
// unused later
var _ = strings.Repeat
