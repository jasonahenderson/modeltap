package harnesshost

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// Stage D-2 tests for runtime-message → HostEvent projection. Mid-
// stream pause buffering of RunDeltaEvent lands in Stage D-3 with its
// own test surface.

func TestProjectStreamTokenMsgIsRunDelta(t *testing.T) {
	got := projectRuntimeMessage(harness.StreamTokenMsg{
		TurnID: "turn-1", Delta: "hello",
	})
	delta, ok := got.(harnessshell.RunDeltaEvent)
	if !ok {
		t.Fatalf("got %T, want RunDeltaEvent", got)
	}
	if delta.RunID != "turn-1" || delta.Delta != "hello" {
		t.Fatalf("delta = %+v", delta)
	}
}

func TestProjectStreamCompleteMsgIsRunCompleted(t *testing.T) {
	got := projectRuntimeMessage(harness.StreamCompleteMsg{TurnID: "turn-1"})
	completed, ok := got.(harnessshell.RunCompletedEvent)
	if !ok {
		t.Fatalf("got %T, want RunCompletedEvent", got)
	}
	if completed.RunID != "turn-1" {
		t.Fatalf("RunID = %q", completed.RunID)
	}
}

func TestProjectTurnSubmittedMsgSuccess(t *testing.T) {
	got := projectRuntimeMessage(harness.TurnSubmittedMsg{TurnID: "turn-1"})
	accepted, ok := got.(harnessshell.SubmissionAcceptedEvent)
	if !ok {
		t.Fatalf("got %T, want SubmissionAcceptedEvent", got)
	}
	if accepted.RunID != "turn-1" {
		t.Fatalf("accepted = %+v", accepted)
	}
}

func TestProjectTurnSubmittedMsgErrorIsSubmissionFailed(t *testing.T) {
	got := projectRuntimeMessage(harness.TurnSubmittedMsg{
		TurnID: "turn-1",
		Err:    errors.New("dispatch failed"),
	})
	failed, ok := got.(harnessshell.SubmissionFailedEvent)
	if !ok {
		t.Fatalf("got %T, want SubmissionFailedEvent", got)
	}
	if !strings.Contains(failed.Message, "dispatch failed") {
		t.Fatalf("message = %q, missing dispatch failed", failed.Message)
	}
}

func TestProjectStatusUpdateMsgIsHostStatus(t *testing.T) {
	got := projectRuntimeMessage(harness.StatusUpdateMsg{
		TurnID: "turn-1", Message: "routing to claude-opus-4-7",
	})
	status, ok := got.(harnessshell.HostStatusEvent)
	if !ok {
		t.Fatalf("got %T, want HostStatusEvent", got)
	}
	if !strings.Contains(status.Status, "claude-opus") {
		t.Fatalf("status text = %q", status.Status)
	}
	if status.Kind != harnessshell.StatusStreaming {
		t.Fatalf("kind = %v, want StatusStreaming", status.Kind)
	}
}

func TestProjectBranchMessagesFlattenIntoRunEvents(t *testing.T) {
	started := projectRuntimeMessage(harness.BranchStartedMsg{
		TurnID: "turn-1", BranchID: "b1", Model: "claude-opus-4-7",
	}).(harnessshell.RunStartedEvent)
	if started.RunID != "turn-1:b1" {
		t.Fatalf("branch RunID = %q, want turn-1:b1", started.RunID)
	}
	if started.Label != "claude-opus-4-7" {
		t.Fatalf("label = %q", started.Label)
	}

	completed := projectRuntimeMessage(harness.BranchCompleteMsg{
		TurnID: "turn-1", BranchID: "b1",
	}).(harnessshell.RunCompletedEvent)
	if completed.RunID != "turn-1:b1" {
		t.Fatalf("complete RunID = %q", completed.RunID)
	}

	failed := projectRuntimeMessage(harness.BranchErrorMsg{
		TurnID: "turn-1", BranchID: "b1", Error: "model errored",
	}).(harnessshell.RunFailedEvent)
	if failed.RunID != "turn-1:b1" || failed.Message != "model errored" {
		t.Fatalf("failed = %+v", failed)
	}
}

func TestProjectToolActivityStartIsHostStatus(t *testing.T) {
	got := projectRuntimeMessage(harness.ToolActivityMsg{
		Phase: harness.ToolActivityStart, ToolName: "Read", Summary: "foo.txt",
	}).(harnessshell.HostStatusEvent)
	if !strings.Contains(got.Status, "Read") || !strings.Contains(got.Status, "foo.txt") {
		t.Fatalf("status = %q", got.Status)
	}
	if !strings.HasPrefix(got.Status, "⚙ ") {
		t.Fatalf("expected gear prefix; got %q", got.Status)
	}
}

