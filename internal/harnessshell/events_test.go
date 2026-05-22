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

// PATCH-0035: RunStartedEvent stamps runStartedAt and queues a
// streamTickCmd; the rendered status appends "(Ns)" once the clock
// advances; RunCompletedEvent clears the timestamp and the next tick
// is a no-op so the loop ends.
func TestStreamingStatusAppendsElapsedSeconds(t *testing.T) {
	m := New()
	nowAt := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	m.state.now = func() time.Time { return nowAt }

	// Drive a RunStartedEvent through Update so the same pendingCmds
	// drain path the host hits is exercised.
	updated, cmd := m.Update(RunStartedEvent{RunID: "run-1", Label: "qwen"})
	m = updated.(Model)
	if m.state.runStartedAt.IsZero() {
		t.Fatalf("runStartedAt not set after RunStartedEvent")
	}
	if cmd == nil {
		t.Fatalf("RunStartedEvent did not return a tick cmd")
	}
	// Status during streaming with elapsed=0s is just the base
	// status (composeStatusWithElapsed gates on elapsed > 0).
	if got := composeStatusWithElapsed(&m.state); !strings.Contains(got, "Streaming") {
		t.Errorf("base status missing 'Streaming': %q", got)
	}

	// Advance the clock 4s and recheck — the rendered status should
	// now carry "(4s)".
	nowAt = nowAt.Add(4 * time.Second)
	if got := composeStatusWithElapsed(&m.state); !strings.Contains(got, "(4s)") {
		t.Errorf("status missing '(4s)' after 4s elapsed: %q", got)
	}

	// A streamTickMsg while streaming reschedules itself.
	_, tickCmd := m.Update(streamTickMsg(nowAt))
	if tickCmd == nil {
		t.Errorf("streamTickMsg during streaming did not reschedule")
	}

	// RunCompletedEvent clears runStartedAt and streaming; the next
	// tick is a no-op (no rescheduled cmd).
	updated, _ = m.Update(RunCompletedEvent{RunID: "run-1"})
	m = updated.(Model)
	if !m.state.runStartedAt.IsZero() {
		t.Errorf("runStartedAt not cleared on RunCompletedEvent: %v", m.state.runStartedAt)
	}
	if m.state.streaming {
		t.Errorf("streaming still true after RunCompletedEvent")
	}
	_, tickCmd = m.Update(streamTickMsg(nowAt.Add(5 * time.Second)))
	if tickCmd != nil {
		// drainPendingActions and other paths can return nil cmds;
		// the tick loop must not reschedule itself once streaming
		// has stopped.
		if got, ok := tickCmd().(streamTickMsg); ok {
			t.Errorf("streamTickMsg after completion rescheduled itself: got %v", got)
		}
	}
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

// PATCH-0038: /clear is now host-routed. Pressing Enter on /clear
// emits RunHostCommandAction{Name: "clear"} (just like any other host
// command); the shell does NOT wipe the transcript locally. The wipe
// happens later on receipt of TranscriptClearEvent (covered by
// TestTranscriptClearEventWipesTranscript below).
func TestEnterSlashClearDispatchesHostCommand(t *testing.T) {
	m := newWithFixedClock()
	m.state.transcriptItems = []TranscriptItem{
		{Role: RoleUser, Text: "old"},
		{Role: RoleAssistant, Text: "older"},
	}
	m.state.input.SetValue("/clear")

	m, actions := drainActions(t, m, enterKey())
	if len(actions) != 1 {
		t.Fatalf("/clear should emit one RunHostCommandAction, got %d", len(actions))
	}
	run, ok := actions[0].(RunHostCommandAction)
	if !ok {
		t.Fatalf("action[0] = %T, want RunHostCommandAction", actions[0])
	}
	if run.Invocation.Name != "clear" {
		t.Fatalf("invocation name = %q, want %q", run.Invocation.Name, "clear")
	}
	// Transcript untouched until host emits TranscriptClearEvent.
	if len(m.state.transcriptItems) != 2 {
		t.Fatalf("transcript should be untouched until TranscriptClearEvent, got %d items", len(m.state.transcriptItems))
	}
}

// PATCH-0038: TranscriptClearEvent is what actually wipes the visible
// transcript. The host emits it after a successful Runtime session.create
// triggered by /clear.
func TestTranscriptClearEventWipesTranscript(t *testing.T) {
	m := newWithFixedClock()
	m.state.transcriptItems = []TranscriptItem{
		{Role: RoleUser, Text: "old"},
		{Role: RoleAssistant, Text: "older"},
	}
	m.state.queuedSubmissions = []QueuedSubmission{{ID: "q-1", Text: "queued"}}

	updated, _ := m.Update(TranscriptClearEvent{})
	m = updated.(Model)
	if len(m.state.transcriptItems) != 0 {
		t.Errorf("transcript not wiped: %d items", len(m.state.transcriptItems))
	}
	if len(m.state.queuedSubmissions) != 0 {
		t.Errorf("queue not wiped: %d items", len(m.state.queuedSubmissions))
	}
}

func TestEnterHostCommandEmitsRunHostCommandAction(t *testing.T) {
	cases := []struct {
		input    string
		wantName string
		wantArgs string
	}{
		{"/models", "models", ""},
		{"/model qwen3.5:35b", "model", "qwen3.5:35b"},
		{"/sessions", "sessions", ""},
		{"/run policy", "run", "policy"},
		{"/attach abc-123", "attach", "abc-123"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			m := newWithFixedClock()
			m.state.input.SetValue(tc.input)

			m, actions := drainActions(t, m, enterKey())

			if len(actions) != 1 {
				t.Fatalf("expected 1 action, got %d (%+v)", len(actions), actions)
			}
			run, ok := actions[0].(RunHostCommandAction)
			if !ok {
				t.Fatalf("action[0] = %T, want RunHostCommandAction", actions[0])
			}
			if run.Invocation.Name != tc.wantName {
				t.Errorf("name = %q, want %q", run.Invocation.Name, tc.wantName)
			}
			if run.Invocation.Args != tc.wantArgs {
				t.Errorf("args = %q, want %q", run.Invocation.Args, tc.wantArgs)
			}
			if run.Invocation.Raw != tc.input {
				t.Errorf("raw = %q, want %q", run.Invocation.Raw, tc.input)
			}
			// No optimistic transcript rows for host commands.
			if len(m.state.transcriptItems) != 0 {
				t.Errorf("expected no transcript items for host command, got %d", len(m.state.transcriptItems))
			}
			// Composer reset.
			if got := m.state.input.Value(); got != "" {
				t.Errorf("input should reset after host command, got %q", got)
			}
		})
	}
}

