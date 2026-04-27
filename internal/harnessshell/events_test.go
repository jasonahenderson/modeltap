package harnessshell

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Stage C-3 tests for submit-action emission and run-lifecycle host-event
// intake. These exercise the shell-owned state machine; WU-102 still owns
// parity coverage against the spike behavior.

func enterKey() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

func altEnterKey() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter, Alt: true}
}

// drainActions runs Update once with msg, collects emitted ActionMsgs, and
// returns the updated model and the actions in emission order.
func drainActions(t *testing.T, m Model, msg tea.Msg) (Model, []Action) {
	t.Helper()
	updated, cmd := m.Update(msg)
	mu := updated.(Model)
	if cmd == nil {
		return mu, nil
	}
	out := cmd()
	collected, ok := collectActionMsg(out)
	if !ok {
		// Non-action cmds (e.g. textarea blink) are valid; just no actions.
		return mu, nil
	}
	actions := make([]Action, 0, len(collected))
	for _, am := range collected {
		actions = append(actions, am.Action)
	}
	return mu, actions
}

func newWithFixedClock() Model {
	m := New()
	m.state.now = func() time.Time { return time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC) }
	return m
}

func TestEnterSubmitDirectEmitsAction(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("hello world")

	m, actions := drainActions(t, m, enterKey())

	if len(actions) != 1 {
		t.Fatalf("expected 1 SubmitTurnAction, got %d (%+v)", len(actions), actions)
	}
	sub, ok := actions[0].(SubmitTurnAction)
	if !ok {
		t.Fatalf("action[0] = %T, want SubmitTurnAction", actions[0])
	}
	if sub.Submission.Text != "hello world" {
		t.Fatalf("submission text = %q, want %q", sub.Submission.Text, "hello world")
	}
	if sub.Submission.Source != SubmissionSourceDirect {
		t.Fatalf("submission source = %v, want %v", sub.Submission.Source, SubmissionSourceDirect)
	}
	if sub.Submission.ID == "" {
		t.Fatalf("submission ID empty")
	}
	if !sub.Submission.RequestedAt.Equal(time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("RequestedAt = %v, want fixed clock value", sub.Submission.RequestedAt)
	}
	if !m.state.streaming {
		t.Fatalf("streaming should be true after direct submit")
	}
	if got := len(m.state.transcriptItems); got != 2 {
		t.Fatalf("transcript items = %d, want 2 (user + assistant placeholder)", got)
	}
	if m.state.transcriptItems[0].Role != RoleUser {
		t.Fatalf("first row role = %v, want RoleUser", m.state.transcriptItems[0].Role)
	}
	if m.state.transcriptItems[1].Role != RoleAssistant || !m.state.transcriptItems[1].Streaming {
		t.Fatalf("second row should be streaming assistant placeholder; got %+v", m.state.transcriptItems[1])
	}
	if m.state.input.Value() != "" {
		t.Fatalf("input should reset after submit; got %q", m.state.input.Value())
	}
}

func TestEnterSubmitShellNativeClearDoesNotEmit(t *testing.T) {
	m := newWithFixedClock()
	m.state.transcriptItems = []TranscriptItem{
		{Role: RoleUser, Text: "old"},
		{Role: RoleAssistant, Text: "older"},
	}
	m.state.input.SetValue("/clear")

	m, actions := drainActions(t, m, enterKey())
	if len(actions) != 0 {
		t.Fatalf("/clear should not emit actions, got %d", len(actions))
	}
	if len(m.state.transcriptItems) != 0 {
		t.Fatalf("transcript should be empty after /clear, got %d items", len(m.state.transcriptItems))
	}
	if m.state.statusKind != StatusReady {
		t.Fatalf("statusKind = %v, want StatusReady", m.state.statusKind)
	}
}

func TestEnterWhileStreamingEnqueues(t *testing.T) {
	m := newWithFixedClock()
	m.state.streaming = true
	m.state.input.SetValue("follow up")

	m, actions := drainActions(t, m, enterKey())
	if len(actions) != 0 {
		t.Fatalf("enqueue should not emit actions, got %d", len(actions))
	}
	if len(m.state.queuedSubmissions) != 1 {
		t.Fatalf("expected 1 queued submission, got %d", len(m.state.queuedSubmissions))
	}
	if got := m.state.queuedSubmissions[0].Text; got != "follow up" {
		t.Fatalf("queued text = %q, want %q", got, "follow up")
	}
}