func TestProjectToolActivityEndUsesOutcomeGlyph(t *testing.T) {
	cases := []struct {
		status string
		prefix string
	}{
		{"success", "✓ "},
		{"error", "✗ "},
		{"rejected", "⊘ "},
		{"unknown", "• "},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			got := projectRuntimeMessage(harness.ToolActivityMsg{
				Phase: harness.ToolActivityEnd, ToolName: "Read", Status: tc.status,
			}).(harnessshell.HostStatusEvent)
			if !strings.HasPrefix(got.Status, tc.prefix) {
				t.Fatalf("status = %q, want prefix %q", got.Status, tc.prefix)
			}
		})
	}
}

func TestProjectPermissionPromptMsgIsRequested(t *testing.T) {
	got := projectRuntimeMessage(harness.PermissionPromptMsg{
		ToolCallID: "tc-1", ToolName: "Write",
		Description: "Write workspace/foo.txt",
		Input:       []byte(`{"path":"workspace/foo.txt"}`),
	}).(harnessshell.PermissionRequestedEvent)
	if got.Request.ID != "tc-1" || got.Request.ToolLabel != "Write" {
		t.Fatalf("request = %+v", got.Request)
	}
	if !strings.Contains(got.Request.Target, "workspace/foo.txt") {
		t.Fatalf("target = %q", got.Request.Target)
	}
	if got.Request.Summary != "Write workspace/foo.txt" {
		t.Fatalf("summary = %q", got.Request.Summary)
	}
}

func TestProjectConnStateMsgIsHostStatus(t *testing.T) {
	cases := []struct {
		state string
		kind  harnessshell.StatusKind
	}{
		{"connected", harnessshell.StatusReady},
		{"connecting", harnessshell.StatusStreaming},
		{"reconnecting", harnessshell.StatusStreaming},
		{"disconnected", harnessshell.StatusError},
		{"error", harnessshell.StatusError},
		{"weird-unknown", harnessshell.StatusReady},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			got := projectRuntimeMessage(harness.ConnStateMsg{
				Info: harness.ConnStateInfo{State: tc.state},
			}).(harnessshell.HostStatusEvent)
			if got.Kind != tc.kind {
				t.Fatalf("state %q kind = %v, want %v", tc.state, got.Kind, tc.kind)
			}
		})
	}
}

func TestProjectModelContextCostUpdateAreHostStatus(t *testing.T) {
	model := projectRuntimeMessage(harness.ModelUpdateMsg{Name: "claude-opus-4-7"}).(harnessshell.HostStatusEvent)
	if !strings.Contains(model.Status, "claude-opus-4-7") {
		t.Fatalf("model status = %q", model.Status)
	}
	ctx := projectRuntimeMessage(harness.ContextUpdateMsg{Pct: 42, Used: 4200, Max: 10000}).(harnessshell.HostStatusEvent)
	if !strings.Contains(ctx.Status, "42%") || !strings.Contains(ctx.Status, "4200") {
		t.Fatalf("context status = %q", ctx.Status)
	}
	cost := projectRuntimeMessage(harness.CostUpdateMsg{Total: 0.0123}).(harnessshell.HostStatusEvent)
	if !strings.Contains(cost.Status, "0.0123") {
		t.Fatalf("cost status = %q", cost.Status)
	}
}

func TestProjectIgnoresUnrelatedMessages(t *testing.T) {
	// After WU-106 the App-only msg types (BannerMsg, PasteDetectedMsg,
	// etc.) are deleted; the projection layer continues to ignore any
	// unknown type. We verify with a couple of tea.Msg shapes that
	// don't match the runtime-event keep list.
	for _, msg := range []interface{}{
		struct{ unrelatedFooMsg int }{},
		"plain string",
		42,
		nil,
	} {
		if got := projectRuntimeMessage(msg); got != nil {
			t.Fatalf("projectRuntimeMessage(%T) = %T, want nil", msg, got)
		}
	}
}

func TestAdapterForwardsProjectedPermissionRequestToShell(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	// PermissionPromptMsg projects to PermissionRequestedEvent which
	// the shell renders as a composer-hosted permission control. The
	// summary text is visible in the rendered output, providing an
	// end-to-end check of adapter → projection → shell.Update →
	// rendered chrome.
	updated, _ := a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = updated.(Adapter)

	updated, _ = a.Update(harness.PermissionPromptMsg{
		ToolCallID:  "tc-1",
		ToolName:    "Read",
		Description: "PROJECTED-PERMISSION-SUMMARY",
		Input:       []byte(`{"path":"foo.txt"}`),
	})
	a = updated.(Adapter)

	view := a.View()
	if !strings.Contains(view, "PROJECTED-PERMISSION-SUMMARY") {
		t.Fatalf("expected projected permission summary in view; got:\n%s", view)
	}
}

// Compile-time check that Adapter satisfies tea.Model.
var _ tea.Model = Adapter{}
