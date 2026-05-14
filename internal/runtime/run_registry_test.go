package runtime

import (
	"fmt"
	"testing"
)

func TestTurnTrackerCapsRegisteredTurns(t *testing.T) {
	tt := newTurnTracker()
	for i := 0; i < maxTrackedTurns+1; i++ {
		tt.register(fmt.Sprintf("turn-%d", i), func() {})
	}
	if got := len(tt.byTurnID); got != maxTrackedTurns {
		t.Fatalf("tracked turns = %d, want %d", got, maxTrackedTurns)
	}
}

func TestRunRegistryCapsRegisteredRunsAndTurnIndex(t *testing.T) {
	rr := newRunRegistry()
	for i := 0; i < maxTrackedRuns+1; i++ {
		id := fmt.Sprintf("run-%d", i)
		rr.register(id, fmt.Sprintf("turn-%d", i), "sess", "conn", func() {})
	}
	if got := len(rr.byRunID); got != maxTrackedRuns {
		t.Fatalf("tracked runs = %d, want %d", got, maxTrackedRuns)
	}
	if got := len(rr.byTurnID); got != maxTrackedRuns {
		t.Fatalf("tracked turn index = %d, want %d", got, maxTrackedRuns)
	}
}

func TestRunRegistryCancelRemovesTurnIndex(t *testing.T) {
	rr := newRunRegistry()
	cancelled := false
	rr.register("run-1", "turn-1", "sess", "conn", func() { cancelled = true })

	if !rr.cancel("run-1") {
		t.Fatalf("cancel returned false")
	}
	if !cancelled {
		t.Fatalf("cancel func was not called")
	}
	if got := rr.runIDForTurn("turn-1"); got != "" {
		t.Fatalf("runIDForTurn after cancel = %q, want empty", got)
	}
}

func TestRunRegistryRegisterReplacesOldTurnIndex(t *testing.T) {
	rr := newRunRegistry()
	rr.register("run-1", "turn-1", "sess", "conn", func() {})
	rr.register("run-1", "turn-2", "sess", "conn", func() {})

	if got := rr.runIDForTurn("turn-1"); got != "" {
		t.Fatalf("old turn index = %q, want empty", got)
	}
	if got := rr.runIDForTurn("turn-2"); got != "run-1" {
		t.Fatalf("new turn index = %q, want run-1", got)
	}
}
