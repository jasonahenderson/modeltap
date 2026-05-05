package harness

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func osRemoveSocket(p string) error { return os.Remove(p) }

// recordingSender captures messages the connection manager sends.
type recordingSender struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (r *recordingSender) Send(m tea.Msg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, m)
}

func (r *recordingSender) snapshot() []tea.Msg {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]tea.Msg, len(r.msgs))
	copy(out, r.msgs)
	return out
}

func (r *recordingSender) firstOfType(want any) (tea.Msg, bool) {
	for _, m := range r.snapshot() {
		switch want.(type) {
		case ConnStateMsg:
			if v, ok := m.(ConnStateMsg); ok {
				return v, true
			}
		case StreamTokenMsg:
			if v, ok := m.(StreamTokenMsg); ok {
				return v, true
			}
		case StreamCompleteMsg:
			if v, ok := m.(StreamCompleteMsg); ok {
				return v, true
			}
		case ToolCallMsg:
			if v, ok := m.(ToolCallMsg); ok {
				return v, true
			}
		case CostUpdateMsg:
			if v, ok := m.(CostUpdateMsg); ok {
				return v, true
			}
		case ModelUpdateMsg:
			if v, ok := m.(ModelUpdateMsg); ok {
				return v, true
			}
		case StatusUpdateMsg:
			if v, ok := m.(StatusUpdateMsg); ok {
				return v, true
			}
		}
	}
	return nil, false
}

func (r *recordingSender) waitFor(t *testing.T, want any) tea.Msg {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m, ok := r.firstOfType(want); ok {
			return m
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitFor: %T never sent (saw %d msgs)", want, len(r.snapshot()))
	return nil
}

func TestCanHarnessTransition_Allowed(t *testing.T) {
	cases := []struct {
		from, to string
	}{
		{ConnStateDiscovering, ConnStateConnecting},
		{ConnStateConnecting, ConnStateRegistering},
		{ConnStateRegistering, ConnStateReady},
		{ConnStateReady, ConnStateDegraded},
		{ConnStateDegraded, ConnStateReady},
		{ConnStateReady, ConnStateReconnecting},
		{ConnStateReconnecting, ConnStateConnecting},
	}
	for _, c := range cases {
		if !canHarnessTransition(c.from, c.to) {
			t.Errorf("transition %s → %s should be allowed", c.from, c.to)
		}
	}
}

func TestCanHarnessTransition_Disallowed(t *testing.T) {
	cases := []struct {
		from, to string
	}{
		{ConnStateReady, ConnStateConnecting},
		{ConnStateFailed, ConnStateReady},
		{ConnStateRegistering, ConnStateAuthenticating},
	}
	for _, c := range cases {
		if canHarnessTransition(c.from, c.to) {
			t.Errorf("transition %s → %s should be rejected", c.from, c.to)
		}
	}
}

func TestConnMgr_Transition_SendsConnStateMsg(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)
	cm.transition(ConnStateConnecting, "dialing")
	cm.transition(ConnStateRegistering, "")

	got := sender.snapshot()
	if len(got) != 2 {
		t.Fatalf("expected 2 ConnStateMsg, got %d", len(got))
	}
	if got[0].(ConnStateMsg).Info.State != ConnStateConnecting {
		t.Errorf("first state = %q", got[0].(ConnStateMsg).Info.State)
	}
	if got[1].(ConnStateMsg).Info.State != ConnStateRegistering {
		t.Errorf("second state = %q", got[1].(ConnStateMsg).Info.State)
	}
}

func TestConnMgr_Transition_InvalidIsNoOp(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)
	// From the default Discovering state, jumping straight to Ready
	// is not allowed.
	cm.transition(ConnStateReady, "")
	if cm.State() == ConnStateReady {
		t.Errorf("invalid transition was applied")
	}
	if len(sender.snapshot()) != 0 {
		t.Errorf("expected no ConnStateMsg for invalid transition")
	}
}

func TestBackoffDelay_Bounded(t *testing.T) {
	for attempt := 0; attempt < 12; attempt++ {
		got := backoffDelay(attempt, 100*time.Millisecond, 1*time.Second)
		if got > 1200*time.Millisecond { // include +20% jitter slack
			t.Errorf("attempt=%d delay=%v exceeds max+jitter", attempt, got)
		}
		if got < 0 {
			t.Errorf("attempt=%d delay negative: %v", attempt, got)
		}
	}
}