func TestEmptyEnterReleasesQueueWhenIdle(t *testing.T) {
	m := newWithFixedClock()
	m.state.queuedSubmissions = []QueuedSubmission{{ID: "q-1", Text: "queued one", Entries: []string{"queued one"}}}

	m, actions := drainActions(t, m, enterKey())
	if len(actions) != 1 {
		t.Fatalf("expected 1 SubmitTurnAction from queue release, got %d", len(actions))
	}
	sub := actions[0].(SubmitTurnAction)
	if sub.Submission.Source != SubmissionSourceQueueRelease {
		t.Fatalf("source = %v, want SubmissionSourceQueueRelease", sub.Submission.Source)
	}
	if sub.Submission.Text != "queued one" {
		t.Fatalf("text = %q, want %q", sub.Submission.Text, "queued one")
	}
	if len(m.state.queuedSubmissions) != 0 {
		t.Fatalf("queue should be drained, still has %d", len(m.state.queuedSubmissions))
	}
	if !m.state.streaming {
		t.Fatalf("streaming should be true after release")
	}
}

func TestEmptyEnterIdleNoQueueIsNoOp(t *testing.T) {
	m := newWithFixedClock()
	m, actions := drainActions(t, m, enterKey())
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(actions))
	}
	if m.state.streaming {
		t.Fatalf("streaming should remain false")
	}
	if len(m.state.transcriptItems) != 0 {
		t.Fatalf("transcript should remain empty")
	}
}

func TestAltEnterInsertsNewline(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("first")
	m, actions := drainActions(t, m, altEnterKey())
	if len(actions) != 0 {
		t.Fatalf("Alt+Enter should not submit, got %d actions", len(actions))
	}
	if !strings.Contains(m.state.input.Value(), "\n") {
		t.Fatalf("Alt+Enter should insert newline; input = %q", m.state.input.Value())
	}
}

func TestRunLifecycleHappyPath(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("ping")
	m, actions := drainActions(t, m, enterKey())
	sub := actions[0].(SubmitTurnAction)
	subID := sub.Submission.ID

	m, _ = drainActions(t, m, SubmissionAcceptedEvent{SubmissionID: subID, RunID: "run-1"})
	if got := m.state.transcriptItems[1].RunID; got != "run-1" {
		t.Fatalf("placeholder RunID = %q, want %q", got, "run-1")
	}
	m, _ = drainActions(t, m, RunStartedEvent{SubmissionID: subID, RunID: "run-1", Label: "test-label"})
	if m.state.label != "test-label" {
		t.Fatalf("label = %q, want %q", m.state.label, "test-label")
	}
	m, _ = drainActions(t, m, RunDeltaEvent{RunID: "run-1", Delta: "hello "})
	m, _ = drainActions(t, m, RunDeltaEvent{RunID: "run-1", Delta: "there"})
	if got := m.state.transcriptItems[1].Text; got != "hello there" {
		t.Fatalf("assistant text = %q, want %q", got, "hello there")
	}
	if !m.state.transcriptItems[1].Streaming {
		t.Fatalf("assistant should still be streaming before completion")
	}
	m, _ = drainActions(t, m, RunCompletedEvent{RunID: "run-1"})
	if m.state.streaming {
		t.Fatalf("streaming should be false after RunCompletedEvent")
	}
	if m.state.transcriptItems[1].Streaming {
		t.Fatalf("assistant row should be non-streaming after RunCompletedEvent")
	}
	if m.state.activeRunID != "" {
		t.Fatalf("activeRunID should reset, got %q", m.state.activeRunID)
	}
}

func TestRunCompletedAutoReleasesQueue(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("first")
	m, firstActions := drainActions(t, m, enterKey())
	firstSub := firstActions[0].(SubmitTurnAction)

	// While streaming, enqueue a follow-up.
	m.state.input.SetValue("follow up")
	m, _ = drainActions(t, m, enterKey())
	if len(m.state.queuedSubmissions) != 1 {
		t.Fatalf("expected 1 queued, got %d", len(m.state.queuedSubmissions))
	}

	m, _ = drainActions(t, m, SubmissionAcceptedEvent{SubmissionID: firstSub.Submission.ID, RunID: "run-1"})
	m, releaseActions := drainActions(t, m, RunCompletedEvent{RunID: "run-1"})

	if len(releaseActions) != 1 {
		t.Fatalf("expected auto-release SubmitTurnAction, got %d", len(releaseActions))
	}
	released := releaseActions[0].(SubmitTurnAction)
	if released.Submission.Source != SubmissionSourceQueueRelease {
		t.Fatalf("auto-release source = %v, want SubmissionSourceQueueRelease", released.Submission.Source)
	}
	if released.Submission.Text != "follow up" {
		t.Fatalf("auto-release text = %q, want %q", released.Submission.Text, "follow up")
	}
	if !m.state.streaming {
		t.Fatalf("auto-release should set streaming=true")
	}
}

