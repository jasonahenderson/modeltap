package harnessshell

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Host-event intake for the reusable shell. Per WU-098 §"Shell receives",
// the shell applies host-driven state changes through typed events; no
// callback hooks, no untyped messages.
//
// Stage C-3 wires the run-lifecycle subset of events: SubmissionAccepted,
// SubmissionFailed, RunStarted, RunDelta, RunCompleted, RunStopped,
// RunFailed. Permission, preview, and host-status events land in
// subsequent Stage C commits; any unhandled event is silently ignored at
// this stage so partial-implementation does not panic the host loop.

// streamTickMsg is the 1Hz tick that drives the elapsed-seconds
// component of the streaming status line. The tick handler in
// Model.Update reschedules itself via streamTickCmd while
// state.streaming is true; once streaming ends, the in-flight tick
// arrives, finds streaming==false, and stops rescheduling. PATCH-0035.
type streamTickMsg time.Time

// streamTickCmd returns a tea.Cmd that fires a streamTickMsg one
// second from now.
func streamTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return streamTickMsg(t)
	})
}

// applyHostEvent dispatches a typed [HostEvent] into shell-owned state.
// Per WU-098, the shell mutates only shell-owned state in response.
func (s *state) applyHostEvent(evt HostEvent) {
	switch e := evt.(type) {
	case SubmissionAcceptedEvent:
		s.applySubmissionAccepted(e)
	case SubmissionFailedEvent:
		s.applySubmissionFailed(e)
	case RunStartedEvent:
		s.applyRunStarted(e)
	case RunDeltaEvent:
		s.applyRunDelta(e)
	case RunCompletedEvent:
		s.applyRunCompleted(e)
	case RunStoppedEvent:
		s.applyRunStopped(e)
	case RunFailedEvent:
		s.applyRunFailed(e)
	case PermissionRequestedEvent:
		s.applyPermissionRequested(e)
	case PermissionResolvedEvent:
		s.applyPermissionResolved(e)
	case PreviewLoadedEvent:
		s.applyPreviewLoaded(e)
	case HostStatusEvent:
		s.applyHostStatus(e)
	case HostInfoEvent:
		s.applyHostInfo(e)
	}
}

// applyPreviewLoaded paints the host-supplied preview payload into the
// shell-local preview dialog. The dialog shows immediately; Esc closes
// it (handled in Model.Update).
func (s *state) applyPreviewLoaded(e PreviewLoadedEvent) {
	title := e.Preview.Title
	if title == "" {
		title = e.Target.TokenID
	}
	s.preview = &PreviewDialog{Title: title, Content: e.Preview.Content}
	s.status = "Preview loaded"
}

// applyHostStatus applies a host-supplied status string and structured
// kind. The shell uses Kind (not the string) to drive chrome decisions
// (pulsing dot, interrupt-armed styling, permission-pending highlight).
func (s *state) applyHostStatus(e HostStatusEvent) {
	if e.Status != "" {
		s.status = e.Status
	}
	if e.Kind != "" {
		s.statusKind = e.Kind
	}
}

// applyHostInfo appends a host-supplied informational row to the
// transcript. Used for slash-command output that must persist in the
// visible transcript rather than flash through the chrome status line.
// Empty Text is a no-op so callers can guard at the emit site or here.
func (s *state) applyHostInfo(e HostInfoEvent) {
	if e.Text == "" {
		return
	}
	s.transcriptItems = append(s.transcriptItems, TranscriptItem{
		Kind: TranscriptItemKindHostInfo,
		Role: RoleHostInfo,
		Text: e.Text,
	})
}

// applyPermissionRequested appends a transcript event row for the request
// and registers a [PendingPermission] in shell-owned state. The composer
// permission UI then surfaces controls automatically via [Render].
func (s *state) applyPermissionRequested(e PermissionRequestedEvent) {
	if e.Request.ID == "" {
		return
	}
	transcriptID := "perm-" + e.Request.ID
	s.transcriptItems = append(s.transcriptItems, TranscriptItem{
		ID:    transcriptID,
		Kind:  TranscriptItemKindEvent,
		Role:  RoleEvent,
		Text:  e.Request.Summary,
		Event: &EventState{Status: "requested", RequestID: e.Request.ID},
	})
	s.pendingPermissions = append(s.pendingPermissions, PendingPermission{
		Request:        e.Request,
		TranscriptID:   transcriptID,
		SelectedAction: 0,
	})
	s.activePermissionIndex = len(s.pendingPermissions) - 1
	s.statusKind = StatusPermissionPending
	if e.Request.SessionPolicyState.SessionApproved {
		s.status = "Permission required (session policy active)"
	} else {
		s.status = "Permission required"
	}
}

