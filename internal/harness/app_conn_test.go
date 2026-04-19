package harness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// fakeConn records App→ConnSurface interactions and returns canned
// replies. State / Client can be adjusted between calls to exercise
// the "no client" paths.
type fakeConn struct {
	mu sync.Mutex

	state     string
	reconnect int
	client    ConnProtocolClient
}

func (f *fakeConn) State() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *fakeConn) Reconnect() tea.Cmd {
	f.mu.Lock()
	f.reconnect++
	f.mu.Unlock()
	return func() tea.Msg { return reconnectStartedMsg{} }
}

func (f *fakeConn) Client() ConnProtocolClient {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.client
}

// reconnectStartedMsg is a test marker so we can assert Reconnect
// actually ran in the returned tea.Cmd.
type reconnectStartedMsg struct{}

// fakeClient satisfies ConnProtocolClient. Each method records its
// last-seen input and returns canned output.
type fakeClient struct {
	mu sync.Mutex

	transformCalls []*protocol.ContentTransform
	transformResp  protocol.ContentTransformResponse
	transformErr   error

	submitCalls []*protocol.TurnSubmit
	submitAck   TurnSubmitAck
	submitErr   error

	historyCalls []*protocol.HistoryList
	historyResp  protocol.HistoryListResponse
	historyErr   error

	modelListCalls int
	modelListResp  protocol.ModelListResponse
	modelListErr   error

	modelSwitchCalls []*protocol.ModelSwitch
	modelSwitchResp  protocol.ModelSwitchResponse
	modelSwitchErr   error

	sessionListCalls int
	sessionListResp  protocol.SessionListResponse
	sessionListErr   error

	sessionResumeCalls []string
	sessionResumeResp  protocol.SessionResumeResponse
	sessionResumeErr   error

	sessionClearCalls []string
	sessionClearResp  protocol.SessionClearResponse
	sessionClearErr   error

	sessionForkCalls []string
	sessionForkResp  protocol.SessionForkResponse
	sessionForkErr   error

	contextListCalls []string
	contextListResp  protocol.ContextListResponse
	contextListErr   error
}

func (f *fakeClient) ContentTransform(ctx context.Context, req *protocol.ContentTransform) (json.RawMessage, error) {
	f.mu.Lock()
	f.transformCalls = append(f.transformCalls, req)
	err := f.transformErr
	resp := f.transformResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return json.Marshal(resp)
}

func (f *fakeClient) SubmitTurn(ctx context.Context, submit *protocol.TurnSubmit) (*TurnSubmitAck, error) {
	f.mu.Lock()
	f.submitCalls = append(f.submitCalls, submit)
	err := f.submitErr
	ack := f.submitAck
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &ack, nil
}

func (f *fakeClient) HistoryList(ctx context.Context, req *protocol.HistoryList) (*protocol.HistoryListResponse, error) {
	f.mu.Lock()
	f.historyCalls = append(f.historyCalls, req)
	err := f.historyErr
	resp := f.historyResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *fakeClient) ModelList(ctx context.Context) (*protocol.ModelListResponse, error) {
	f.mu.Lock()
	f.modelListCalls++
	err := f.modelListErr
	resp := f.modelListResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *fakeClient) ModelSwitch(ctx context.Context, req *protocol.ModelSwitch) (*protocol.ModelSwitchResponse, error) {
	f.mu.Lock()
	f.modelSwitchCalls = append(f.modelSwitchCalls, req)
	err := f.modelSwitchErr
	resp := f.modelSwitchResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *fakeClient) SessionList(ctx context.Context) (*protocol.SessionListResponse, error) {
	f.mu.Lock()
	f.sessionListCalls++
	err := f.sessionListErr
	resp := f.sessionListResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *fakeClient) SessionResume(ctx context.Context, sessionID string, project protocol.ProjectContext) (*protocol.SessionResumeResponse, error) {
	f.mu.Lock()
	f.sessionResumeCalls = append(f.sessionResumeCalls, sessionID)
	err := f.sessionResumeErr
	resp := f.sessionResumeResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *fakeClient) SessionClear(ctx context.Context, sessionID string) (*protocol.SessionClearResponse, error) {
	f.mu.Lock()
	f.sessionClearCalls = append(f.sessionClearCalls, sessionID)
	err := f.sessionClearErr
	resp := f.sessionClearResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *fakeClient) SessionFork(ctx context.Context, sessionID string) (*protocol.SessionForkResponse, error) {
	f.mu.Lock()
	f.sessionForkCalls = append(f.sessionForkCalls, sessionID)
	err := f.sessionForkErr
	resp := f.sessionForkResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *fakeClient) ContextList(ctx context.Context, sessionID string) (*protocol.ContextListResponse, error) {
	f.mu.Lock()
	f.contextListCalls = append(f.contextListCalls, sessionID)
	err := f.contextListErr
	resp := f.contextListResp
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func drainCmdAny(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestApp_PasteSummarize_Success(t *testing.T) {
	fc := &fakeClient{
		transformResp: protocol.ContentTransformResponse{Content: "short summary"},
	}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})

	original := strings.Repeat("wall of text ", 300)
	_, cmd := app.Update(PasteSummarizeRequestMsg{Content: original})
	if cmd == nil {
		t.Fatal("expected summarize dispatch cmd")
	}
	msg := drainCmdAny(cmd)
	res, ok := msg.(PasteResolvedMsg)
	if !ok {
		t.Fatalf("expected PasteResolvedMsg, got %T (%+v)", msg, msg)
	}
	if res.Strategy != PasteStrategySummarize {
		t.Errorf("Strategy = %q, want summarize", res.Strategy)
	}
	if res.Content != "short summary" {
		t.Errorf("Content = %q, want short summary", res.Content)
	}
	if res.Original != original {
		t.Errorf("Original not propagated")
	}
	if len(fc.transformCalls) != 1 {
		t.Errorf("transform call count = %d, want 1", len(fc.transformCalls))
	} else if fc.transformCalls[0].Transform != "summarize" {
		t.Errorf("Transform = %q, want summarize", fc.transformCalls[0].Transform)
	}
}

