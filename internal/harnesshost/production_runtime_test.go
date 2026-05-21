package harnesshost

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harnesshost/testutil"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// WU-104a integration tests for ProductionRuntime.SubmitTurn against
// the testutil Runtime stub. Verifies the live ConnectionManager →
// ProtocolClient → SubmitTurn → ack pipeline end-to-end.

func newProductionRuntimeForTest(t *testing.T, stub *testutil.RuntimeStub) *ProductionRuntime {
	t.Helper()
	cfg := ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{
			SocketPath: stub.SocketPath(),
			Registration: &protocol.CapabilitiesRegister{
				ProtocolVersion: "1",
				HarnessVersion:  "test",
				HarnessPlatform: "terminal",
				Project:         protocol.ProjectContext{Root: "/tmp"},
			},
		},
		ProjectRoot: "/tmp",
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  "test",
			HarnessPlatform: "terminal",
		},
		PermissionTimeout: 100 * time.Millisecond,
	}
	r, err := NewProductionRuntime(cfg)
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	// Connect the runtime synchronously so the live ProtocolClient
	// is available for SubmitTurn calls.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return r
}

func TestProductionRuntimeSubmitTurnReachesStub(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()

	r := newProductionRuntimeForTest(t, stub)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	accepted, err := r.SubmitTurn(ctx, SubmitRequest{
		SubmissionID: "sub-1",
		Text:         "hello",
		Source:       harnessshell.SubmissionSourceDirect,
		RequestedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("SubmitTurn: %v", err)
	}
	if accepted.RunID == "" {
		t.Fatalf("SubmitTurn returned empty RunID")
	}
	if !strings.HasPrefix(accepted.RunID, "stub-turn-") {
		t.Fatalf("RunID = %q, want stub-assigned 'stub-turn-N'", accepted.RunID)
	}

	submits := stub.Submits()
	if len(submits) != 1 {
		t.Fatalf("Runtime stub received %d submits, want 1", len(submits))
	}
	var got struct {
		TurnID    string `json:"turn_id"`
		SessionID string `json:"session_id"`
		Sequence  int    `json:"sequence"`
		Mode      string `json:"mode"`
		Content   string `json:"content"`
	}
	if err := json.Unmarshal(submits[0], &got); err != nil {
		t.Fatalf("unmarshal submit: %v", err)
	}
	if got.Content != "hello" {
		t.Fatalf("submitted content = %q, want %q", got.Content, "hello")
	}
	if got.TurnID == "" {
		t.Fatalf("harness-assigned TurnID empty")
	}
	if got.Sequence != 1 {
		t.Fatalf("first submit sequence = %d, want 1", got.Sequence)
	}
}

func TestProductionRuntimeSubmitTurnRecordsServerSession(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()

	r := newProductionRuntimeForTest(t, stub)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = r.SubmitTurn(ctx, SubmitRequest{Text: "first"})
	if err != nil {
		t.Fatalf("first SubmitTurn: %v", err)
	}
	if got := r.mode.SessionID(); got != "stub-session" {
		t.Fatalf("SessionID after submit = %q, want %q", got, "stub-session")
	}

	// Second submit reuses the recorded session ID.
	_, err = r.SubmitTurn(ctx, SubmitRequest{Text: "second"})
	if err != nil {
		t.Fatalf("second SubmitTurn: %v", err)
	}
	submits := stub.Submits()
	if len(submits) != 2 {
		t.Fatalf("expected 2 submits, got %d", len(submits))
	}
	var got struct {
		SessionID string `json:"session_id"`
		Sequence  int    `json:"sequence"`
	}
	_ = json.Unmarshal(submits[1], &got)
	if got.SessionID != "stub-session" {
		t.Fatalf("second submit session_id = %q, want stub-session", got.SessionID)
	}
	if got.Sequence != 2 {
		t.Fatalf("second submit sequence = %d, want 2", got.Sequence)
	}
}

func TestProductionRuntimeSessionResumeSeedsNextSubmitSequence(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()
	stub.SetSessionResume(protocol.SessionResumeResponse{
		SessionID:    "sess-resumed",
		Project:      protocol.ProjectContext{Root: "/tmp"},
		NextSequence: 4,
	})

	r := newProductionRuntimeForTest(t, stub)
	r.sender.onSend = func(tea.Msg) {}
	r.mode.SwitchSession("sess-old", 9)

	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "sessions", Args: "resume sess-resumed"}); err != nil {
		t.Fatalf("DispatchCommand(sessions resume): %v", err)
	}
	if got := r.mode.SessionID(); got != "sess-resumed" {
		t.Fatalf("SessionID after resume = %q, want sess-resumed", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := r.SubmitTurn(ctx, SubmitRequest{Text: "after resume"}); err != nil {
		t.Fatalf("SubmitTurn after resume: %v", err)
	}

	submits := stub.Submits()
	if len(submits) == 0 {
		t.Fatalf("expected submit after resume")
	}
	var got struct {
		SessionID string `json:"session_id"`
		Sequence  int    `json:"sequence"`
	}
	if err := json.Unmarshal(submits[len(submits)-1], &got); err != nil {
		t.Fatalf("unmarshal submit: %v", err)
	}
	if got.SessionID != "sess-resumed" {
		t.Fatalf("submit session_id = %q, want sess-resumed", got.SessionID)
	}
	if got.Sequence != 4 {
		t.Fatalf("submit sequence = %d, want 4", got.Sequence)
	}
}

func TestProductionRuntimeClearResetsNextSubmitSequence(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()

	r := newProductionRuntimeForTest(t, stub)
	r.sender.onSend = func(tea.Msg) {}
	r.mode.SwitchSession("sess-old", 7)

	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "clear"}); err != nil {
		t.Fatalf("DispatchCommand(clear): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := r.SubmitTurn(ctx, SubmitRequest{Text: "fresh session"}); err != nil {
		t.Fatalf("SubmitTurn after clear: %v", err)
	}

	submits := stub.Submits()
	if len(submits) == 0 {
		t.Fatalf("expected submit after clear")
	}
	var got struct {
		SessionID string `json:"session_id"`
		Sequence  int    `json:"sequence"`
	}
	if err := json.Unmarshal(submits[len(submits)-1], &got); err != nil {
		t.Fatalf("unmarshal submit: %v", err)
	}
	if got.SessionID != "stub-session" {
		t.Fatalf("submit session_id = %q, want stub-session", got.SessionID)
	}
	if got.Sequence != 1 {
		t.Fatalf("submit sequence = %d, want 1", got.Sequence)
	}
}