// applyPermissionResolved updates the matching transcript event row with
// the host-reported outcome and removes the pending permission from
// shell-owned state. The composer permission controls disappear once the
// pending list is empty.
func (s *state) applyPermissionResolved(e PermissionResolvedEvent) {
	if e.RequestID == "" {
		return
	}
	for i := range s.transcriptItems {
		item := &s.transcriptItems[i]
		if item.Event != nil && item.Event.RequestID == e.RequestID {
			item.Event.Status = transcriptStatusForOutcome(e.Outcome)
			break
		}
	}
	s.removePendingPermissionByID(e.RequestID)
	if len(s.pendingPermissions) == 0 {
		if s.streaming {
			s.statusKind = StatusStreaming
			s.status = "Resuming run"
		} else {
			s.statusKind = StatusReady
			if e.Message != "" {
				s.status = e.Message
			} else {
				s.status = "Permission resolved"
			}
		}
	} else {
		s.statusKind = StatusPermissionPending
		s.status = "Permission resolved; more pending"
	}
}

// transcriptStatusForOutcome maps a host-reported [PermissionOutcome] to
// the transcript-event-row status string consumed by [Render].
func transcriptStatusForOutcome(o PermissionOutcome) string {
	switch o {
	case OutcomeApprovedOnce, OutcomeApprovedSession:
		return "granted"
	case OutcomeDenied:
		return "denied"
	default:
		return "done"
	}
}

// applySubmissionAccepted correlates the optimistically-rendered assistant
// placeholder with the host-assigned RunID. Per WU-098, the assistant row
// already exists from beginSubmission; this event only carries the
// correlation.
func (s *state) applySubmissionAccepted(e SubmissionAcceptedEvent) {
	if e.RunID == "" {
		return
	}
	s.activeRunID = e.RunID
	s.assignRunIDToPlaceholder(e.SubmissionID, e.RunID)
}

// applySubmissionFailed removes the optimistic assistant placeholder and
// surfaces failure text. The host did not accept the submission, so the
// placeholder is no longer correct.
func (s *state) applySubmissionFailed(e SubmissionFailedEvent) {
	s.removePlaceholderForSubmission(e.SubmissionID)
	s.streaming = false
	s.streamPulse = 0
	s.interruptArmed = false
	s.activeRunID = ""
	s.statusKind = StatusError
	if e.Message != "" {
		s.status = "Submission failed: " + e.Message
	} else {
		s.status = "Submission failed"
	}
}

// applyRunStarted ensures the streaming flag is set and the assistant
// placeholder is correlated to the run. For an attached/replayed run with no
// local submission placeholder, it creates a selected assistant replay row.
func (s *state) applyRunStarted(e RunStartedEvent) {
	// PATCH-0035: start the elapsed-seconds clock and schedule the
	// 1Hz tick that refreshes the status line. The tick reschedules
	// itself while streaming==true and naturally stops on terminal
	// events when runStartedAt is cleared. The cmd factory is
	// injectable so tests can replace tea.Tick with a no-op.
	if s.runStartedAt.IsZero() {
		s.runStartedAt = s.nowOrDefault()
		if tickFn := s.streamTick; tickFn != nil {
			if cmd := tickFn(); cmd != nil {
				s.pendingCmds = append(s.pendingCmds, cmd)
			}
		}
	}
	s.streaming = true
	s.statusKind = StatusStreaming
	if e.RunID != "" {
		s.activeRunID = e.RunID
		if e.SubmissionID != "" {
			s.assignRunIDToPlaceholder(e.SubmissionID, e.RunID)
		} else if s.assistantRowIndexForRun(e.RunID) < 0 {
			s.transcriptItems = append(s.transcriptItems, TranscriptItem{
				ID:        "run-" + e.RunID,
				Kind:      TranscriptItemKindMessage,
				Role:      RoleAssistant,
				RunID:     e.RunID,
				Streaming: true,
			})
		}
	}
	if e.Label != "" {
		s.label = e.Label
	}
	if s.status == "" {
		s.status = "Streaming response"
	}
}

// applyRunDelta appends streaming text to the active assistant row. The
// row is identified by RunID; if no row is correlated yet (mid-flight
// races), the delta is dropped to keep transcript invariants intact.
func (s *state) applyRunDelta(e RunDeltaEvent) {
	if e.Delta == "" {
		return
	}
	idx := s.assistantRowIndexForRun(e.RunID)
	if idx < 0 {
		return
	}
	s.transcriptItems[idx].Text += e.Delta
	s.transcriptItems[idx].Streaming = true
}