func TestEnterBareSlashIsNotHostCommand(t *testing.T) {
	// "/" alone has no command name; should fall through to a regular
	// turn submission, not a host-command dispatch.
	m := newWithFixedClock()
	m.state.input.SetValue("/")

	_, actions := drainActions(t, m, enterKey())
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if _, ok := actions[0].(SubmitTurnAction); !ok {
		t.Fatalf("action[0] = %T, want SubmitTurnAction (bare slash is text)", actions[0])
	}
}

func TestEnterShellNativeSelectTogglesMouseCapture(t *testing.T) {
	m := newWithFixedClock()
	if !m.state.mouseCaptureDisabled {
		t.Fatalf("selection mode should be the default")
	}

	// First /select: enters chat mouse-capture mode.
	m.state.input.SetValue("/select")
	updated, cmd := m.Update(enterKey())
	mu := updated.(Model)
	if mu.state.mouseCaptureDisabled {
		t.Fatalf("first /select should set mouseCaptureDisabled=false")
	}
	if cmd == nil {
		t.Fatalf("first /select should return a Cmd (tea.EnableMouseAllMotion)")
	}
	if mu.state.input.Value() != "" {
		t.Errorf("composer should be cleared after /select; got %q", mu.state.input.Value())
	}
	if !strings.Contains(mu.state.status, "Chat mode") {
		t.Errorf("status should announce chat mode; got %q", mu.state.status)
	}

	// Second /select: returns to terminal-native selection mode.
	mu.state.input.SetValue("/select")
	updated, cmd = mu.Update(enterKey())
	mu = updated.(Model)
	if !mu.state.mouseCaptureDisabled {
		t.Fatalf("second /select should set mouseCaptureDisabled=true")
	}
	if cmd == nil {
		t.Fatalf("second /select should return a Cmd (tea.DisableMouse)")
	}
	if !strings.Contains(mu.state.status, "Selection mode") {
		t.Errorf("status should announce selection mode; got %q", mu.state.status)
	}
}

