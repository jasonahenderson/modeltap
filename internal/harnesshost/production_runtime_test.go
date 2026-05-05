package harnesshost

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harnesshost/testutil"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// WU-104a integration tests for ProductionRuntime.SubmitTurn against
// the testutil BFF stub. Verifies the live ConnectionManager →
// ProtocolClient → SubmitTurn → ack pipeline end-to-end.

func newProductionRuntimeForTest(t *testing.T, stub *testutil.BFFStub) *ProductionRuntime {
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
	stub, err := testutil.NewBFFStub()
	if err != nil {
		t.Fatalf("NewBFFStub: %v", err)
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
		t.Fatalf("BFF stub received %d submits, want 1", len(submits))
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
	stub, err := testutil.NewBFFStub()
	if err != nil {
		t.Fatalf("NewBFFStub: %v", err)
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
	if !strings.Contains(err.Error(), "no live BFF client") {
		t.Fatalf("error = %v, want 'no live BFF client'", err)
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
	stub, err := testutil.NewBFFStub()
	if err != nil {
		t.Fatalf("NewBFFStub: %v", err)
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
