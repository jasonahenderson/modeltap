package harnesshost

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// WU-102 Layer 3 integration tests for the
// internal/harnessshell + internal/harnesshost composition. Per
// WU-102 §"Cutover and integration tests", this layer's job is not
// to duplicate Layer 1 or Layer 2 coverage — it confirms the
// composition path is wired correctly end-to-end.
//
// The tests use a fakeRuntime and inject realistic runtime tea.Msg
// sequences from internal/harness so the projection + pause buffer +
// inner shell pipeline is exercised end-to-end.

// pumpAdapter feeds msg into the adapter, then drains every emitted
// tea.Cmd back through Update until no more messages flow. Useful
// for mid-stream tests where one Update invocation triggers a chain
// of follow-up events (e.g. SubmissionAcceptedEvent forwarding plus
// a synthetic RunStartedEvent).
func pumpAdapter(t *testing.T, a Adapter, msg tea.Msg, max int) Adapter {
	t.Helper()
	pending := []tea.Cmd{}
	updated, cmd := a.Update(msg)
	a = updated.(Adapter)
	if cmd != nil {
		pending = append(pending, cmd)
	}
	for steps := 0; len(pending) > 0; steps++ {
		if steps > max {
			t.Fatalf("pumpAdapter exceeded %d steps; likely infinite loop", max)
		}
		next := pending[0]
		pending = pending[1:]
		if next == nil {
			continue
		}
		out := next()
		if out == nil {
			continue
		}
		switch typed := out.(type) {
		case tea.BatchMsg:
			for _, c := range typed {
				pending = append(pending, c)
			}
			continue
		}
		updated, cmd := a.Update(out)
		a = updated.(Adapter)
		if cmd != nil {
			pending = append(pending, cmd)
		}
	}
	return a
}

func TestIntegrationSubmitStreamCompletePipeline(t *testing.T) {
	// End-to-end adapter-side pipeline: shell-emitted ActionMsg →
	// adapter dispatches to runtime → correlation table populated →
	// runtime stream messages project to RunDelta/RunCompleted via
	// the projection layer. Visual rendering of streamed deltas
	// requires the shell-side emission path (which creates the
	// optimistic assistant placeholder); that is covered by the
	// harnessshell event tests. This test asserts the adapter half.
	rt := &fakeRuntime{submitResult: SubmitAccepted{RunID: "run-1", Label: "test-model"}}
	a := New(harnessshell.New(), rt)
	updated, _ := a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = updated.(Adapter)

	a = pumpAdapter(t, a, harnessshell.ActionMsg{Action: harnessshell.SubmitTurnAction{
		Submission: harnessshell.Submission{
			ID:     "sub-1",
			Text:   "hello",
			Source: harnessshell.SubmissionSourceDirect,
		},
	}}, 50)

	if len(rt.submitCalls) != 1 || rt.submitCalls[0].SubmissionID != "sub-1" {
		t.Fatalf("expected 1 SubmitTurn call for sub-1; got %v", rt.submitCalls)
	}
	if got := a.submissionToRun["sub-1"]; got != "run-1" {
		t.Fatalf("submissionToRun[sub-1] = %q, want run-1", got)
	}
	if got := a.runToSubmission["run-1"]; got != "sub-1" {
		t.Fatalf("runToSubmission[run-1] = %q, want sub-1", got)
	}

	// Runtime messages project and forward through the adapter's
	// pipeline without panicking; the projection layer's mapping is
	// asserted by projection_test.go.
	a = pumpAdapter(t, a, harness.StreamTokenMsg{TurnID: "run-1", Delta: "wor"}, 10)
	a = pumpAdapter(t, a, harness.StreamTokenMsg{TurnID: "run-1", Delta: "ld"}, 10)
	a = pumpAdapter(t, a, harness.StreamCompleteMsg{TurnID: "run-1"}, 10)

	// Sanity: View renders without panicking. The footer hint is the
	// most stable always-present marker after PATCH-0027 (the prior
	// "background agents" marker no longer renders when AgentCount
	// is zero, which is the default).
	view := a.View()
	if !strings.Contains(view, "Tab focus") {
		t.Fatalf("View should render the composer chrome; got:\n%s", view)
	}
}

func TestIntegrationMidStreamPermissionPauseAndResume(t *testing.T) {
	// End-to-end mid-stream pause integration: permission arriving
	// during an active stream causes the adapter to buffer
	// RunDeltaEvent forwarding; PermissionResolvedEvent drains the
	// buffer in arrival order. Asserted at the adapter state level
	// (rendering deltas inside an assistant row requires a shell-
	// driven submit path; that integration is covered separately by
	// the harnessshell event tests).
	rt := &fakeRuntime{submitResult: SubmitAccepted{RunID: "run-1"}}
	a := New(harnessshell.New(), rt)
	updated, _ := a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = updated.(Adapter)

	// Permission arrives — pause begins.
	a = pumpAdapter(t, a, harness.PermissionPromptMsg{
		ToolCallID:  "perm-1",
		ToolName:    "Read",
		Description: "PROJECTED-PERMISSION",
		Input:       []byte(`{}`),
	}, 10)

	view := a.View()
	if !strings.Contains(view, "PROJECTED-PERMISSION") {
		t.Fatalf("permission summary should be visible during pause; got:\n%s", view)
	}

	// Deltas while paused buffer instead of forwarding.
	a = pumpAdapter(t, a, harness.StreamTokenMsg{TurnID: "run-1", Delta: "DURING-"}, 10)
	a = pumpAdapter(t, a, harness.StreamTokenMsg{TurnID: "run-1", Delta: "PAUSE"}, 10)
	if len(a.pauseBuffer) != 2 {
		t.Fatalf("expected 2 buffered deltas, got %d", len(a.pauseBuffer))
	}
	if a.pauseBuffer[0].Delta != "DURING-" || a.pauseBuffer[1].Delta != "PAUSE" {
		t.Fatalf("buffer should preserve arrival order; got %+v", a.pauseBuffer)
	}

	// Resolve the permission — buffered deltas drain.
	a = pumpAdapter(t, a, harnessshell.PermissionResolvedEvent{
		RequestID: "perm-1", Outcome: harnessshell.OutcomeApprovedOnce,
	}, 10)

	if len(a.pauseBuffer) != 0 {
		t.Fatalf("buffer should drain on resolution, got %d", len(a.pauseBuffer))
	}
	if _, stillPending := a.pendingPermissions["perm-1"]; stillPending {
		t.Fatalf("perm-1 should be cleared from pendingPermissions")
	}
}

func TestIntegrationNoSpikeImportsRemain(t *testing.T) {
	// Synthetic regression check for WU-102 acceptance criterion 3:
	// "no spike-shaped surface remains as a behavior authority" and
	// the WU-100 Stage E rule "nothing in the repo imports a path
	// containing harnessspike". A real grep is the canonical check;
	// this test serves as a documentation anchor.
	t.Skip("Repo-level invariant; verified by `go list ./...` + grep at " +
		"WU-100 Stage E commit time. Test exists to anchor WU-102 §" +
		"\"Tests that move to host adapter / top-level integration\"")
}
