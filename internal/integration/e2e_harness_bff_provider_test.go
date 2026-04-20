package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/bff"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// TestE2E_HarnessToBFFToMockProvider is the WU-088 end-to-end pass.
// It composes the full modeltap stack short of real networking:
// harness ConnectionManager → unix-socket BFF → TurnDispatcher →
// httptest mock upstream. The goal is to prove that a turn.submit
// from the harness reaches the upstream and that the BFF returns a
// TurnSubmitResponse with an assigned TurnID, not to assert on every
// streaming event (that's covered at the BFF layer in turn_test.go).
func TestE2E_HarnessToBFFToMockProvider(t *testing.T) {
	// ---- upstream mock provider ----
	var gotCalls int32
	recorded := make(chan []byte, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCalls++
		body := make([]byte, 0, 1024)
		buf := make([]byte, 1024)
		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				body = append(body, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		select {
		case recorded <- body:
		default:
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// Minimal Anthropic-format SSE. One text delta + done.
		_, _ = w.Write([]byte(`event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}

event: message_stop
data: {"type":"message_stop"}

`))
	}))
	t.Cleanup(upstream.Close)

	// ---- BFF with the mock upstream wired in ----
	sockPath := shortSocketPath(t)
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := bff.DefaultServerConfig()
	cfg.SocketPath = sockPath
	cfg.HeartbeatInterval = 50 * time.Millisecond
	cfg.HeartbeatTimeout = 500 * time.Millisecond

	srv := bff.NewServer(store, cfg)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	// Real Anthropic adapter — the mock upstream speaks Anthropic SSE
	// so the streaming relay has a valid parser. Endpoint points at
	// the httptest server.
	srv.Adapters().Register(provider.NewAnthropicProvider())
	if err := srv.Providers().Add(&bff.ProviderEndpoint{
		Name:   "mock",
		Type:   bff.ProviderTypeAnthropic,
		APIKey: "sk-test",
		Host:   upstream.URL,
	}); err != nil {
		t.Fatalf("Add endpoint: %v", err)
	}
	srv.Dispatch().SetHTTPClient(upstream.Client())

	// Register a model in the catalog and bind default routing to it.
	srv.Models().SetManual(map[string]bff.ModelOverrideConfig{
		"claude-sonnet-4-6": {
			Provider:      "mock",
			ContextWindow: 200000,
			Capabilities:  []string{"text"},
			Description:   "integration test model",
		},
	})
	srv.Models().Refresh()
	srv.Routing().Replace(protocol.RoutingPolicy{
		"default": rawDefault(t, "claude-sonnet-4-6"),
	})

	// ---- harness side ----
	sender := &recordingSender{}
	cm := harness.NewConnectionManager(harness.ConnectionConfig{
		SocketPath: sockPath,
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  "test",
			HarnessPlatform: "test",
		},
	}, sender)
	t.Cleanup(cm.Disconnect)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cm.ConnectSync(ctx); err != nil {
		t.Fatalf("ConnectSync: %v", err)
	}

	// Create a session by appending one directly — turn.submit requires
	// an active session that the server recognizes.
	sessionID := "e2e-sess"
	if err := store.CreateSession(ctx, &storage.Session{
		ID: sessionID, UserID: "local", Project: "/tmp/proj",
		Summary: "e2e", Status: "active",
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := cm.Client().SessionResume(ctx, sessionID, protocol.ProjectContext{Root: "/tmp/proj"}); err != nil {
		t.Fatalf("SessionResume: %v", err)
	}

	app := harness.NewApp(harness.AppOptions{Conn: harness.WrapConnectionManager(cm)})
	app.State().SessionID = sessionID

	// Submit the turn via the App path — this exercises the full
	// PATCH-0003 chain: SubmitMsg → dispatchTurnSubmit → SubmitTurn
	// → BFF handleTurnSubmit → TurnDispatcher → upstream.
	_, cmd := app.Update(harness.SubmitMsg{Content: "say hi"})
	if cmd == nil {
		t.Fatal("expected dispatch cmd")
	}
	msg := cmd()
	ack, ok := msg.(harness.TurnSubmittedMsg)
	if !ok {
		t.Fatalf("expected TurnSubmittedMsg, got %T (%+v)", msg, msg)
	}
	if ack.Err != nil {
		t.Fatalf("TurnSubmittedMsg.Err: %v", ack.Err)
	}
	if ack.TurnID == "" {
		t.Error("TurnID should be populated on a successful ack")
	}

	// The upstream should have received one request within the timeout.
	select {
	case body := <-recorded:
		if len(body) == 0 {
			t.Error("upstream request body was empty")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("upstream never saw a request (calls=%d)", gotCalls)
	}

	// Streaming events must reach the harness sender after the relay
	// finishes parsing the mock SSE. Poll the recorder up to 2s for a
	// StreamTokenMsg carrying the mock's "hi" delta plus a
	// StreamCompleteMsg closing the turn. (model.selected is only
	// emitted by /model switch today — not by every turn.submit — so
	// we don't assert on it here.)
	deadline := time.Now().Add(2 * time.Second)
	var sawToken, sawComplete bool
	for time.Now().Before(deadline) && !(sawToken && sawComplete) {
		for _, m := range sender.snapshot() {
			switch v := m.(type) {
			case harness.StreamTokenMsg:
				if v.TurnID == ack.TurnID && v.Delta == "hi" {
					sawToken = true
				}
			case harness.StreamCompleteMsg:
				if v.TurnID == ack.TurnID {
					sawComplete = true
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !sawToken {
		t.Errorf("expected StreamTokenMsg(TurnID=%q, Delta=\"hi\") within 2s", ack.TurnID)
	}
	if !sawComplete {
		t.Errorf("expected StreamCompleteMsg(TurnID=%q) within 2s", ack.TurnID)
	}
}

// rawDefault marshals a single-model routing value (wire shape: a
// bare string, not an array).
func rawDefault(t *testing.T, model string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}