func TestProductionRuntimeSubmitTurnFailsWithoutClient(t *testing.T) {
	// Construct without starting — Client() returns nil.
	cfg := ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
	}
	r, err := NewProductionRuntime(cfg)
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	_, err = r.SubmitTurn(context.Background(), SubmitRequest{Text: "x"})
	if err == nil {
		t.Fatalf("SubmitTurn before connect should error")
	}
	if !strings.Contains(err.Error(), "no live runtime client") {
		t.Fatalf("error = %v, want 'no live runtime client'", err)
	}
}

func TestProductionRuntimeWU104bWU104cStubs(t *testing.T) {
	cfg := ProductionRuntimeConfig{
		ConnConfig:        harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
		PermissionTimeout: 10 * time.Millisecond,
	}
	r, err := NewProductionRuntime(cfg)
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	// DispatchCommand returns nil for known commands (status events
	// surface result/errors via the sender). Unknown commands also
	// return nil but emit StatusError.
	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "build"}); err != nil {
		t.Fatalf("DispatchCommand mode change should not error, got %v", err)
	}
	if r.mode.CurrentMode() != protocol.ModeBuild {
		t.Fatalf("mode = %v, want ModeBuild", r.mode.CurrentMode())
	}
	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "definitely-unknown"}); err != nil {
		t.Fatalf("DispatchCommand unknown command should not error (status event surfaces)")
	}

	// SummarizePaste passes through.
	got, err := r.SummarizePaste(context.Background(), "raw text")
	if err != nil || got != "raw text" {
		t.Fatalf("SummarizePaste = (%q,%v), want (raw text, nil)", got, err)
	}

	// InterruptRun without a live client synthesizes RunStoppedEvent
	// rather than returning an error. The error return is nil.
	if err := r.InterruptRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("InterruptRun: should return nil even without client, got %v", err)
	}

	// ResolvePermission with no pending request is a no-op.
	if err := r.ResolvePermission(context.Background(), "perm-unknown", harnessshell.DecisionApproveOnce); err != nil {
		t.Fatalf("ResolvePermission unknown ID should be no-op, got %v", err)
	}

	// LoadPreview without path returns an unresolved error.
	if _, err := r.LoadPreview(context.Background(), PreviewRequest{}); err == nil {
		t.Fatalf("LoadPreview without path should error")
	}
}

