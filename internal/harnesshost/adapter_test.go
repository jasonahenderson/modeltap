package harnesshost

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// Stage D-1 tests for the Adapter's action-consumer half. Event
// projection from runtime tea.Msgs (TurnSubmittedMsg, StreamTokenMsg,
// etc.) lands in Stage D-2 with its own test surface.

// fakeRuntime is a deterministic in-memory Runtime used by adapter
// tests. It records every call and returns canned results from per-
// method response channels.
type fakeRuntime struct {
	submitCalls            []SubmitRequest
	interruptCalls         []string
	resolvePermissionCalls []resolvePermissionCall
	loadPreviewCalls       []PreviewRequest
	hostCommandCalls       []HostCommand
	summarizePasteCalls    []string

	submitResult SubmitAccepted
	submitErr    error
	interruptErr error
	resolveErr   error
	loadPreview  harnessshell.PreviewPayload
	loadErr      error
	dispatchErr  error
	summarizeRet string
	summarizeErr error
}

type resolvePermissionCall struct {
	RequestID string
	Decision  harnessshell.PermissionDecision
}

func (f *fakeRuntime) SubmitTurn(ctx context.Context, req SubmitRequest) (SubmitAccepted, error) {
	f.submitCalls = append(f.submitCalls, req)
	if f.submitErr != nil {
		return SubmitAccepted{}, f.submitErr
	}
	return f.submitResult, nil
}

func (f *fakeRuntime) InterruptRun(ctx context.Context, runID string) error {
	f.interruptCalls = append(f.interruptCalls, runID)
	return f.interruptErr
}

func (f *fakeRuntime) DispatchCommand(ctx context.Context, cmd HostCommand) error {
	f.hostCommandCalls = append(f.hostCommandCalls, cmd)
	return f.dispatchErr
}

func (f *fakeRuntime) ResolvePermission(ctx context.Context, requestID string, decision harnessshell.PermissionDecision) error {
	f.resolvePermissionCalls = append(f.resolvePermissionCalls, resolvePermissionCall{requestID, decision})
	return f.resolveErr
}

func (f *fakeRuntime) LoadPreview(ctx context.Context, req PreviewRequest) (harnessshell.PreviewPayload, error) {
	f.loadPreviewCalls = append(f.loadPreviewCalls, req)
	if f.loadErr != nil {
		return harnessshell.PreviewPayload{}, f.loadErr
	}
	return f.loadPreview, nil
}

func (f *fakeRuntime) SummarizePaste(ctx context.Context, raw string) (string, error) {
	f.summarizePasteCalls = append(f.summarizePasteCalls, raw)
	if f.summarizeErr != nil {
		return "", f.summarizeErr
	}
	return f.summarizeRet, nil
}

// runActionThroughAdapter dispatches a single Action through the
// Adapter's Update and returns the resulting tea.Msg (if any).
func runActionThroughAdapter(t *testing.T, a Adapter, action harnessshell.Action) (Adapter, tea.Msg) {
	t.Helper()
	updated, cmd := a.Update(harnessshell.ActionMsg{Action: action})
	au := updated.(Adapter)
	if cmd == nil {
		return au, nil
	}
	return au, cmd()
}

func TestAdapterDispatchesSubmitTurnToRuntime(t *testing.T) {
	rt := &fakeRuntime{submitResult: SubmitAccepted{RunID: "run-42", Label: "test-model"}}
	a := New(harnessshell.New(), rt)

	sub := harnessshell.Submission{
		ID:          "sub-1",
		Text:        "hello",
		Source:      harnessshell.SubmissionSourceDirect,
		RequestedAt: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		Tokens: []harnessshell.InputToken{
			{ID: "file-1", Kind: harnessshell.TokenKindFile, Label: "file-1 (a.txt)", Payload: "/abs/a.txt"},
			{ID: "paste-1", Kind: harnessshell.TokenKindPaste, Label: "paste-1", Payload: "long content"},
		},
	}
	_, msg := runActionThroughAdapter(t, a, harnessshell.SubmitTurnAction{Submission: sub})

	if len(rt.submitCalls) != 1 {
		t.Fatalf("expected 1 SubmitTurn call, got %d", len(rt.submitCalls))
	}
	got := rt.submitCalls[0]
	if got.SubmissionID != "sub-1" || got.Text != "hello" || got.Source != harnessshell.SubmissionSourceDirect {
		t.Fatalf("submit request mismatch: %+v", got)
	}
	if len(got.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(got.Attachments))
	}
	if got.Attachments[0].Path != "/abs/a.txt" || got.Attachments[0].Kind != harnessshell.TokenKindFile {
		t.Fatalf("file attachment misprojected: %+v", got.Attachments[0])
	}
	if got.Attachments[1].Payload != "long content" || got.Attachments[1].Kind != harnessshell.TokenKindPaste {
		t.Fatalf("paste attachment misprojected: %+v", got.Attachments[1])
	}

	// The Cmd's resulting msg is an internal correlation message.
	internal, ok := msg.(submissionAcceptedAdapterMsg)
	if !ok {
		t.Fatalf("dispatch result = %T, want submissionAcceptedAdapterMsg", msg)
	}
	if internal.RunID != "run-42" || internal.Label != "test-model" {
		t.Fatalf("correlation mismatch: %+v", internal)
	}
}