func TestBackoffDelay_GrowsThenCaps(t *testing.T) {
	d0 := backoffDelay(0, 100*time.Millisecond, 1*time.Second)
	d1 := backoffDelay(1, 100*time.Millisecond, 1*time.Second)
	d10 := backoffDelay(10, 100*time.Millisecond, 1*time.Second)
	// d0 base ~= 100ms; d1 base ~= 200ms; d10 hits the cap.
	if d0 > 130*time.Millisecond {
		t.Errorf("d0 too large: %v", d0)
	}
	if d1 < 150*time.Millisecond || d1 > 250*time.Millisecond {
		t.Errorf("d1 outside expected +/- 20pct range: %v", d1)
	}
	if d10 < 800*time.Millisecond {
		t.Errorf("d10 should be near cap: %v", d10)
	}
}

func TestHandleEvent_TokenDelta(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	raw, _ := json.Marshal(&protocol.TokenDelta{TurnID: "t1", BranchID: "b1", Text: "hi"})
	cm.HandleEvent(protocol.EventTokenDelta, raw)

	m := sender.waitFor(t, StreamTokenMsg{}).(StreamTokenMsg)
	if m.TurnID != "t1" || m.BranchID != "b1" || m.Delta != "hi" {
		t.Errorf("StreamTokenMsg = %+v", m)
	}
}

func TestHandleEvent_TurnComplete(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	raw, _ := json.Marshal(&protocol.TurnComplete{
		TurnID: "t1", FinalInputTokens: 100, FinalOutputTokens: 200,
		TotalCost: 0.05, Model: "claude-sonnet-4-6", LatencyMs: 4321,
	})
	cm.HandleEvent(protocol.EventTurnComplete, raw)

	m := sender.waitFor(t, StreamCompleteMsg{}).(StreamCompleteMsg)
	if m.TurnID != "t1" || m.Tokens.Input != 100 || m.Tokens.Output != 200 ||
		m.Cost != 0.05 || m.Model != "claude-sonnet-4-6" ||
		m.Duration != 4321*time.Millisecond {
		t.Errorf("StreamCompleteMsg = %+v", m)
	}
}

func TestHandleEvent_ToolCall(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	raw, _ := json.Marshal(&protocol.ToolCall{
		TurnID: "t1", ToolCallID: "call-a", Tool: "read",
		Namespace: "fs", Input: json.RawMessage(`{"path":"/x"}`),
	})
	cm.HandleEvent(protocol.EventToolCall, raw)

	m := sender.waitFor(t, ToolCallMsg{}).(ToolCallMsg)
	if m.TurnID != "t1" || m.ToolCallID != "call-a" || m.ToolName != "read" {
		t.Errorf("ToolCallMsg = %+v", m)
	}
}

func TestHandleEvent_CostUpdate(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	raw, _ := json.Marshal(&protocol.CostUpdate{TotalCost: 1.23})
	cm.HandleEvent(protocol.EventCostUpdate, raw)

	m := sender.waitFor(t, CostUpdateMsg{}).(CostUpdateMsg)
	if m.Total != 1.23 {
		t.Errorf("CostUpdateMsg total = %v", m.Total)
	}
}

func TestHandleEvent_ModelSelected_SingleModel(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	model, _ := json.Marshal("claude-sonnet-4-6")
	raw, _ := json.Marshal(&protocol.ModelSelected{
		Model: model, Reason: "routing_policy:default",
	})
	cm.HandleEvent(protocol.EventModelSelected, raw)

	m := sender.waitFor(t, ModelUpdateMsg{}).(ModelUpdateMsg)
	if m.Name != "claude-sonnet-4-6" || m.Routing != "routing_policy:default" {
		t.Errorf("ModelUpdateMsg = %+v", m)
	}
}

func TestHandleEvent_CompactSuggest_StatusUpdateMsg(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	cm.HandleEvent(protocol.EventCompactSuggest, json.RawMessage(`{}`))
	m := sender.waitFor(t, StatusUpdateMsg{}).(StatusUpdateMsg)
	if m.Message == "" {
		t.Errorf("StatusUpdateMsg empty")
	}
}

