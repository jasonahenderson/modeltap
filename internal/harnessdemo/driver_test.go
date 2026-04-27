package harnessdemo

import (
	"context"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/harnessshell"
	"github.com/jasonahenderson/modeltap/internal/harnesshost"
)

func ctx() context.Context { return context.Background() }

func fakeSubmitRequest(text string) harnesshost.SubmitRequest {
	return harnesshost.SubmitRequest{
		SubmissionID: "sub-test",
		Text:         text,
		Source:       harnessshell.SubmissionSourceDirect,
		RequestedAt:  time.Now(),
	}
}

func TestFakeRuntimeSubmitTurnRegistersStream(t *testing.T) {
	r := New().WithStreamDelay(0)
	accepted, err := r.SubmitTurn(ctx(), fakeSubmitRequest("hello"))
	if err != nil {
		t.Fatalf("SubmitTurn err = %v", err)
	}
	if accepted.RunID == "" {
		t.Fatalf("SubmitTurn returned empty RunID")
	}
	if accepted.Label == "" {
		t.Fatalf("SubmitTurn returned empty Label")
	}

	runs := r.TakeUnstartedRuns()
	if len(runs) != 1 || runs[0] != accepted.RunID {
		t.Fatalf("TakeUnstartedRuns = %v, want [%s]", runs, accepted.RunID)
	}

	chunk, paused, ok := r.PopStreamChunk(accepted.RunID)
	if !ok {
		t.Fatalf("expected at least one chunk for the new run")
	}
	if paused {
		t.Fatalf("stream should not be paused for a non-permission turn")
	}
	if chunk == "" {
		t.Fatalf("first chunk should be non-empty")
	}
}

func TestFakeRuntimePermissionDemoFlow(t *testing.T) {
	r := New().WithStreamDelay(0)

	accepted, _ := r.SubmitTurn(ctx(), fakeSubmitRequest("/perm please"))
	if !r.IsPermissionDemoRun(accepted.RunID) {
		t.Fatalf("/perm run should be flagged as permission demo")
	}
	_, paused, ok := r.PopStreamChunk(accepted.RunID)
	if !paused || ok {
		t.Fatalf("paused permission demo: paused=%v ok=%v want paused=true ok=false", paused, ok)
	}

	r.RegisterPermissionRequest("perm-1", accepted.RunID)
	if err := r.ResolvePermission(ctx(), "perm-1", harnessshell.DecisionApproveOnce); err != nil {
		t.Fatalf("ResolvePermission err = %v", err)
	}

	runs := r.TakeUnstartedRuns()
	if len(runs) == 0 {
		t.Fatalf("expected the resolved run to be re-queued for ticking")
	}

	chunk, paused, ok := r.PopStreamChunk(accepted.RunID)
	if !ok || paused {
		t.Fatalf("post-grant stream should be active; got paused=%v ok=%v", paused, ok)
	}
	if chunk == "" {
		t.Fatalf("granted reply should produce non-empty chunks")
	}
}

func TestFakeRuntimePermissionDeniedTerminatesStream(t *testing.T) {
	r := New().WithStreamDelay(0)

	accepted, _ := r.SubmitTurn(ctx(), fakeSubmitRequest("/perm denied"))
	r.RegisterPermissionRequest("perm-1", accepted.RunID)
	if err := r.ResolvePermission(ctx(), "perm-1", harnessshell.DecisionDeny); err != nil {
		t.Fatalf("ResolvePermission err = %v", err)
	}

	runs := r.TakeUnstartedRuns()
	if len(runs) == 0 {
		t.Fatalf("expected the denied run to be re-queued so the Driver can emit StreamComplete")
	}
	_, paused, ok := r.PopStreamChunk(accepted.RunID)
	if paused {
		t.Fatalf("denied stream should not stay paused")
	}
	if ok {
		t.Fatalf("denied stream should have no chunks; got chunk")
	}
}

func TestFakeRuntimeInterruptDropsStream(t *testing.T) {
	r := New().WithStreamDelay(0)
	accepted, _ := r.SubmitTurn(ctx(), fakeSubmitRequest("ping"))
	if err := r.InterruptRun(ctx(), accepted.RunID); err != nil {
		t.Fatalf("InterruptRun err = %v", err)
	}
	_, _, ok := r.PopStreamChunk(accepted.RunID)
	if ok {
		t.Fatalf("interrupted run should produce no chunks")
	}
}

func TestFakeRuntimeLoadPreviewReturnsSyntheticPayload(t *testing.T) {
	r := New()
	payload, err := r.LoadPreview(ctx(), harnesshost.PreviewRequest{
		TokenID: "tok-1", Source: "composer",
	})
	if err != nil {
		t.Fatalf("LoadPreview err = %v", err)
	}
	if payload.Title == "" || payload.Content == "" {
		t.Fatalf("preview payload missing fields: %+v", payload)
	}
}

func TestDriverDispatchesSubmitToFakeRuntime(t *testing.T) {
	shell := harnessshell.New()
	r := New().WithStreamDelay(0)
	d := NewDriver(shell, r)

	action := harnessshell.SubmitTurnAction{
		Submission: harnessshell.Submission{
			ID:          "sub-1",
			Text:        "ping",
			Source:      harnessshell.SubmissionSourceDirect,
			RequestedAt: time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
		},
	}
	updated, cmd := d.Update(harnessshell.ActionMsg{Action: action})
	d = updated.(Driver)

	if cmd == nil {
		t.Fatalf("expected dispatch cmd from Driver.Update")
	}
	_ = cmd()

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.activeStreams) != 1 {
		t.Fatalf("expected 1 active stream after dispatch; got %d", len(r.activeStreams))
	}
}