func TestProductionRuntimeLoadPreviewReadsFile(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()

	// Write a fixture file in a temp project root.
	dir := t.TempDir()
	fixture := dir + "/hello.txt"
	if err := os.WriteFile(fixture, []byte("PREVIEW-CONTENT-MARKER"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := ProductionRuntimeConfig{
		ConnConfig:  harness.ConnectionConfig{SocketPath: stub.SocketPath()},
		ProjectRoot: dir,
	}
	r, err := NewProductionRuntime(cfg)
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	payload, err := r.LoadPreview(ctx, PreviewRequest{
		TokenID: "tok-1",
		Path:    "hello.txt",
		Source:  "composer",
	})
	if err != nil {
		t.Fatalf("LoadPreview: %v", err)
	}
	if payload.Title == "" {
		t.Fatalf("preview title empty")
	}
	if !strings.Contains(payload.Content, "PREVIEW-CONTENT-MARKER") {
		t.Fatalf("preview content missing marker; got %q", payload.Content)
	}
}

func TestProductionRuntimeResolvePermissionUnblocksCallback(t *testing.T) {
	cfg := ProductionRuntimeConfig{
		ConnConfig:        harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
		PermissionTimeout: 100 * time.Millisecond,
	}
	r, err := NewProductionRuntime(cfg)
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	// Register a fake promise channel directly to simulate the
	// permissionPromptCallback being mid-flight.
	requestID := "perm-test"
	promise := make(chan harnessshell.PermissionDecision, 1)
	r.permPromises.Store(requestID, promise)
	defer r.permPromises.Delete(requestID)

	if err := r.ResolvePermission(context.Background(), requestID, harnessshell.DecisionApproveOnce); err != nil {
		t.Fatalf("ResolvePermission: %v", err)
	}
	select {
	case got := <-promise:
		if got != harnessshell.DecisionApproveOnce {
			t.Fatalf("decision = %v, want DecisionApproveOnce", got)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("ResolvePermission did not unblock the channel")
	}
}

// PATCH-0029: bootstrapSession must not overwrite a session id that
// a racing turn.submit already wrote. The race shape: ConnStateReady
// fires; bootstrapSession is goroutine'd; turn.submit runs first
// because the user typed fast, auto-creates session "stub-session"
// on the Runtime, and stores it via SetSessionID. session.create then
// returns later with a different id; bootstrapSession must observe
// the existing id and skip the Set.
func TestProductionRuntimeBootstrapSessionDoesNotOverwrite(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()

	r := newProductionRuntimeForTest(t, stub)

	// Simulate the race: a turn.submit completed before bootstrapSession.
	r.mode.SetSessionID("turn-assigned-session")

	// Run bootstrapSession directly (not via observeRuntimeMessage) to
	// keep the test deterministic. The stub answers session.create with
	// "stub-session"; we want to verify that response is discarded.
	r.bootstrapSession(context.Background())

	if got := r.mode.SessionID(); got != "turn-assigned-session" {
		t.Errorf("bootstrapSession overwrote turn-assigned session id: got %q, want %q",
			got, "turn-assigned-session")
	}
}

// PATCH-0028 + PATCH-0029: when no turn raced ahead, bootstrapSession
// adopts the session id returned by session.create.
func TestProductionRuntimeBootstrapSessionAdoptsWhenEmpty(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()

	r := newProductionRuntimeForTest(t, stub)

	// Pre-condition: no session id stored.
	if got := r.mode.SessionID(); got != "" {
		t.Fatalf("expected empty session id pre-bootstrap, got %q", got)
	}

	r.bootstrapSession(context.Background())

	if got := r.mode.SessionID(); got != "stub-session" {
		t.Errorf("bootstrapSession did not adopt session.create id: got %q, want %q",
			got, "stub-session")
	}
}

// PATCH-0038: bootstrapSession falls through to session.create when
// session.list fails (e.g. unsupported by the Runtime stub) or returns
// zero sessions. The harness ends up with a session id and emits a
// welcome HostInfoEvent.
func TestProductionRuntimeBootstrapFallsBackToCreateWhenListUnavailable(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()

	r := newProductionRuntimeForTest(t, stub)

	var msgs []any
	r.sender.onSend = func(msg tea.Msg) { msgs = append(msgs, msg) }

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r.bootstrapSession(ctx)

	if got := r.mode.SessionID(); got != "stub-session" {
		t.Fatalf("SessionID = %q, want stub-session (fallback create)", got)
	}
	// Welcome message names the new session.
	var welcomed bool
	for _, m := range msgs {
		if info, ok := m.(harnessshell.HostInfoEvent); ok && strings.Contains(info.Text, "New session stub-session") {
			welcomed = true
			break
		}
	}
	if !welcomed {
		t.Errorf("expected welcome HostInfoEvent for new session, msgs = %+v", msgs)
	}
}

// PATCH-0038: /clear refuses while a run is streaming. Mid-stream
// clear would require cancelling the active run first; surface a
// clear error instead of trying to redefine "new conversation"
// semantics while content is still arriving.
func TestProductionRuntimeClearCommandRejectsActiveRun(t *testing.T) {
	r, err := NewProductionRuntime(ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
	})
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	var msgs []any
	r.sender.onSend = func(msg tea.Msg) { msgs = append(msgs, msg) }

	r.mode.SetActiveRunID("run-streaming")
	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "clear"}); err != nil {
		t.Fatalf("DispatchCommand(clear) returned non-nil: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 status event, got %d (%+v)", len(msgs), msgs)
	}
	se, ok := msgs[0].(harnessshell.HostStatusEvent)
	if !ok {
		t.Fatalf("msgs[0] = %T, want HostStatusEvent", msgs[0])
	}
	if se.Kind != harnessshell.StatusError {
		t.Errorf("kind = %v, want StatusError", se.Kind)
	}
	if !strings.Contains(se.Status, "cannot start new conversation") {
		t.Errorf("status missing reject text: %q", se.Status)
	}
}