func TestEnterShellNativeSelectDoesNotEmitAction(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("/select")
	_, actions := drainActions(t, m, enterKey())
	if len(actions) != 0 {
		t.Fatalf("/select should not emit actions, got %d (%+v)", len(actions), actions)
	}
}

func TestEnterShellNativeQuitCommandsReturnQuit(t *testing.T) {
	for _, command := range []string{"/quit", "/exit"} {
		t.Run(command, func(t *testing.T) {
			m := newWithFixedClock()
			m.state.input.SetValue(command)

			updated, cmd := m.Update(enterKey())
			mu := updated.(Model)
			if cmd == nil {
				t.Fatalf("%s returned nil cmd, want tea.Quit", command)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("%s returned cmd %T, want tea.QuitMsg", command, cmd())
			}
			if got := mu.state.input.Value(); got != "" {
				t.Fatalf("input should reset after %s, got %q", command, got)
			}
		})
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

// PATCH-0036: slash commands typed during a streaming run must
// dispatch as RunHostCommandAction immediately, not get enqueued
// as user content. Before this fix, /cancel during streaming was
// queued and the in-flight run continued to completion.
func TestEnterSlashCommandDuringStreamingDispatches(t *testing.T) {
	cases := []struct {
		input    string
		wantName string
		wantArgs string
	}{
		{"/cancel run-abc", "cancel", "run-abc"},
		{"/run", "run", ""},
		{"/runs", "runs", ""},
		{"/detach", "detach", ""},
		{"/sessions clear", "sessions", "clear"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			m := newWithFixedClock()
			m.state.streaming = true
			m.state.input.SetValue(tc.input)

			m, actions := drainActions(t, m, enterKey())
			if len(actions) != 1 {
				t.Fatalf("expected 1 action, got %d (%+v)", len(actions), actions)
			}
			run, ok := actions[0].(RunHostCommandAction)
			if !ok {
				t.Fatalf("action[0] = %T, want RunHostCommandAction (slash should not enqueue during streaming)", actions[0])
			}
			if run.Invocation.Name != tc.wantName {
				t.Errorf("name = %q, want %q", run.Invocation.Name, tc.wantName)
			}
			if run.Invocation.Args != tc.wantArgs {
				t.Errorf("args = %q, want %q", run.Invocation.Args, tc.wantArgs)
			}
			if len(m.state.queuedSubmissions) != 0 {
				t.Errorf("slash command should not enqueue during streaming, got %d queued", len(m.state.queuedSubmissions))
			}
		})
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

func TestDetachedRunDeltaDoesNotMutateForegroundTranscript(t *testing.T) {
	m := newWithFixedClock()
	m.state.input.SetValue("ping")
	m, actions := drainActions(t, m, enterKey())
	subID := actions[0].(SubmitTurnAction).Submission.ID

	m, _ = drainActions(t, m, SubmissionAcceptedEvent{SubmissionID: subID, RunID: "run-attached"})
	m, _ = drainActions(t, m, RunDeltaEvent{RunID: "run-detached", Delta: "background chatter"})

	if got := m.state.transcriptItems[1].Text; got != "" {
		t.Fatalf("foreground assistant text = %q, want empty", got)
	}
	m, _ = drainActions(t, m, RunDeltaEvent{RunID: "run-attached", Delta: "foreground"})
	if got := m.state.transcriptItems[1].Text; got != "foreground" {
		t.Fatalf("foreground assistant text = %q, want foreground", got)
	}
}

func TestRunStartedWithoutSubmissionCreatesReplayRow(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, RunStartedEvent{RunID: "run-replay"})
	m, _ = drainActions(t, m, RunDeltaEvent{RunID: "run-replay", Delta: "replayed"})

	if len(m.state.transcriptItems) != 1 {
		t.Fatalf("transcript items = %d, want 1", len(m.state.transcriptItems))
	}
	row := m.state.transcriptItems[0]
	if row.RunID != "run-replay" || row.Text != "replayed" || !row.Streaming {
		t.Fatalf("replay row = %+v", row)
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

func TestPermissionRequestedAppendsTranscriptAndPending(t *testing.T) {
	m := newWithFixedClock()
	req := PermissionRequest{
		ID:        "perm-1",
		ToolLabel: "Read README",
		Target:    "README.md",
		Summary:   "Read workspace file",
	}
	m, _ = drainActions(t, m, PermissionRequestedEvent{Request: req})

	if len(m.state.transcriptItems) != 1 {
		t.Fatalf("expected 1 transcript event row, got %d", len(m.state.transcriptItems))
	}
	row := m.state.transcriptItems[0]
	if row.Kind != TranscriptItemKindEvent || row.Event == nil || row.Event.Status != "requested" || row.Event.RequestID != "perm-1" {
		t.Fatalf("event row badly initialized: %+v", row)
	}
	if len(m.state.pendingPermissions) != 1 {
		t.Fatalf("expected 1 pending permission, got %d", len(m.state.pendingPermissions))
	}
	if m.state.statusKind != StatusPermissionPending {
		t.Fatalf("statusKind = %v, want StatusPermissionPending", m.state.statusKind)
	}
}

func TestPermissionEnterEmitsResolveAction(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, PermissionRequestedEvent{Request: PermissionRequest{
		ID: "perm-1", ToolLabel: "Read", Target: "x", Summary: "do x",
	}})

	// Default selectedAction is 0 → DecisionApproveOnce.
	_, actions := drainActions(t, m, enterKey())
	if len(actions) != 1 {
		t.Fatalf("expected 1 ResolvePermissionAction, got %d", len(actions))
	}
	resolve, ok := actions[0].(ResolvePermissionAction)
	if !ok {
		t.Fatalf("action[0] = %T, want ResolvePermissionAction", actions[0])
	}
	if resolve.RequestID != "perm-1" || resolve.Decision != DecisionApproveOnce {
		t.Fatalf("resolve = %+v, want ID=perm-1 Decision=approve_once", resolve)
	}
}

func TestPermissionYNShortcutsEmitResolve(t *testing.T) {
	cases := []struct {
		name     string
		key      tea.KeyMsg
		decision PermissionDecision
	}{
		{"approve y", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}, DecisionApproveOnce},
		{"approve Y", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}, DecisionApproveOnce},
		{"deny n", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, DecisionDeny},
		{"deny N", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}, DecisionDeny},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newWithFixedClock()
			m, _ = drainActions(t, m, PermissionRequestedEvent{Request: PermissionRequest{
				ID: "perm-1", ToolLabel: "Read", Target: "x", Summary: "do x",
			}})
			_, actions := drainActions(t, m, tc.key)
			if len(actions) != 1 {
				t.Fatalf("expected 1 action, got %d", len(actions))
			}
			r, ok := actions[0].(ResolvePermissionAction)
			if !ok {
				t.Fatalf("action[0] = %T", actions[0])
			}
			if r.Decision != tc.decision {
				t.Fatalf("decision = %v, want %v", r.Decision, tc.decision)
			}
		})
	}
}