func TestRunStoppedDoesNotAutoRelease(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("first")
	m, firstActions := drainActions(t, m, enterKey())
	firstSub := firstActions[0].(SubmitTurnAction)

	m.state.input.SetValue("waiting")
	m, _ = drainActions(t, m, enterKey())
	if len(m.state.queuedSubmissions) != 1 {
		t.Fatalf("expected 1 queued, got %d", len(m.state.queuedSubmissions))
	}

	m, _ = drainActions(t, m, SubmissionAcceptedEvent{SubmissionID: firstSub.Submission.ID, RunID: "run-1"})
	m, stopActions := drainActions(t, m, RunStoppedEvent{RunID: "run-1", Reason: StopReasonInterrupt})

	if len(stopActions) != 0 {
		t.Fatalf("RunStopped must NOT emit auto-release actions; got %d", len(stopActions))
	}
	if len(m.state.queuedSubmissions) != 1 {
		t.Fatalf("queue should remain after stop; have %d", len(m.state.queuedSubmissions))
	}
	if m.state.streaming {
		t.Fatalf("streaming should be false after RunStoppedEvent")
	}
	if m.state.status != "Interrupted" {
		t.Fatalf("status = %q, want %q", m.state.status, "Interrupted")
	}
}

func TestSubmissionFailedRemovesPlaceholder(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("doomed")
	m, actions := drainActions(t, m, enterKey())
	subID := actions[0].(SubmitTurnAction).Submission.ID
	if len(m.state.transcriptItems) != 2 {
		t.Fatalf("expected 2 placeholder rows, got %d", len(m.state.transcriptItems))
	}

	m, _ = drainActions(t, m, SubmissionFailedEvent{SubmissionID: subID, Message: "rate limited"})

	if len(m.state.transcriptItems) != 0 {
		t.Fatalf("placeholders should be removed on SubmissionFailed; have %d", len(m.state.transcriptItems))
	}
	if m.state.streaming {
		t.Fatalf("streaming should be false after SubmissionFailed")
	}
	if m.state.statusKind != StatusError {
		t.Fatalf("statusKind = %v, want StatusError", m.state.statusKind)
	}
	if !strings.Contains(m.state.status, "rate limited") {
		t.Fatalf("status %q should mention failure message", m.state.status)
	}
}

func TestEscArmsThenEmitsInterrupt(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("ping")
	m, _ = drainActions(t, m, enterKey())
	m, _ = drainActions(t, m, SubmissionAcceptedEvent{SubmissionID: "sub-1", RunID: "run-1"})
	if !m.state.streaming {
		t.Fatalf("streaming should be true after submit")
	}

	// First Esc arms the interrupt without emitting.
	m, actions := drainActions(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if len(actions) != 0 {
		t.Fatalf("first Esc should not emit; got %d actions", len(actions))
	}
	if !m.state.interruptArmed {
		t.Fatalf("first Esc should arm interrupt")
	}
	if m.state.statusKind != StatusInterruptArmed {
		t.Fatalf("statusKind = %v, want StatusInterruptArmed", m.state.statusKind)
	}

	// Second Esc emits InterruptRunAction.
	m, actions = drainActions(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if len(actions) != 1 {
		t.Fatalf("second Esc should emit 1 action, got %d", len(actions))
	}
	interrupt, ok := actions[0].(InterruptRunAction)
	if !ok {
		t.Fatalf("action[0] = %T, want InterruptRunAction", actions[0])
	}
	if interrupt.RunID != "run-1" {
		t.Fatalf("interrupt RunID = %q, want %q", interrupt.RunID, "run-1")
	}
	if m.state.interruptArmed {
		t.Fatalf("interruptArmed should reset after emit")
	}

	// The host then emits RunStoppedEvent which clears streaming and
	// preserves any queued work per FEAT-0014.
	m, _ = drainActions(t, m, RunStoppedEvent{RunID: "run-1", Reason: StopReasonInterrupt})
	if m.state.streaming {
		t.Fatalf("streaming should clear after RunStoppedEvent")
	}
}

func TestEscWhileNotStreamingIsIgnored(t *testing.T) {
	m := newWithFixedClock()
	m, actions := drainActions(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if len(actions) != 0 {
		t.Fatalf("Esc while idle should not emit; got %d", len(actions))
	}
	if m.state.interruptArmed {
		t.Fatalf("Esc while idle should not arm interrupt")
	}
}

func TestRunDeltaWithoutCorrelationFallsBackToLastStreaming(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("ping")
	m, _ = drainActions(t, m, enterKey())

	// Delta arrives before SubmissionAcceptedEvent — RunID not yet
	// correlated. The delta should fall back to the last streaming
	// assistant row so output isn't dropped.
	m, _ = drainActions(t, m, RunDeltaEvent{RunID: "run-1", Delta: "early"})
	if got := m.state.transcriptItems[1].Text; got != "early" {
		t.Fatalf("fallback delta application failed; assistant text = %q", got)
	}
}