// PATCH-0038: /sessions current prints the active session id when
// one exists; prints "No active session" when not bootstrapped.
func TestProductionRuntimeSessionsCurrent(t *testing.T) {
	r, err := NewProductionRuntime(ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
	})
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	var msgs []any
	r.sender.onSend = func(msg tea.Msg) { msgs = append(msgs, msg) }

	// No active session yet — surfaces a status, not an info row.
	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "sessions", Args: "current"}); err != nil {
		t.Fatalf("DispatchCommand(sessions current): %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(msgs))
	}
	if se, ok := msgs[0].(harnessshell.HostStatusEvent); !ok || !strings.Contains(se.Status, "No active session") {
		t.Errorf("msgs[0] = %#v, want HostStatusEvent containing 'No active session'", msgs[0])
	}

	msgs = nil
	r.mode.SetSessionID("sess-abc-123")
	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "sessions", Args: "current"}); err != nil {
		t.Fatalf("DispatchCommand(sessions current): %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(msgs))
	}
	info, ok := msgs[0].(harnessshell.HostInfoEvent)
	if !ok {
		t.Fatalf("msgs[0] = %T, want HostInfoEvent", msgs[0])
	}
	if !strings.Contains(info.Text, "sess-abc-123") {
		t.Errorf("info text missing session id: %q", info.Text)
	}
}