func TestApp_PasteSummarize_FailureFallsBackToCancel(t *testing.T) {
	fc := &fakeClient{transformErr: errors.New("boom")}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})

	// Drive the failing cmd directly.
	_, cmd := app.Update(PasteSummarizeRequestMsg{Content: "original"})
	failMsg := drainCmdAny(cmd)
	// Expand the failure sentinel through Update to get the batched
	// (banner, cancel) follow-ups.
	model, batched := app.Update(failMsg)
	a, _ := model.(App)
	if batched == nil {
		t.Fatal("expected batched follow-up")
	}
	// tea.Batch produces a single Msg — a BatchMsg containing the cmds
	// or each fires independently depending on runtime. Easiest: run
	// the batched cmd and require we can find both outcomes in its
	// result set by converting manually.
	_ = a
	// Call the batched cmd; it returns a tea.BatchMsg which is a slice
	// of tea.Cmd. Flatten and invoke each to find the banner + resolve.
	out := batched()
	msgs := flattenBatch(out)
	var sawBanner, sawResolve bool
	for _, m := range msgs {
		if b, ok := m.(BannerMsg); ok {
			if !strings.Contains(b.Text, "boom") {
				t.Errorf("banner should include error reason: %q", b.Text)
			}
			sawBanner = true
		}
		if r, ok := m.(PasteResolvedMsg); ok {
			if r.Strategy != PasteStrategyCancel {
				t.Errorf("fallback Strategy = %q, want cancel", r.Strategy)
			}
			sawResolve = true
		}
	}
	if !sawBanner {
		t.Error("missing BannerMsg in fallback batch")
	}
	if !sawResolve {
		t.Error("missing PasteResolvedMsg in fallback batch")
	}
}

func TestApp_PasteSummarize_NoConn_FallsBack(t *testing.T) {
	app := NewApp(AppOptions{}) // no conn

	_, cmd := app.Update(PasteSummarizeRequestMsg{Content: "content"})
	failMsg := drainCmdAny(cmd)
	fm, ok := failMsg.(pasteSummarizeFailureMsg)
	if !ok {
		t.Fatalf("expected pasteSummarizeFailureMsg, got %T", failMsg)
	}
	if !strings.Contains(fm.reason, "no connection") {
		t.Errorf("reason should mention connection: %q", fm.reason)
	}
}

func TestApp_SubmitCommand_Status_Wired(t *testing.T) {
	conn := &fakeConn{state: ConnStateReady}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{Content: "/status", IsCommand: true, Command: "status"})
	b, ok := drainCmdAny(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, ConnStateReady) {
		t.Errorf("banner should include state %q, got %q", ConnStateReady, b.Text)
	}
}

func TestApp_SubmitCommand_Status_NoConn(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{Content: "/status", IsCommand: true, Command: "status"})
	b, ok := drainCmdAny(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, "no connection") {
		t.Errorf("banner should explain unwired state: %q", b.Text)
	}
}