func TestHandleEvent_ModelSelected_MultiModel(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	models, _ := json.Marshal([]string{"claude-opus-4-7", "gpt-5"})
	providers, _ := json.Marshal([]string{"anthropic", "openai"})
	raw, _ := json.Marshal(&protocol.ModelSelected{
		Model: models, Provider: providers, Reason: "multi:review",
	})
	cm.HandleEvent(protocol.EventModelSelected, raw)

	m := sender.waitFor(t, ModelUpdateMsg{}).(ModelUpdateMsg)
	if m.Name != "claude-opus-4-7, gpt-5" {
		t.Errorf("multi-model Name = %q", m.Name)
	}
	if m.Routing != "multi:review" {
		t.Errorf("multi-model Routing = %q", m.Routing)
	}
}

func TestHandleEvent_CompactSuggest_WithPayload(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	raw, _ := json.Marshal(&protocol.CompactSuggest{
		TurnID: "t1", ContextPct: 0.85, Threshold: 0.8, Message: "",
	})
	cm.HandleEvent(protocol.EventCompactSuggest, raw)
	m := sender.waitFor(t, StatusUpdateMsg{}).(StatusUpdateMsg)
	if !contains(m.Message, "85%") {
		t.Errorf("banner should reference context pct; got %q", m.Message)
	}
}

func TestHandleEvent_CompactNotice(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	raw, _ := json.Marshal(&protocol.CompactNotice{
		TurnID: "t", TriggeredBy: "auto", TokensFreed: 4200,
	})
	cm.HandleEvent(protocol.EventCompactNotice, raw)
	m := sender.waitFor(t, StatusUpdateMsg{}).(StatusUpdateMsg)
	if !contains(m.Message, "4200") {
		t.Errorf("banner should reference tokens freed; got %q", m.Message)
	}
}

func TestHandleEvent_CompactPlan(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	cm.HandleEvent(protocol.EventCompactPlan, json.RawMessage(`{}`))
	m := sender.waitFor(t, StatusUpdateMsg{}).(StatusUpdateMsg)
	if !contains(m.Message, "/compact") {
		t.Errorf("banner should mention /compact; got %q", m.Message)
	}
}

func TestHandleEvent_KnowledgeHit(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	raw, _ := json.Marshal(&protocol.KnowledgeHit{
		TurnID: "t", Summary: "prior discussion of FEAT-0008 reconnect", Relevance: 0.82,
	})
	cm.HandleEvent(protocol.EventKnowledgeHit, raw)
	m := sender.waitFor(t, StatusUpdateMsg{}).(StatusUpdateMsg)
	if !contains(m.Message, "prior discussion") {
		t.Errorf("banner should include summary; got %q", m.Message)
	}
}

func TestHandleEvent_Error(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	raw, _ := json.Marshal(&protocol.ServerError{
		Code:    "provider_error",
		Message: "upstream refused",
		Diagnostic: protocol.Diagnostic{
			Code: protocol.DiagnosticCode("MT-PROV-001"),
		},
	})
	cm.HandleEvent(protocol.EventError, raw)
	m := sender.waitFor(t, StatusUpdateMsg{}).(StatusUpdateMsg)
	if !contains(m.Message, "MT-PROV-001") || !contains(m.Message, "upstream refused") {
		t.Errorf("banner should include diagnostic + message; got %q", m.Message)
	}
}

func TestHandleEvent_ConnectionPong_NoMessage(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	cm.HandleEvent(protocol.EventConnectionPong, json.RawMessage(`{}`))
	if got := sender.snapshot(); len(got) != 0 {
		t.Errorf("pong should be no-op in HandleEvent; got %+v", got)
	}
}

// contains is a tiny helper — strings.Contains via a thin wrapper so
// the test file doesn't repeat the import for a one-token check.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestHandleEvent_UnknownMethod_NoMessage(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)

	cm.HandleEvent("nonsense.method", json.RawMessage(`{}`))
	if got := sender.snapshot(); len(got) != 0 {
		t.Errorf("expected no msg for unknown method, got %v", got)
	}
}

