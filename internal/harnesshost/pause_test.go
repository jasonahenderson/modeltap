package harnesshost

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// Stage D-3 tests for the mid-stream pause buffer per WU-099 §"Mid-
// stream Pause". The adapter buffers RunDeltaEvent forwarding while
// any permission is pending and replays buffered deltas in arrival
// order once the pending set drains.

// drainCmd executes a tea.Cmd and returns the produced messages,
// flattening tea.BatchMsg into its constituents.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	out := cmd()
	switch m := out.(type) {
	case tea.BatchMsg:
		var msgs []tea.Msg
		for _, c := range m {
			msgs = append(msgs, drainCmd(c)...)
		}
		return msgs
	case nil:
		return nil
	default:
		return []tea.Msg{m}
	}
}

// pushEvent feeds a HostEvent into the adapter via Update and returns
// the updated adapter plus any messages produced by the resulting cmd.
// Used to simulate runtime → shell event flow in tests.
func pushEvent(t *testing.T, a Adapter, evt harnessshell.HostEvent) (Adapter, []tea.Msg) {
	t.Helper()
	updated, cmd := a.Update(evt)
	return updated.(Adapter), drainCmd(cmd)
}

func TestPauseBufferRunDeltaForwardsImmediatelyWhenNoPermissionPending(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	// No permission is pending; RunDelta should NOT be buffered.
	a, _ = pushEvent(t, a, harnessshell.RunDeltaEvent{RunID: "r1", Delta: "x"})

	if len(a.pauseBuffer) != 0 {
		t.Fatalf("pauseBuffer should be empty when no permission pending, got %d", len(a.pauseBuffer))
	}
}

func TestPauseBufferBuffersDeltaWhilePermissionPending(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	// Permission requested → registers pending.
	a, _ = pushEvent(t, a, harnessshell.PermissionRequestedEvent{
		Request: harnessshell.PermissionRequest{ID: "p1", ToolLabel: "Read", Summary: "x"},
	})
	if _, ok := a.pendingPermissions["p1"]; !ok {
		t.Fatalf("p1 not registered as pending; got %+v", a.pendingPermissions)
	}

	// RunDelta now buffers instead of forwarding.
	a, _ = pushEvent(t, a, harnessshell.RunDeltaEvent{RunID: "r1", Delta: "first"})
	a, _ = pushEvent(t, a, harnessshell.RunDeltaEvent{RunID: "r1", Delta: "second"})

	if len(a.pauseBuffer) != 2 {
		t.Fatalf("pauseBuffer = %d, want 2", len(a.pauseBuffer))
	}
	if a.pauseBuffer[0].Delta != "first" || a.pauseBuffer[1].Delta != "second" {
		t.Fatalf("buffer contents wrong: %+v", a.pauseBuffer)
	}
}

func TestPauseBufferReplaysOnPermissionResolvedInArrivalOrder(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	a, _ = pushEvent(t, a, harnessshell.PermissionRequestedEvent{
		Request: harnessshell.PermissionRequest{ID: "p1", Summary: "x"},
	})
	a, _ = pushEvent(t, a, harnessshell.RunDeltaEvent{RunID: "r1", Delta: "alpha "})
	a, _ = pushEvent(t, a, harnessshell.RunDeltaEvent{RunID: "r1", Delta: "beta"})
	if len(a.pauseBuffer) != 2 {
		t.Fatalf("expected 2 buffered deltas, got %d", len(a.pauseBuffer))
	}

	// Resolve permission: pending drains, buffer replays.
	a, _ = pushEvent(t, a, harnessshell.PermissionResolvedEvent{
		RequestID: "p1", Outcome: harnessshell.OutcomeApprovedOnce,
	})

	if len(a.pendingPermissions) != 0 {
		t.Fatalf("pending should be empty after resolve, got %+v", a.pendingPermissions)
	}
	if len(a.pauseBuffer) != 0 {
		t.Fatalf("pauseBuffer should be empty after replay, got %d", len(a.pauseBuffer))
	}
}

func TestPauseBufferKeepsBufferingWhenAdditionalPermissionsRemain(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	a, _ = pushEvent(t, a, harnessshell.PermissionRequestedEvent{Request: harnessshell.PermissionRequest{ID: "p1", Summary: "a"}})
	a, _ = pushEvent(t, a, harnessshell.PermissionRequestedEvent{Request: harnessshell.PermissionRequest{ID: "p2", Summary: "b"}})
	a, _ = pushEvent(t, a, harnessshell.RunDeltaEvent{RunID: "r1", Delta: "x"})
	if len(a.pauseBuffer) != 1 {
		t.Fatalf("expected 1 buffered delta, got %d", len(a.pauseBuffer))
	}

	// Resolve only one of the two pending permissions: pause stays
	// active because p2 is still pending.
	a, _ = pushEvent(t, a, harnessshell.PermissionResolvedEvent{
		RequestID: "p1", Outcome: harnessshell.OutcomeApprovedOnce,
	})
	if len(a.pendingPermissions) != 1 {
		t.Fatalf("expected 1 pending remaining, got %+v", a.pendingPermissions)
	}
	if len(a.pauseBuffer) != 1 {
		t.Fatalf("buffer should not drain while pending remains; got %d", len(a.pauseBuffer))
	}

	// Resolve p2: now both pending are gone and buffer drains.
	a, _ = pushEvent(t, a, harnessshell.PermissionResolvedEvent{
		RequestID: "p2", Outcome: harnessshell.OutcomeApprovedOnce,
	})
	if len(a.pendingPermissions) != 0 {
		t.Fatalf("expected 0 pending after p2 resolve, got %+v", a.pendingPermissions)
	}
	if len(a.pauseBuffer) != 0 {
		t.Fatalf("buffer should drain after last resolve, got %d", len(a.pauseBuffer))
	}
}

func TestPauseBufferIgnoresDuplicateResolveForUnknownRequest(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	// Register one pending then resolve a different one — pending
	// should remain registered.
	a, _ = pushEvent(t, a, harnessshell.PermissionRequestedEvent{Request: harnessshell.PermissionRequest{ID: "p1", Summary: "x"}})
	a, _ = pushEvent(t, a, harnessshell.PermissionResolvedEvent{RequestID: "p99", Outcome: harnessshell.OutcomeDenied})

	if _, ok := a.pendingPermissions["p1"]; !ok {
		t.Fatalf("p1 should still be pending after unrelated resolve; got %+v", a.pendingPermissions)
	}
}

func TestPauseBufferNonRunDeltaEventsForwardEvenWhilePending(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	a, _ = pushEvent(t, a, harnessshell.PermissionRequestedEvent{Request: harnessshell.PermissionRequest{ID: "p1", Summary: "x"}})

	// HostStatusEvent is NOT a RunDelta; it forwards immediately
	// even while a permission is pending. The shell needs status
	// updates (e.g., "preview loading") to flow through during a
	// pause.
	updated, _ := a.Update(harnessshell.HostStatusEvent{Status: "test", Kind: harnessshell.StatusReady})
	a = updated.(Adapter)

	// pauseBuffer is unchanged.
	if len(a.pauseBuffer) != 0 {
		t.Fatalf("HostStatusEvent should not buffer; got buffer size %d", len(a.pauseBuffer))
	}
}