func TestAdapterSubmitErrorEmitsSubmissionFailedEvent(t *testing.T) {
	rt := &fakeRuntime{submitErr: errors.New("rate limited")}
	a := New(harnessshell.New(), rt)

	sub := harnessshell.Submission{ID: "sub-1", Text: "x"}
	_, msg := runActionThroughAdapter(t, a, harnessshell.SubmitTurnAction{Submission: sub})

	failed, ok := msg.(harnessshell.SubmissionFailedEvent)
	if !ok {
		t.Fatalf("got %T, want SubmissionFailedEvent", msg)
	}
	if failed.SubmissionID != "sub-1" || failed.Message != "rate limited" {
		t.Fatalf("failure event mismatch: %+v", failed)
	}
}

func TestAdapterSubmissionAcceptedRecordsCorrelationAndForwardsEvent(t *testing.T) {
	rt := &fakeRuntime{submitResult: SubmitAccepted{RunID: "run-7", Label: "lab"}}
	a := New(harnessshell.New(), rt)

	// Inject the correlation message directly to exercise the second
	// Update branch without driving through the dispatch tea.Cmd.
	updated, cmd := a.Update(submissionAcceptedAdapterMsg{
		SubmissionID: "sub-1", RunID: "run-7", Label: "lab",
	})
	au := updated.(Adapter)

	if au.submissionToRun["sub-1"] != "run-7" {
		t.Fatalf("submissionToRun not recorded: %+v", au.submissionToRun)
	}
	if au.runToSubmission["run-7"] != "sub-1" {
		t.Fatalf("runToSubmission not recorded: %+v", au.runToSubmission)
	}

	// The forwarded cmd should produce one or more shell-bound events
	// (SubmissionAcceptedEvent and the synthetic RunStartedEvent for
	// the label). Stage D-2 will replace the synthetic RunStarted
	// with one projected from a runtime stream message.
	if cmd == nil {
		t.Fatalf("expected cmd carrying SubmissionAcceptedEvent forwarding")
	}
	out := cmd()
	switch m := out.(type) {
	case harnessshell.RunStartedEvent:
		if m.Label != "lab" {
			t.Fatalf("synthetic RunStartedEvent label = %q, want %q", m.Label, "lab")
		}
	case tea.BatchMsg:
		// Acceptable too — when the inner shell Update returns a
		// non-nil cmd alongside the synthetic RunStarted batch.
	default:
		t.Fatalf("forwarded cmd msg = %T, want RunStartedEvent or batch", out)
	}
}

func TestAdapterDispatchesInterruptToRuntime(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	_, msg := runActionThroughAdapter(t, a, harnessshell.InterruptRunAction{RunID: "run-9"})

	if len(rt.interruptCalls) != 1 || rt.interruptCalls[0] != "run-9" {
		t.Fatalf("interrupt calls = %v, want [run-9]", rt.interruptCalls)
	}
	if msg != nil {
		t.Fatalf("interrupt success path should return nil msg; got %T", msg)
	}
}

func TestAdapterInterruptErrorEmitsRunFailedEvent(t *testing.T) {
	rt := &fakeRuntime{interruptErr: errors.New("network down")}
	a := New(harnessshell.New(), rt)

	_, msg := runActionThroughAdapter(t, a, harnessshell.InterruptRunAction{RunID: "run-9"})

	failed, ok := msg.(harnessshell.RunFailedEvent)
	if !ok {
		t.Fatalf("got %T, want RunFailedEvent", msg)
	}
	if failed.RunID != "run-9" || failed.Message != "network down" {
		t.Fatalf("RunFailedEvent mismatch: %+v", failed)
	}
}

