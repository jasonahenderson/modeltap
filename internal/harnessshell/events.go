package harnessshell

// Host-event intake for the reusable shell. Per WU-098 §"Shell receives",
// the shell applies host-driven state changes through typed events; no
// callback hooks, no untyped messages.
//
// Stage C-3 wires the run-lifecycle subset of events: SubmissionAccepted,
// SubmissionFailed, RunStarted, RunDelta, RunCompleted, RunStopped,
// RunFailed. Permission, preview, and host-status events land in
// subsequent Stage C commits; any unhandled event is silently ignored at
// this stage so partial-implementation does not panic the host loop.

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
// placeholder is correlated to the run. Per WU-098 the placeholder is
// inserted optimistically, so RunStarted carries no UX requirement
// beyond chrome state and label updates.
func (s *state) applyRunStarted(e RunStartedEvent) {
	s.streaming = true
	s.statusKind = StatusStreaming
	if e.RunID != "" {
		s.activeRunID = e.RunID
		s.assignRunIDToPlaceholder(e.SubmissionID, e.RunID)
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
			if item.Role == RoleAssistant && item.RunID == runID {
				return i
			}
		}
	}
	for i := len(s.transcriptItems) - 1; i >= 0; i-- {
		item := &s.transcriptItems[i]
		if item.Role == RoleAssistant && item.Streaming {
			return i
		}
	}
	return -1
}