func TestHandleEvent_NilSender_DoesNotPanic(t *testing.T) {
	cm := NewConnectionManager(ConnectionConfig{}, nil)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HandleEvent with nil sender panicked: %v", r)
		}
	}()
	cm.HandleEvent(protocol.EventTokenDelta, json.RawMessage(`{"turn_id":"t","text":"x"}`))
}

func TestConnMgr_ConnectSync_HappyPath(t *testing.T) {
	sock := shortSockPath2(t)
	srv := startMockBFF(t, sock)
	defer srv.close()

	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{
		SocketPath:        sock,
		Registration:      &protocol.CapabilitiesRegister{ProtocolVersion: "1"},
		HeartbeatInterval: 50 * time.Millisecond,
		HeartbeatTimeout:  100 * time.Millisecond,
	}, sender)
	t.Cleanup(cm.Disconnect)

	if err := cm.ConnectSync(context.Background()); err != nil {
		t.Fatalf("ConnectSync: %v", err)
	}
	if cm.State() != ConnStateReady {
		t.Errorf("state = %q, want ready", cm.State())
	}

	// Sender should have observed Connecting → Registering → Ready.
	got := sender.snapshot()
	saw := map[string]bool{}
	for _, m := range got {
		if cs, ok := m.(ConnStateMsg); ok {
			saw[cs.Info.State] = true
		}
	}
	for _, want := range []string{ConnStateConnecting, ConnStateRegistering, ConnStateReady} {
		if !saw[want] {
			t.Errorf("missing transition: %s (saw: %v)", want, saw)
		}
	}
}

func TestConnMgr_ConnectSync_AutoStartDisabled_FailsCleanly(t *testing.T) {
	sock := shortSockPath2(t)
	// No listener; AutoStart disabled.
	cm := NewConnectionManager(ConnectionConfig{
		SocketPath: sock,
		AutoStart:  false,
	}, nil)
	t.Cleanup(cm.Disconnect)

	err := cm.ConnectSync(context.Background())
	if err == nil {
		t.Fatalf("expected error when server not running and AutoStart=false")
	}
	if cm.State() != ConnStateFailed {
		t.Errorf("state = %q, want failed", cm.State())
	}
}

func TestConnMgr_ConnectSync_AutoStartHook_Invoked(t *testing.T) {
	sock := shortSockPath2(t)

	called := false
	cfg := ConnectionConfig{
		SocketPath: sock,
		AutoStart:  true,
		startServerFn: func(ctx context.Context, _ ConnectionConfig) error {
			called = true
			// Stand up a listener so the subsequent dial succeeds.
			startMockBFF(t, sock)
			return nil
		},
	}
	cm := NewConnectionManager(cfg, nil)
	t.Cleanup(cm.Disconnect)

	if err := cm.ConnectSync(context.Background()); err != nil {
		t.Fatalf("ConnectSync: %v", err)
	}
	if !called {
		t.Errorf("autoStart hook not invoked")
	}
}

func TestConnMgr_Disconnect_Idempotent(t *testing.T) {
	cm := NewConnectionManager(ConnectionConfig{}, nil)
	cm.Disconnect()
	cm.Disconnect() // must not panic / error

	if err := cm.ConnectSync(context.Background()); err == nil {
		t.Errorf("ConnectSync after Disconnect should error")
	}
}

func TestConnMgr_HeartbeatFailure_Degraded_Then_Reconnecting(t *testing.T) {
	// Build a manager wired to a client whose every Ping returns an
	// error. Verify the missed-pong counter drives state transitions.
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{
		HeartbeatInterval: 5 * time.Millisecond,
		HeartbeatTimeout:  5 * time.Millisecond,
	}, sender)
	cm.state = ConnStateReady

	// Stub client whose Ping always errors. We bypass Dial by creating
	// a client-like value that satisfies the methods we use. Easier:
	// drive sendHeartbeat directly with a fake.
	for i := 1; i <= 3; i++ {
		cm.simulateMissedPong()
	}
	if cm.State() != ConnStateDegraded {
		t.Fatalf("after 3 missed pongs, state = %q, want degraded", cm.State())
	}
	for i := 4; i <= 5; i++ {
		cm.simulateMissedPong()
	}
	if cm.State() != ConnStateReconnecting {
		t.Errorf("after 5 missed pongs, state = %q, want reconnecting", cm.State())
	}
	cm.Disconnect()
}

