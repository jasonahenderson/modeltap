package harnessshell

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// WU-102 Layer 1 parity tests for queue invariants. Maps to FEAT-0014
// success criterion 4 ("queue follow-ups while streaming, release on
// idle Enter, queue survives interrupt"), the WU-098 queue invariants
// (visible queuedSubmissions FIFO + transient pendingSubmissions
// merge buffer), and the WU-102 §"Required Parity Coverage Areas /
// Queue behavior" assertions.

// submitWith drives a direct submit by setting the input value and
// pressing Enter. Returns the updated model and emitted actions.
func submitWith(t *testing.T, m Model, text string) (Model, []Action) {
	t.Helper()
	m.state.input.SetValue(text)
	return drainActions(t, m, enterKey())
}

func TestQueueMultiItemReleasePreservesFIFO(t *testing.T) {
	// FEAT-0014 SC4: queue follow-ups while streaming, then release
	// after completion. Verify FIFO order is preserved across the
	// pendingSubmissions merge buffer.
	m := newWithFixedClock()

	// First submit kicks off a streaming run.
	m, firstActions := submitWith(t, m, "first")
	firstSub := firstActions[0].(SubmitTurnAction).Submission

	// While streaming, enqueue three follow-ups.
	m, _ = submitWith(t, m, "second")
	m, _ = submitWith(t, m, "third")
	m, _ = submitWith(t, m, "fourth")
	if len(m.state.queuedSubmissions) != 3 {
		t.Fatalf("expected 3 queued, got %d", len(m.state.queuedSubmissions))
	}

	// Correlate first run, then complete it. Auto-release fires.
	m, _ = drainActions(t, m, SubmissionAcceptedEvent{SubmissionID: firstSub.ID, RunID: "run-1"})
	m, releaseActions := drainActions(t, m, RunCompletedEvent{RunID: "run-1"})

	if len(releaseActions) != 1 {
		t.Fatalf("expected 1 auto-release SubmitTurnAction, got %d", len(releaseActions))
	}
	released := releaseActions[0].(SubmitTurnAction).Submission
	if released.Source != SubmissionSourceQueueRelease {
		t.Fatalf("source = %v, want SubmissionSourceQueueRelease", released.Source)
	}

	// FIFO: merged Entries should preserve "second", "third", "fourth"
	// in submission order.
	if len(released.Entries) != 3 {
		t.Fatalf("expected 3 merged entries, got %d (%+v)", len(released.Entries), released.Entries)
	}
	if released.Entries[0] != "second" || released.Entries[1] != "third" || released.Entries[2] != "fourth" {
		t.Fatalf("FIFO order not preserved: %+v", released.Entries)
	}
}

func TestQueueSurvivesInterruptThenIdleEnterReleases(t *testing.T) {
	// FEAT-0014 SC4 verbatim end-to-end: stream a turn, queue a
	// follow-up during streaming, interrupt the stream, then idle
	// empty-Enter releases the queued follow-up.
	m := newWithFixedClock()
	m, firstActions := submitWith(t, m, "first")
	firstSub := firstActions[0].(SubmitTurnAction).Submission

	m, _ = submitWith(t, m, "queued-followup")
	if len(m.state.queuedSubmissions) != 1 {
		t.Fatalf("expected 1 queued, got %d", len(m.state.queuedSubmissions))
	}

	// Correlate then interrupt: arm + emit + RunStopped.
	m, _ = drainActions(t, m, SubmissionAcceptedEvent{SubmissionID: firstSub.ID, RunID: "run-1"})
	m, _ = drainActions(t, m, tea.KeyMsg{Type: tea.KeyEsc}) // arm
	m, _ = drainActions(t, m, tea.KeyMsg{Type: tea.KeyEsc}) // emit InterruptRunAction
	m, _ = drainActions(t, m, RunStoppedEvent{RunID: "run-1", Reason: StopReasonInterrupt})

	if len(m.state.queuedSubmissions) != 1 {
		t.Fatalf("queue must NOT auto-release after interrupt; got %d", len(m.state.queuedSubmissions))
	}
	if m.state.streaming {
		t.Fatalf("streaming should clear after RunStopped")
	}

	// Idle empty-Enter now releases the queued work.
	m, releaseActions := drainActions(t, m, enterKey())
	if len(releaseActions) != 1 {
		t.Fatalf("idle empty-Enter should release queued work; got %d actions", len(releaseActions))
	}
	released := releaseActions[0].(SubmitTurnAction).Submission
	if released.Source != SubmissionSourceQueueRelease {
		t.Fatalf("source = %v, want SubmissionSourceQueueRelease", released.Source)
	}
	if released.Text != "queued-followup" {
		t.Fatalf("released text = %q, want %q", released.Text, "queued-followup")
	}
}

func TestQueuedSubmissionPreservesTextAndTokens(t *testing.T) {
	// Submitting while streaming with tokens enqueues both text and
	// tokens; tokens preserve their kind/payload through the queue.
	m := newWithFixedClock()
	m, _ = submitWith(t, m, "first") // streaming starts

	m.state.inputTokens = []InputToken{
		{ID: "paste-1", Kind: TokenKindPaste, Label: "paste-1", Payload: "captured paste"},
	}
	m.state.input.SetValue("with attachment")
	m, _ = drainActions(t, m, enterKey())

	if len(m.state.queuedSubmissions) != 1 {
		t.Fatalf("expected 1 queued, got %d", len(m.state.queuedSubmissions))
	}
	q := m.state.queuedSubmissions[0]
	if q.Text != "with attachment" {
		t.Fatalf("queued text = %q", q.Text)
	}
	if len(q.Tokens) != 1 || q.Tokens[0].Payload != "captured paste" {
		t.Fatalf("tokens not preserved through queue: %+v", q.Tokens)
	}
}