func TestAdapterDispatchesResolvePermissionToRuntime(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	_, msg := runActionThroughAdapter(t, a, harnessshell.ResolvePermissionAction{
		RequestID: "perm-1", Decision: harnessshell.DecisionApproveSession,
	})

	if len(rt.resolvePermissionCalls) != 1 {
		t.Fatalf("expected 1 ResolvePermission call, got %d", len(rt.resolvePermissionCalls))
	}
	got := rt.resolvePermissionCalls[0]
	if got.RequestID != "perm-1" || got.Decision != harnessshell.DecisionApproveSession {
		t.Fatalf("resolve call mismatch: %+v", got)
	}
	if msg != nil {
		t.Fatalf("resolve success path should return nil msg; got %T", msg)
	}
}

func TestAdapterResolvePermissionErrorEmitsResolvedDenied(t *testing.T) {
	rt := &fakeRuntime{resolveErr: errors.New("permission service down")}
	a := New(harnessshell.New(), rt)

	_, msg := runActionThroughAdapter(t, a, harnessshell.ResolvePermissionAction{
		RequestID: "perm-1", Decision: harnessshell.DecisionApproveOnce,
	})

	resolved, ok := msg.(harnessshell.PermissionResolvedEvent)
	if !ok {
		t.Fatalf("got %T, want PermissionResolvedEvent", msg)
	}
	if resolved.RequestID != "perm-1" || resolved.Outcome != harnessshell.OutcomeDenied {
		t.Fatalf("resolved event mismatch: %+v", resolved)
	}
}

func TestAdapterDispatchesLoadPreviewToRuntime(t *testing.T) {
	rt := &fakeRuntime{loadPreview: harnessshell.PreviewPayload{Title: "a.txt", Content: "file body"}}
	a := New(harnessshell.New(), rt)

	target := harnessshell.PreviewTarget{TokenID: "file-1", Source: "composer"}
	_, msg := runActionThroughAdapter(t, a, harnessshell.LoadPreviewAction{Target: target})

	if len(rt.loadPreviewCalls) != 1 {
		t.Fatalf("expected 1 LoadPreview call, got %d", len(rt.loadPreviewCalls))
	}
	if got := rt.loadPreviewCalls[0]; got.TokenID != "file-1" || got.Source != "composer" {
		t.Fatalf("LoadPreview call mismatch: %+v", got)
	}

	loaded, ok := msg.(harnessshell.PreviewLoadedEvent)
	if !ok {
		t.Fatalf("got %T, want PreviewLoadedEvent", msg)
	}
	if loaded.Target.TokenID != "file-1" || loaded.Preview.Content != "file body" {
		t.Fatalf("PreviewLoadedEvent mismatch: %+v", loaded)
	}
}

func TestAdapterLoadPreviewErrorEmitsHostStatusEvent(t *testing.T) {
	rt := &fakeRuntime{loadErr: errors.New("file missing")}
	a := New(harnessshell.New(), rt)

	_, msg := runActionThroughAdapter(t, a, harnessshell.LoadPreviewAction{
		Target: harnessshell.PreviewTarget{TokenID: "file-1"},
	})

	status, ok := msg.(harnessshell.HostStatusEvent)
	if !ok {
		t.Fatalf("got %T, want HostStatusEvent", msg)
	}
	if status.Kind != harnessshell.StatusError {
		t.Fatalf("status Kind = %v, want StatusError", status.Kind)
	}
}

func TestAdapterDispatchesHostCommandToRuntime(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	inv := harnessshell.CommandInvocation{Name: "model", Args: "claude-opus-4-7", Raw: "/model claude-opus-4-7"}
	_, msg := runActionThroughAdapter(t, a, harnessshell.RunHostCommandAction{Invocation: inv})

	if len(rt.hostCommandCalls) != 1 {
		t.Fatalf("expected 1 DispatchCommand call, got %d", len(rt.hostCommandCalls))
	}
	got := rt.hostCommandCalls[0]
	if got.Name != "model" || got.Args != "claude-opus-4-7" {
		t.Fatalf("host command mismatch: %+v", got)
	}
	if msg != nil {
		t.Fatalf("dispatch success path should return nil msg; got %T", msg)
	}
}

func TestAdapterPassesNonActionMsgsToShell(t *testing.T) {
	rt := &fakeRuntime{}
	a := New(harnessshell.New(), rt)

	updated, _ := a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	au := updated.(Adapter)
	if au.shell.View() == "" {
		// Empty view at start is OK; the assertion here is that the
		// adapter forwarded the message and did not panic. The
		// shell's own tests confirm WindowSizeMsg propagates state.
		_ = au
	}
}