func TestPermissionLeftRightWalkActionSelector(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, PermissionRequestedEvent{Request: PermissionRequest{
		ID: "perm-1", ToolLabel: "Read", Target: "x", Summary: "do x",
	}})
	if got := m.state.pendingPermissions[0].SelectedAction; got != 0 {
		t.Fatalf("initial SelectedAction = %d, want 0", got)
	}
	m, _ = drainActions(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if got := m.state.pendingPermissions[0].SelectedAction; got != 1 {
		t.Fatalf("after Right SelectedAction = %d, want 1", got)
	}
	m, _ = drainActions(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if got := m.state.pendingPermissions[0].SelectedAction; got != 2 {
		t.Fatalf("after Right Right SelectedAction = %d, want 2", got)
	}
	// Clamps at 2 (Deny).
	m, _ = drainActions(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if got := m.state.pendingPermissions[0].SelectedAction; got != 2 {
		t.Fatalf("clamp failed; SelectedAction = %d", got)
	}
	// Enter now emits Deny.
	_, actions := drainActions(t, m, enterKey())
	r := actions[0].(ResolvePermissionAction)
	if r.Decision != DecisionDeny {
		t.Fatalf("decision = %v, want DecisionDeny", r.Decision)
	}
}

func TestPermissionUpDownWalksMultiplePending(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, PermissionRequestedEvent{Request: PermissionRequest{ID: "p1", Summary: "first"}})
	m, _ = drainActions(t, m, PermissionRequestedEvent{Request: PermissionRequest{ID: "p2", Summary: "second"}})
	if got := m.state.activePermissionIndex; got != 1 {
		t.Fatalf("activePermissionIndex = %d, want 1 after second request", got)
	}
	m, _ = drainActions(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if got := m.state.activePermissionIndex; got != 0 {
		t.Fatalf("after Up activePermissionIndex = %d, want 0", got)
	}
	m, _ = drainActions(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if got := m.state.activePermissionIndex; got != 1 {
		t.Fatalf("after Down activePermissionIndex = %d, want 1", got)
	}
}

func TestPermissionResolvedUpdatesTranscriptAndStatus(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, PermissionRequestedEvent{Request: PermissionRequest{
		ID: "perm-1", ToolLabel: "Read", Summary: "do x",
	}})
	m, _ = drainActions(t, m, PermissionResolvedEvent{
		RequestID: "perm-1",
		Outcome:   OutcomeApprovedOnce,
		Message:   "ok",
	})
	if len(m.state.pendingPermissions) != 0 {
		t.Fatalf("pending permissions should be empty after resolve, got %d", len(m.state.pendingPermissions))
	}
	if got := m.state.transcriptItems[0].Event.Status; got != "granted" {
		t.Fatalf("event row status = %q, want granted", got)
	}
	if m.state.statusKind != StatusReady {
		t.Fatalf("statusKind = %v, want StatusReady", m.state.statusKind)
	}
	if m.state.status != "ok" {
		t.Fatalf("status = %q, want %q", m.state.status, "ok")
	}
}

func TestPermissionResolvedDeniedFlipsTranscript(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, PermissionRequestedEvent{Request: PermissionRequest{ID: "perm-1", Summary: "x"}})
	m, _ = drainActions(t, m, PermissionResolvedEvent{RequestID: "perm-1", Outcome: OutcomeDenied})
	if got := m.state.transcriptItems[0].Event.Status; got != "denied" {
		t.Fatalf("event row status = %q, want denied", got)
	}
}

func TestCtrlOPasteTokenPreviewsLocally(t *testing.T) {
	m := newWithFixedClock()
	m.state.inputTokens = []InputToken{{
		ID: "paste-1", Kind: TokenKindPaste, Label: "paste-1", Payload: "long content",
	}}
	m.state.selectedToken = 0

	m, actions := drainActions(t, m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if len(actions) != 0 {
		t.Fatalf("paste preview should not emit actions, got %d", len(actions))
	}
	if m.state.preview == nil || m.state.preview.Content != "long content" {
		t.Fatalf("preview not populated for paste token: %+v", m.state.preview)
	}
}

func TestCtrlOFileTokenEmitsLoadPreview(t *testing.T) {
	m := newWithFixedClock()
	m.state.inputTokens = []InputToken{{
		ID: "file-1", Kind: TokenKindFile, Label: "file-1 (foo.txt)", Payload: "/abs/foo.txt",
	}}
	m.state.selectedToken = 0

	m, actions := drainActions(t, m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if len(actions) != 1 {
		t.Fatalf("file preview should emit LoadPreviewAction, got %d actions", len(actions))
	}
	a, ok := actions[0].(LoadPreviewAction)
	if !ok {
		t.Fatalf("action[0] = %T, want LoadPreviewAction", actions[0])
	}
	if a.Target.TokenID != "file-1" || a.Target.Source != "composer" {
		t.Fatalf("target = %+v, want composer/file-1", a.Target)
	}
	if m.state.preview != nil {
		t.Fatalf("file preview should defer painting until PreviewLoadedEvent; preview = %+v", m.state.preview)
	}
}

func TestPreviewLoadedEventPaintsDialog(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, PreviewLoadedEvent{
		Target:  PreviewTarget{TokenID: "file-1"},
		Preview: PreviewPayload{Title: "foo.txt", Content: "file contents here"},
	})
	if m.state.preview == nil {
		t.Fatalf("preview dialog should be set after PreviewLoadedEvent")
	}
	if m.state.preview.Title != "foo.txt" || m.state.preview.Content != "file contents here" {
		t.Fatalf("preview = %+v, mismatch", m.state.preview)
	}
}

func TestEscClosesPreviewBeforeArmingInterrupt(t *testing.T) {
	m := newWithFixedClock()
	// Make streaming so without preview Esc would arm interrupt.
	m.state.streaming = true
	m.state.preview = &PreviewDialog{Title: "x", Content: "y"}

	m, _ = drainActions(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.state.preview != nil {
		t.Fatalf("Esc should close preview")
	}
	if m.state.interruptArmed {
		t.Fatalf("Esc should not arm interrupt while preview was open")
	}
}

func TestHostStatusEventAppliesTextAndKind(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, HostStatusEvent{Status: "Connecting", Kind: StatusStreaming})
	if m.state.status != "Connecting" {
		t.Fatalf("status = %q, want %q", m.state.status, "Connecting")
	}
	if m.state.statusKind != StatusStreaming {
		t.Fatalf("statusKind = %v, want StatusStreaming", m.state.statusKind)
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

func TestHostInfoEventAppendsTranscriptRow(t *testing.T) {
	m := newWithFixedClock()
	text := "Available models:\n  claude-sonnet-4-6 (anthropic)\n  qwen3.5:35b (ollama-local)"

	m, _ = drainActions(t, m, HostInfoEvent{Text: text})

	if len(m.state.transcriptItems) != 1 {
		t.Fatalf("expected 1 transcript row, got %d", len(m.state.transcriptItems))
	}
	row := m.state.transcriptItems[0]
	if row.Kind != TranscriptItemKindHostInfo {
		t.Fatalf("row kind = %v, want TranscriptItemKindHostInfo", row.Kind)
	}
	if row.Role != RoleHostInfo {
		t.Fatalf("row role = %q, want %q", row.Role, RoleHostInfo)
	}
	if row.Text != text {
		t.Fatalf("row text = %q, want %q", row.Text, text)
	}
}

func TestHostInfoEventEmptyTextIsNoop(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, HostInfoEvent{Text: ""})
	if len(m.state.transcriptItems) != 0 {
		t.Fatalf("empty HostInfoEvent should not append; got %d rows", len(m.state.transcriptItems))
	}
}

func TestToolActivityEventAppendsAndUpdatesTranscriptRow(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, ToolActivityEvent{
		ID:        "tc-1",
		ToolLabel: "Read",
		Summary:   "internal/harnessshell/events.go",
		State:     ToolActivityRunning,
	})

	if got := len(m.state.transcriptItems); got != 1 {
		t.Fatalf("transcript rows = %d, want 1", got)
	}
	row := m.state.transcriptItems[0]
	if row.Kind != TranscriptItemKindEvent || row.Role != RoleEvent {
		t.Fatalf("row kind/role = %v/%q, want event/event", row.Kind, row.Role)
	}
	if row.Event == nil || row.Event.ToolCallID != "tc-1" || row.Event.Status != "running" {
		t.Fatalf("event state = %+v", row.Event)
	}
	if !strings.Contains(row.Text, "⚙ Read") || !strings.Contains(row.Text, "events.go") {
		t.Fatalf("row text = %q", row.Text)
	}
	if m.state.statusKind != StatusStreaming {
		t.Fatalf("status kind = %q, want streaming", m.state.statusKind)
	}

	m, _ = drainActions(t, m, ToolActivityEvent{
		ID:        "tc-1",
		ToolLabel: "Read",
		Summary:   "loaded 42 lines",
		State:     ToolActivitySuccess,
		Duration:  1234 * time.Millisecond,
	})

	if got := len(m.state.transcriptItems); got != 1 {
		t.Fatalf("update should keep one row, got %d", got)
	}
	row = m.state.transcriptItems[0]
	if row.Event == nil || row.Event.Status != "done" {
		t.Fatalf("updated event state = %+v", row.Event)
	}
	for _, want := range []string{"✓ Read", "loaded 42 lines", "1.234s"} {
		if !strings.Contains(row.Text, want) {
			t.Fatalf("updated row missing %q: %q", want, row.Text)
		}
	}
	if m.state.statusKind != StatusReady {
		t.Fatalf("status kind = %q, want ready", m.state.statusKind)
	}
	renderMsg := transcriptItemToRender(row)
	if renderMsg.EventKind != "tool" {
		t.Fatalf("render event kind = %q, want tool", renderMsg.EventKind)
	}
}

func TestToolActivityEventWithoutIDAppendsRows(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, ToolActivityEvent{ToolLabel: "Grep", Summary: "pattern", State: ToolActivityRunning})
	m, _ = drainActions(t, m, ToolActivityEvent{ToolLabel: "Read", Summary: "file", State: ToolActivityRunning})

	if got := len(m.state.transcriptItems); got != 2 {
		t.Fatalf("transcript rows = %d, want 2", got)
	}
	if !strings.Contains(m.state.transcriptItems[0].Text, "Grep") {
		t.Fatalf("first row = %q", m.state.transcriptItems[0].Text)
	}
	if !strings.Contains(m.state.transcriptItems[1].Text, "Read") {
		t.Fatalf("second row = %q", m.state.transcriptItems[1].Text)
	}
}

func TestToolActivitySuccessKeepsStreamingStatusDuringRun(t *testing.T) {
	m := newWithFixedClock()
	m.state.streaming = true

	m, _ = drainActions(t, m, ToolActivityEvent{
		ID:        "tc-1",
		ToolLabel: "Read",
		Summary:   "done",
		State:     ToolActivitySuccess,
	})

	if m.state.statusKind != StatusStreaming {
		t.Fatalf("status kind = %q, want streaming while run active", m.state.statusKind)
	}
}

func TestToolActivityRowRendersInTranscript(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, ToolActivityEvent{
		ID:        "tc-1",
		ToolLabel: "Grep",
		Summary:   `"SessionID" in internal/runtime`,
		State:     ToolActivityRunning,
	})

	in := m.toRenderInput()
	in.Width = 80
	out := Render(in)

	for _, want := range []string{"⚙ Grep", "SessionID", "internal/runtime"} {
		if !strings.Contains(out.Content, want) {
			t.Fatalf("rendered output missing %q:\n%s", want, out.Content)
		}
	}
}