// PATCH-0041: /sessions show <id> calls session.details and appends
// recent runs for that session.
func TestProductionRuntimeSessionsShowDetails(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()
	stub.SetSessionDetail(protocol.SessionDetail{
		ID:            "sess-target",
		Summary:       "Fix the session browser",
		CreatedAt:     "2026-05-21T00:00:00Z",
		LastActive:    "2026-05-21T00:10:00Z",
		Model:         "claude-sonnet",
		ModelOverride: "claude-opus",
		ContextPct:    42.5,
		TotalCost:     1.2345,
		Turns: []protocol.TurnSummary{
			{Sequence: 1, Summary: "Initial request", Model: "claude-sonnet", Cost: 0.0101},
			{Sequence: 2, Summary: "Compacted old context", Compacted: true, Model: "claude-sonnet"},
		},
		FilesTouched:  []string{"internal/harnesshost/production_runtime.go"},
		FilesModified: []string{"internal/harnesshost/production_runtime_test.go"},
		ServerEvents: []protocol.ServerSessionEvent{
			{Type: "compact", At: "2026-05-21T00:05:00Z", FreedTokens: 1200, Detail: "manual compact"},
		},
	})
	stub.SetRuns([]protocol.RunSummary{
		{
			RunID:           "run-1",
			Status:          protocol.RunStatusRunning,
			Stage:           protocol.RunStageModelCall,
			AttachmentState: protocol.RunAttachmentDetached,
			InputRequired:   true,
			Stuck:           true,
			Title:           "background implementation agent",
		},
	})

	r := newProductionRuntimeForTest(t, stub)
	var msgs []any
	r.sender.onSend = func(msg tea.Msg) { msgs = append(msgs, msg) }

	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "sessions", Args: "show sess-target"}); err != nil {
		t.Fatalf("DispatchCommand(sessions show): %v", err)
	}

	detailReq := lastCallParams[protocol.SessionDetails](t, stub, protocol.MethodSessionDetails)
	if detailReq.SessionID != "sess-target" {
		t.Fatalf("session.details session_id = %q, want sess-target", detailReq.SessionID)
	}
	runReq := lastCallParams[protocol.RunList](t, stub, protocol.MethodRunList)
	if runReq.SessionID != "sess-target" {
		t.Fatalf("run.list session_id = %q, want sess-target", runReq.SessionID)
	}
	if runReq.Limit != 10 {
		t.Fatalf("run.list limit = %d, want 10", runReq.Limit)
	}
	info := lastHostInfo(t, msgs)
	for _, want := range []string{
		"Session sess-target: Fix the session browser",
		"Created: 2026-05-21T00:00:00Z",
		"Last active: 2026-05-21T00:10:00Z",
		"Model: claude-sonnet (override: claude-opus)",
		"Context: 42.5%",
		"Cost: $1.2345",
		"Turns:",
		"#1 claude-sonnet $0.0101 — Initial request",
		"#2 compacted claude-sonnet — Compacted old context",
		"Files touched:",
		"internal/harnesshost/production_runtime.go",
		"Files modified:",
		"internal/harnesshost/production_runtime_test.go",
		"Server events:",
		"compact 2026-05-21T00:05:00Z freed=1200 — manual compact",
		"Runs:",
		"run-1 running/model_call detached input-required stuck — background implementation agent",
		"Drill down with /run <id> or /attach <id>.",
	} {
		if !strings.Contains(info.Text, want) {
			t.Errorf("session details output missing %q in:\n%s", want, info.Text)
		}
	}
}

// PATCH-0041: details is an alias for show, and /session uses the
// same singular command path.
func TestProductionRuntimeSessionDetailsAlias(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()
	r := newProductionRuntimeForTest(t, stub)
	r.sender.onSend = func(tea.Msg) {}

	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "session", Args: "details sess-alias"}); err != nil {
		t.Fatalf("DispatchCommand(session details): %v", err)
	}
	req := lastCallParams[protocol.SessionDetails](t, stub, protocol.MethodSessionDetails)
	if req.SessionID != "sess-alias" {
		t.Fatalf("session.details session_id = %q, want sess-alias", req.SessionID)
	}
}