func TestApp_SubmitCommand_Reconnect_InvokesConn(t *testing.T) {
	conn := &fakeConn{state: ConnStateFailed}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{Content: "/reconnect", IsCommand: true, Command: "reconnect"})
	if cmd == nil {
		t.Fatal("expected batched cmd")
	}
	out := cmd()
	msgs := flattenBatch(out)
	var sawBanner, sawReconnect bool
	for _, m := range msgs {
		if b, ok := m.(BannerMsg); ok {
			if strings.Contains(strings.ToLower(b.Text), "reconnect") {
				sawBanner = true
			}
		}
		if _, ok := m.(reconnectStartedMsg); ok {
			sawReconnect = true
		}
	}
	if !sawBanner {
		t.Error("expected reconnect banner")
	}
	if !sawReconnect {
		t.Error("expected Reconnect() to be invoked")
	}
	if conn.reconnect != 1 {
		t.Errorf("Reconnect call count = %d, want 1", conn.reconnect)
	}
}

func TestApp_SubmitCommand_Unknown(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "bogus"})
	b, ok := drainCmdAny(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, "bogus") {
		t.Errorf("banner should name the unknown command: %q", b.Text)
	}
}

func TestApp_SubmitTurn_Success(t *testing.T) {
	fc := &fakeClient{submitAck: TurnSubmitAck{TurnID: "turn-42", Status: "accepted"}}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.state.SessionID = "sess-1"

	_, cmd := app.Update(SubmitMsg{Content: "hi there"})
	msg := drainCmdAny(cmd)
	tm, ok := msg.(TurnSubmittedMsg)
	if !ok {
		t.Fatalf("expected TurnSubmittedMsg, got %T", msg)
	}
	if tm.Err != nil {
		t.Fatalf("unexpected error: %v", tm.Err)
	}
	if tm.TurnID != "turn-42" {
		t.Errorf("TurnID = %q, want turn-42", tm.TurnID)
	}
	if len(fc.submitCalls) != 1 {
		t.Fatalf("submit call count = %d, want 1", len(fc.submitCalls))
	}
	call := fc.submitCalls[0]
	if call.SessionID != "sess-1" {
		t.Errorf("SessionID = %q", call.SessionID)
	}
	if call.Content != "hi there" {
		t.Errorf("Content = %q", call.Content)
	}
	if call.Sequence < 1 {
		t.Errorf("Sequence = %d, want positive", call.Sequence)
	}
}

func TestApp_SubmitTurn_ConnErrorPropagated(t *testing.T) {
	fc := &fakeClient{submitErr: errors.New("rpc exploded")}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{Content: "hi"})
	tm, ok := drainCmdAny(cmd).(TurnSubmittedMsg)
	if !ok {
		t.Fatalf("expected TurnSubmittedMsg, got %T", cmd())
	}
	if tm.Err == nil || !strings.Contains(tm.Err.Error(), "rpc exploded") {
		t.Errorf("Err = %v", tm.Err)
	}

	// App.Update should then surface a failure banner on the follow-up.
	_, followCmd := app.Update(tm)
	b, ok := drainCmdAny(followCmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", followCmd())
	}
	if !strings.Contains(b.Text, "rpc exploded") {
		t.Errorf("banner should include error: %q", b.Text)
	}
}

func TestApp_SubmitTurn_NoConn_Errors(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{Content: "hi"})
	tm, ok := drainCmdAny(cmd).(TurnSubmittedMsg)
	if !ok {
		t.Fatalf("expected TurnSubmittedMsg, got %T", cmd())
	}
	if tm.Err == nil {
		t.Fatal("expected Err when no conn is wired")
	}
}

func TestApp_SetConn_Swap(t *testing.T) {
	app := NewApp(AppOptions{})
	if app.conn != nil {
		t.Fatal("fresh App should have no conn")
	}
	conn := &fakeConn{state: ConnStateReady}
	app.SetConn(conn)
	if app.conn != conn {
		t.Errorf("SetConn did not install the conn")
	}
}

// flattenBatch expands a tea.Msg produced by tea.Batch into its
// component messages. Bubbletea's BatchMsg is a []tea.Cmd; we run
// each one and collect results.
func flattenBatch(msg tea.Msg) []tea.Msg {
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			if c == nil {
				continue
			}
			out = append(out, flattenBatch(c())...)
		}
		return out
	}
	return []tea.Msg{msg}
}
