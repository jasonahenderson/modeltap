package harnesshost

import (
	"context"
	"encoding/json"
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

func TestProductionRuntimeStubMethodsReturnNotImplemented(t *testing.T) {
	cfg := ProductionRuntimeConfig{
		ConnConfig: harness.ConnectionConfig{SocketPath: "/nonexistent.sock"},
	}
	r, err := NewProductionRuntime(cfg)
	if err != nil {
		t.Fatalf("NewProductionRuntime: %v", err)
	}
	defer r.Close()

	if err := r.InterruptRun(context.Background(), "run-1"); err == nil {
		t.Fatalf("InterruptRun should return not-implemented at WU-104a")
	}
	if _, err := r.LoadPreview(context.Background(), PreviewRequest{}); err == nil {
		t.Fatalf("LoadPreview should return not-implemented at WU-104a")
	}
	if err := r.ResolvePermission(context.Background(), "perm-1", harnessshell.DecisionApproveOnce); err == nil {
		t.Fatalf("ResolvePermission should return not-implemented at WU-104a")
	}
	if err := r.DispatchCommand(context.Background(), HostCommand{Name: "model"}); err == nil {
		t.Fatalf("DispatchCommand should return not-implemented at WU-104a")
	}
	// SummarizePaste passes through at WU-104a.
	got, err := r.SummarizePaste(context.Background(), "raw text")
	if err != nil {
		t.Fatalf("SummarizePaste: %v", err)
	}
	if got != "raw text" {
		t.Fatalf("SummarizePaste = %q, want passthrough %q", got, "raw text")
	}
}