// PATCH-0041: omitted ID defaults to the active session.
func TestProductionRuntimeSessionsShowUsesActiveSession(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()
	r := newProductionRuntimeForTest(t, stub)
	r.sender.onSend = func(tea.Msg) {}
	r.mode.SetSessionID("sess-active")

	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "sessions", Args: "show"}); err != nil {
		t.Fatalf("DispatchCommand(sessions show): %v", err)
	}
	req := lastCallParams[protocol.SessionDetails](t, stub, protocol.MethodSessionDetails)
	if req.SessionID != "sess-active" {
		t.Fatalf("session.details session_id = %q, want sess-active", req.SessionID)
	}
}

// PATCH-0041: omitted ID with no active session surfaces a status
// error before attempting an RPC.
func TestProductionRuntimeSessionsShowRequiresIDOrActiveSession(t *testing.T) {
	r, err := NewProductionRuntime(ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
	})
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()
	var msgs []any
	r.sender.onSend = func(msg tea.Msg) { msgs = append(msgs, msg) }

	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "sessions", Args: "show"}); err != nil {
		t.Fatalf("DispatchCommand(sessions show): %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("msgs = %d, want 1", len(msgs))
	}
	status, ok := msgs[0].(harnessshell.HostStatusEvent)
	if !ok {
		t.Fatalf("msg = %T, want HostStatusEvent", msgs[0])
	}
	if status.Kind != harnessshell.StatusError {
		t.Fatalf("status kind = %v, want StatusError", status.Kind)
	}
	if !strings.Contains(status.Status, "requires <id> or an active session") {
		t.Fatalf("status = %q, want missing id guidance", status.Status)
	}
}

// PATCH-0041: session details still render if run.list fails after
// session.details succeeds.
func TestProductionRuntimeSessionsShowToleratesRunListFailure(t *testing.T) {
	stub, err := testutil.NewRuntimeStub()
	if err != nil {
		t.Fatalf("NewRuntimeStub: %v", err)
	}
	defer stub.Close()
	stub.SetSessionDetail(protocol.SessionDetail{ID: "sess-no-runs", Summary: "visible"})
	stub.SetRunListError("run list unavailable")
	r := newProductionRuntimeForTest(t, stub)
	var msgs []any
	r.sender.onSend = func(msg tea.Msg) { msgs = append(msgs, msg) }

	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "sessions", Args: "show sess-no-runs"}); err != nil {
		t.Fatalf("DispatchCommand(sessions show): %v", err)
	}
	info := lastHostInfo(t, msgs)
	if !strings.Contains(info.Text, "Session sess-no-runs: visible") {
		t.Fatalf("session detail missing after run.list failure:\n%s", info.Text)
	}
	if !strings.Contains(info.Text, "Runs:\n  unavailable:") || !strings.Contains(info.Text, "run list unavailable") {
		t.Fatalf("run-list failure note missing:\n%s", info.Text)
	}
}

// PATCH-0037: /help emits a HostInfoEvent that names each top-level
// host command. The text is rendered into the transcript via the
// PATCH-0018 HostInfo path; users who type /help should see the full
// surface without leaving the shell.
func TestProductionRuntimeHelpCommandListsCommands(t *testing.T) {
	r, err := NewProductionRuntime(ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
	})
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	var msgs []any
	r.sender.onSend = func(msg tea.Msg) { msgs = append(msgs, msg) }

	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "help"}); err != nil {
		t.Fatalf("DispatchCommand(help): %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 HostInfoEvent, got %d (%+v)", len(msgs), msgs)
	}
	info, ok := msgs[0].(harnessshell.HostInfoEvent)
	if !ok {
		t.Fatalf("msg[0] = %T, want HostInfoEvent", msgs[0])
	}
	wantNames := []string{
		"/plan", "/build", "/auto",
		"/model", "/models",
		"/sessions", "show [id]",
		"/context", "/compact",
		"/history",
		"/mcp",
		"/run", "/runs", "/jobs",
		"/attach", "/detach", "/cancel", "/retry", "/continue", "/fork",
		"/clear", "/select", "/help", "/quit", "/exit",
	}
	for _, name := range wantNames {
		if !strings.Contains(info.Text, name) {
			t.Errorf("help text missing %q in:\n%s", name, info.Text)
		}
	}
}