func TestHostInfoRowRendersInTranscript(t *testing.T) {
	m := newWithFixedClock()
	m, _ = drainActions(t, m, HostInfoEvent{Text: "Available models:\n  claude-sonnet-4-6"})

	in := m.toRenderInput()
	in.Width = 80
	out := Render(in)

	if !strings.Contains(out.Content, "Available models:") {
		t.Fatalf("rendered output missing host-info text:\n%s", out.Content)
	}
	if !strings.Contains(out.Content, "claude-sonnet-4-6") {
		t.Fatalf("rendered output missing host-info detail line:\n%s", out.Content)
	}
	if !strings.Contains(out.Content, "│") {
		t.Fatalf("rendered output missing visible host-info boundary:\n%s", out.Content)
	}
}

func TestRenderChromeStatusVisibleAcrossKinds(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		kind       StatusKind
		mustNotBe  string
		shouldShow string
	}{
		{"ready", "Mode: build", StatusReady, "", "Mode: build"},
		{"streaming", "Streaming response", StatusStreaming, "", "Streaming response"},
		{"error", "model.list failed: timeout", StatusError, "", "model.list failed"},
		{"permission", "Permission required", StatusPermissionPending, "", "Permission required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := RenderInput{
				Width:      80,
				Status:     tc.status,
				StatusKind: tc.kind,
				InputView:  "",
			}
			out := Render(in)
			if !strings.Contains(out.Content, tc.shouldShow) {
				t.Fatalf("status %q (%s) not visible in render output:\n%s", tc.status, tc.kind, out.Content)
			}
		})
	}
}

func TestRenderChromeStatusEmptyCollapses(t *testing.T) {
	in := RenderInput{Width: 80}
	withStatus := in
	withStatus.Status = "Streaming response"
	withStatus.StatusKind = StatusStreaming

	emptyOut := Render(in).Content
	statusOut := Render(withStatus).Content

	if len(statusOut) <= len(emptyOut) {
		t.Fatalf("empty Status should produce shorter output than non-empty;\n  empty len=%d\n  status len=%d", len(emptyOut), len(statusOut))
	}
	if strings.Contains(emptyOut, "Streaming response") {
		t.Fatalf("empty Status leaked status text into render output:\n%s", emptyOut)
	}
}