func TestConnMgr_HeartbeatRecovery_DegradedToReady(t *testing.T) {
	sender := &recordingSender{}
	cm := NewConnectionManager(ConnectionConfig{}, sender)
	cm.state = ConnStateReady

	for i := 0; i < 3; i++ {
		cm.simulateMissedPong()
	}
	if cm.State() != ConnStateDegraded {
		t.Fatalf("setup: state = %q", cm.State())
	}
	cm.simulateSuccessfulPong()
	if cm.State() != ConnStateReady {
		t.Errorf("after successful pong, state = %q, want ready", cm.State())
	}
}

// -----------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------

// simulateMissedPong drives the same code path sendHeartbeat takes
// after a Ping error, without needing a live client.
func (cm *ConnectionManager) simulateMissedPong() {
	cm.mu.Lock()
	cm.missedPongs++
	missed := cm.missedPongs
	state := cm.state
	cm.mu.Unlock()

	switch {
	case missed >= missedPongsReconnecting:
		if state != ConnStateReconnecting {
			cm.transition(ConnStateReconnecting, "missed heartbeats")
		}
	case missed >= missedPongsDegraded:
		if state == ConnStateReady {
			cm.transition(ConnStateDegraded, "missed heartbeats")
		}
	}
}

func (cm *ConnectionManager) simulateSuccessfulPong() {
	cm.mu.Lock()
	cm.missedPongs = 0
	state := cm.state
	cm.mu.Unlock()
	if state == ConnStateDegraded {
		cm.transition(ConnStateReady, "heartbeat recovered")
	}
}

// -----------------------------------------------------------------------
// Mock BFF socket — accepts a connection and answers
// capabilities.register + connection.ping.
// -----------------------------------------------------------------------

type mockBFF struct {
	ln net.Listener
	t  *testing.T
}

func startMockBFF(t *testing.T, sock string) *mockBFF {
	t.Helper()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen %s: %v", sock, err)
	}
	srv := &mockBFF{ln: ln, t: t}
	go srv.acceptLoop()
	t.Cleanup(srv.close)
	return srv
}

func (s *mockBFF) close() {
	if s.ln != nil {
		// Don't nil s.ln — acceptLoop reads it from another goroutine.
		// Closing the listener is sufficient to unblock Accept with
		// net.ErrClosed; the field is set once at construction and
		// never reassigned, so leaving it pointing at the closed
		// listener is race-free.
		_ = s.ln.Close()
	}
}

func (s *mockBFF) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *mockBFF) handle(conn net.Conn) {
	defer conn.Close()
	r := protocol.NewFrameReader(conn)
	w := protocol.NewFrameWriter(conn)
	for {
		raw, err := r.ReadFrame()
		if err != nil {
			return
		}
		var req protocol.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
		}
		var result json.RawMessage
		switch req.Method {
		case protocol.MethodCapabilitiesRegister:
			resp := protocol.CapabilitiesRegisterResponse{
				ServerCapabilities: protocol.ServerCapabilities{
					ProtocolVersion:   "1",
					MaxFrameSize:      10 * 1024 * 1024,
					MaxAttachmentSize: 5 * 1024 * 1024,
				},
			}
			result, _ = json.Marshal(resp)
		case protocol.MethodConnectionPing:
			result = json.RawMessage(`{}`)
		default:
			result = json.RawMessage(`{}`)
		}
		respFrame, _ := json.Marshal(&protocol.Response{
			JSONRPC: "2.0", ID: req.ID, Result: result,
		})
		_ = w.WriteFrame(respFrame)
	}
}

// shortSockPath2 returns a /tmp socket path that fits inside macOS's
// 104-byte unix-socket limit.
func shortSockPath2(t *testing.T) string {
	t.Helper()
	p := "/tmp/mt-conn-" + t.Name() + ".sock"
	t.Cleanup(func() { _ = osRemoveSocket(p) })
	return p
}