// PATCH-0033: statusError must strip the JSON-RPC wire framing from
// harness.RPCError so users see "model.list failed: <message>" rather
// than "model.list failed: rpc error -32602: <message>". Plain
// (non-RPC) errors fall through unchanged.
func TestProductionRuntimeStatusError_UnwrapsRPCError(t *testing.T) {
	r, err := NewProductionRuntime(ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
	})
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	var msgs []any
	r.sender.onSend = func(msg tea.Msg) { msgs = append(msgs, msg) }

	rpcErr := &harness.RPCError{Code: -32602, Message: "cannot attach terminal run"}
	if err := r.statusError("run.attach", rpcErr); err != nil {
		t.Fatalf("statusError returned non-nil: %v", err)
	}

	plainErr := errorString("disk full")
	if err := r.statusError("session.list", plainErr); err != nil {
		t.Fatalf("statusError returned non-nil: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("msgs = %d, want 2", len(msgs))
	}
	got1, ok := msgs[0].(harnessshell.HostStatusEvent)
	if !ok {
		t.Fatalf("msgs[0] = %T, want HostStatusEvent", msgs[0])
	}
	if strings.Contains(got1.Status, "rpc error") {
		t.Errorf("RPCError framing leaked: %q", got1.Status)
	}
	if !strings.Contains(got1.Status, "cannot attach terminal run") {
		t.Errorf("inner message lost: %q", got1.Status)
	}
	got2, ok := msgs[1].(harnessshell.HostStatusEvent)
	if !ok {
		t.Fatalf("msgs[1] = %T, want HostStatusEvent", msgs[1])
	}
	if !strings.Contains(got2.Status, "disk full") {
		t.Errorf("plain error mangled: %q", got2.Status)
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }

func lastHostInfo(t *testing.T, msgs []any) harnessshell.HostInfoEvent {
	t.Helper()
	for i := len(msgs) - 1; i >= 0; i-- {
		if info, ok := msgs[i].(harnessshell.HostInfoEvent); ok {
			return info
		}
	}
	t.Fatalf("no HostInfoEvent in %+v", msgs)
	return harnessshell.HostInfoEvent{}
}

func lastCallParams[T any](t *testing.T, stub *testutil.RuntimeStub, method string) T {
	t.Helper()
	calls := stub.Calls()
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Method != method {
			continue
		}
		var out T
		if err := json.Unmarshal(calls[i].Params, &out); err != nil {
			t.Fatalf("unmarshal %s params: %v", method, err)
		}
		return out
	}
	t.Fatalf("method %s not called; calls = %+v", method, calls)
	var zero T
	return zero
}

func TestProductionRuntimeProjectRunReplay(t *testing.T) {
	r, err := NewProductionRuntime(ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
	})
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	var msgs []any
	r.sender.onSend = func(msg tea.Msg) {
		msgs = append(msgs, msg)
	}
	r.projectRunReplay([]protocol.RunEventPayload{
		{
			RunID:   "run-1",
			Type:    protocol.EventRunProgress,
			Payload: json.RawMessage(`{"type":"token_delta","text":"hello"}`),
		},
		{RunID: "run-1", Type: protocol.EventRunCompleted},
	})

	if len(msgs) != 2 {
		t.Fatalf("msgs = %d, want 2", len(msgs))
	}
	delta, ok := msgs[0].(harnessshell.RunDeltaEvent)
	if !ok || delta.RunID != "run-1" || delta.Delta != "hello" {
		t.Fatalf("delta msg = %#v", msgs[0])
	}
	if _, ok := msgs[1].(harnessshell.RunCompletedEvent); !ok {
		t.Fatalf("complete msg = %T, want RunCompletedEvent", msgs[1])
	}
}