// applyRunCompleted clears streaming chrome, marks the assistant row
// non-streaming, and auto-releases queued follow-ups per the FEAT-0014
// invariant "queued work auto-releases only after normal completion".
func (s *state) applyRunCompleted(e RunCompletedEvent) {
	if idx := s.assistantRowIndexForRun(e.RunID); idx >= 0 {
		s.transcriptItems[idx].Streaming = false
	}
	s.streaming = false
	s.streamPulse = 0
	s.interruptArmed = false
	s.activeRunID = ""
	s.runStartedAt = time.Time{} // PATCH-0035
	s.statusKind = StatusReady
	s.status = "Done"

	if len(s.queuedSubmissions) > 0 || len(s.pendingSubmissions) > 0 {
		s.releaseQueuedSubmission()
	}
}

// applyRunStopped clears streaming chrome but does NOT auto-release the
// queue, per the FEAT-0014 invariant "stop does not auto-resume the
// stopped run". The reason and message drive the surfaced status text.
func (s *state) applyRunStopped(e RunStoppedEvent) {
	if idx := s.assistantRowIndexForRun(e.RunID); idx >= 0 {
		s.transcriptItems[idx].Streaming = false
	}
	s.streaming = false
	s.streamPulse = 0
	s.interruptArmed = false
	s.activeRunID = ""
	s.runStartedAt = time.Time{} // PATCH-0035
	s.statusKind = StatusReady
	switch {
	case e.Message != "":
		s.status = e.Message
	case e.Reason == StopReasonInterrupt:
		s.status = "Interrupted"
	default:
		s.status = "Stopped"
	}
}

// applyRunFailed marks the run terminal, clears streaming chrome, and
// surfaces failure text. Queue is not auto-released.
func (s *state) applyRunFailed(e RunFailedEvent) {
	if idx := s.assistantRowIndexForRun(e.RunID); idx >= 0 {
		s.transcriptItems[idx].Streaming = false
	}
	s.streaming = false
	s.streamPulse = 0
	s.interruptArmed = false
	s.activeRunID = ""
	s.runStartedAt = time.Time{} // PATCH-0035
	s.statusKind = StatusError
	if e.Message != "" {
		s.status = "Run failed: " + e.Message
	} else {
		s.status = "Run failed"
	}
}

// assignRunIDToPlaceholder finds the assistant placeholder for a given
// SubmissionID and sets its RunID. No-op if no matching placeholder is
// found (defensive — if the host issues correlation events out of order
// or for a lost submission the transcript stays internally consistent).
func (s *state) assignRunIDToPlaceholder(submissionID, runID string) {
	if submissionID == "" {
		return
	}
	for i := range s.transcriptItems {
		item := &s.transcriptItems[i]
		if item.Role == RoleAssistant && item.SubmissionID == submissionID {
			item.RunID = runID
			return
		}
	}
}

// removePlaceholderForSubmission removes both the user row and the
// assistant placeholder bound to a given SubmissionID (used on
// SubmissionFailedEvent — the optimistic rows must not survive a
// rejected submission).
func (s *state) removePlaceholderForSubmission(submissionID string) {
	if submissionID == "" {
		return
	}
	out := s.transcriptItems[:0]
	for _, item := range s.transcriptItems {
		if item.SubmissionID == submissionID {
			continue
		}
		out = append(out, item)
	}
	s.transcriptItems = out
}

// assistantRowIndexForRun returns the transcript index of the streaming
// assistant row associated with the given RunID, or -1 if not found. The
// search prefers rows whose RunID matches; as a fallback it accepts the
// last assistant row marked streaming so an early RunDelta that arrives
// before correlation does not silently drop output.
func (s *state) assistantRowIndexForRun(runID string) int {
	if runID != "" {
		for i := range s.transcriptItems {
			item := &s.transcriptItems[i]
			if item.Role == RoleAssistant && item.RunID == runID && item.Streaming {
				return i
			}
		}
		for i := len(s.transcriptItems) - 1; i >= 0; i-- {
			item := &s.transcriptItems[i]
			if item.Role == RoleAssistant && item.Streaming && item.RunID == "" {
				return i
			}
		}
		return -1
	}
	for i := len(s.transcriptItems) - 1; i >= 0; i-- {
		item := &s.transcriptItems[i]
		if item.Role == RoleAssistant && item.Streaming {
			return i
		}
	}
	return -1
}
