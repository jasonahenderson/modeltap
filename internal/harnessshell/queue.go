package harnessshell

// Queued submission lifecycle, merge rules, and submit-action emission.
// Stage C wires the queuedSubmissions / pendingSubmissions invariants from
// WU-098 and the SubmitTurnAction emission contract (including the
// optimistic transcript rendering required by WU-098 §"Optimistic
// transcript rendering").

import (
	"fmt"
	"strings"
	"time"
)

// shellNativeClearCommand is the buffer text the shell handles locally
// without crossing the boundary as an action. Per WU-100 §"Definite scope
// rule for the reusable package", /clear is shell-native; host-native
// commands cross via [RunHostCommandAction] (added in a later commit).
const shellNativeClearCommand = "/clear"

func isShellNativeQuitCommand(content string) bool {
	switch strings.TrimSpace(content) {
	case "/quit", "/exit":
		return true
	default:
		return false
	}
}

// isShellNativeSelectCommand reports whether content is the
// /select shell-native command (PATCH-0030). /select toggles mouse
// capture so the user can let the terminal handle native click-drag
// text selection (e.g., to copy a run id from /runs output).
func isShellNativeSelectCommand(content string) bool {
	return strings.TrimSpace(content) == "/select"
}

// enqueueSubmission appends a queued follow-up entry to the visible queue.
// Per WU-098 queue invariants, the visible queue is FIFO and merges happen
// only at release time.
func (s *state) enqueueSubmission(content string, tokens []InputToken) {
	var queuedTokens []InputToken
	if len(tokens) > 0 {
		queuedTokens = append(queuedTokens, tokens...)
	}
	entries := []string{content}
	if strings.TrimSpace(content) == "" {
		entries = nil
	}
	s.queuedSubmissions = append(s.queuedSubmissions, QueuedSubmission{
		ID:      s.nextSubmissionID(),
		Text:    content,
		Entries: entries,
		Tokens:  queuedTokens,
	})
	s.status = fmt.Sprintf("Queued follow-up message (%d waiting)", len(s.queuedSubmissions))
}

// drainQueueIntoMerged promotes queuedSubmissions into pendingSubmissions
// (if not already there) and returns the merged QueuedSubmission ready to
// be turned into a SubmitTurnAction. The caller is responsible for
// consuming the returned merged submission and resetting both buffers.
func (s *state) drainQueueIntoMerged() (QueuedSubmission, bool) {
	if len(s.pendingSubmissions) == 0 && len(s.queuedSubmissions) > 0 {
		s.pendingSubmissions = append(s.pendingSubmissions, s.queuedSubmissions...)
		s.queuedSubmissions = nil
	}
	if len(s.pendingSubmissions) == 0 {
		return QueuedSubmission{}, false
	}
	merged := mergeQueuedSubmissions(s.pendingSubmissions)
	s.pendingSubmissions = nil
	return merged, true
}

// mergeQueuedSubmissions concatenates a sequence of queued submissions into
// a single merged submission preserving FIFO order across both Text and
// Entries.
func mergeQueuedSubmissions(items []QueuedSubmission) QueuedSubmission {
	var merged QueuedSubmission
	if len(items) == 0 {
		return merged
	}
	var parts []string
	for _, item := range items {
		if strings.TrimSpace(item.Text) != "" {
			parts = append(parts, strings.TrimSpace(item.Text))
		}
		if len(item.Entries) > 0 {
			merged.Entries = append(merged.Entries, item.Entries...)
		} else if strings.TrimSpace(item.Text) != "" {
			merged.Entries = append(merged.Entries, strings.TrimSpace(item.Text))
		}
		if len(item.Tokens) > 0 {
			merged.Tokens = append(merged.Tokens, item.Tokens...)
		}
	}
	merged.Text = strings.Join(parts, "\n\n")
	return merged
}

// nextSubmissionID returns a stable shell-generated submission identifier.
func (s *state) nextSubmissionID() string {
	s.submissionCounter++
	return fmt.Sprintf("sub-%d", s.submissionCounter)
}

// emitSubmitOnEnter routes the Enter key for the composer into the
// shell-owned submit pipeline. Returns true if the key was consumed by
// shell-native logic (submit, queue follow-up, queue release, /clear); a
// false return means Enter should be forwarded to the textarea (for
// instance an empty Enter while idle with no queued work).
//
// The pipeline matches the FEAT-0014 invariants:
//   - empty Enter while idle with non-empty queue releases queued work
//   - non-empty Enter while streaming or with queued work enqueues a
//     follow-up; if idle, the queue then auto-releases
//   - the shell-native /clear command resets the transcript without
//     crossing the boundary
//   - any other non-empty submit emits SubmitTurnAction with optimistic
//     user + assistant placeholder transcript rows
func (s *state) emitSubmitOnEnter() bool {
	content := strings.TrimSpace(s.input.Value())

	if content == "" && len(s.inputTokens) == 0 {
		if s.currentPendingPermission() != nil {
			s.resolveActivePermission(permissionDecisionFromAction(s.pendingPermissions[s.activePermissionIndex].SelectedAction))
			return true
		}
		if !s.streaming && (len(s.queuedSubmissions) > 0 || len(s.pendingSubmissions) > 0) {
			s.status = "Releasing queued follow-up"
			s.statusKind = StatusReady
			s.releaseQueuedSubmission()
			return true
		}
		return false
	}

	s.pushHistory(content)

	if s.streaming || len(s.queuedSubmissions) > 0 || len(s.pendingSubmissions) > 0 {
		s.enqueueSubmission(content, s.inputTokens)
		s.input.Reset()
		s.syncInputHeight()
		s.inputTokens = nil
		s.selectedToken = -1
		if !s.streaming {
			s.releaseQueuedSubmission()
		}
		return true
	}

	if content == shellNativeClearCommand && len(s.inputTokens) == 0 {
		s.transcriptItems = nil
		s.transcriptRefs = nil
		s.selectedTranscriptRef = -1
		s.input.Reset()
		s.syncInputHeight()
		s.status = "Transcript cleared"
		s.statusKind = StatusReady
		return true
	}

	// PATCH-0023: dispatch host-native slash commands. /quit and
	// /exit are intercepted earlier in model.go; /clear is handled
	// just above. Any other leading-slash input with no attached
	// tokens is a host-native command. The shell does not pre-validate
	// the name; the host runtime's DispatchCommand emits an "Unknown
	// command" status for anything it does not recognize.
	if strings.HasPrefix(content, "/") && len(content) > 1 && len(s.inputTokens) == 0 {
		s.dispatchHostCommand(content)
		return true
	}

	var submittedTokens []InputToken
	if len(s.inputTokens) > 0 {
		submittedTokens = append(submittedTokens, s.inputTokens...)
	}
	s.beginSubmission(content, nil, submittedTokens, SubmissionSourceDirect)
	return true
}

// dispatchHostCommand emits a [RunHostCommandAction] for a host-native
// slash command. The input is the full "/name [args]" string with the
// leading slash already verified by the caller. Per PATCH-0023, this
// is the missing piece described in queue.go's pre-existing comment
// ("host-native commands cross via [RunHostCommandAction] (added in a
// later commit)") that was never added.
func (s *state) dispatchHostCommand(content string) {
	raw := content
	trimmed := strings.TrimPrefix(content, "/")
	name := trimmed
	args := ""
	if idx := strings.IndexAny(trimmed, " \t"); idx >= 0 {
		name = trimmed[:idx]
		args = strings.TrimSpace(trimmed[idx+1:])
	}

	s.input.Reset()
	s.syncInputHeight()
	s.inputTokens = nil
	s.selectedToken = -1
	s.status = "Running /" + name
	s.statusKind = StatusReady

	s.pendingActions = append(s.pendingActions, RunHostCommandAction{
		Invocation: CommandInvocation{
			Name: name,
			Args: args,
			Raw:  raw,
		},
	})
}

// releaseQueuedSubmission promotes queued work into a single merged
// submission and emits SubmitTurnAction with Source=queue_release. No-op
// when both queue buffers are empty.
func (s *state) releaseQueuedSubmission() {
	merged, ok := s.drainQueueIntoMerged()
	if !ok {
		return
	}
	s.beginSubmission(merged.Text, merged.Entries, merged.Tokens, SubmissionSourceQueueRelease)
}

// beginSubmission appends the optimistic user and assistant placeholder
// transcript rows, resets composer state, and queues a SubmitTurnAction
// for the host. Per WU-098 §"Optimistic transcript rendering" the user
// must never see a state in which the user row exists without an
// accompanying assistant placeholder row.
func (s *state) beginSubmission(content string, entries []string, tokens []InputToken, source SubmissionSource) {
	submissionID := s.nextSubmissionID()

	expanded := map[string]bool{}
	for _, tok := range tokens {
		if tok.Kind == TokenKindPaste {
			expanded[tok.ID] = true
		}
	}

	s.transcriptItems = append(s.transcriptItems,
		TranscriptItem{
			ID:           "msg-user-" + submissionID,
			Kind:         TranscriptItemKindMessage,
			Role:         RoleUser,
			Text:         strings.TrimSpace(content),
			Entries:      entries,
			Tokens:       tokens,
			Expanded:     expanded,
			SubmissionID: submissionID,
		},
		TranscriptItem{
			ID:           "msg-assistant-" + submissionID,
			Kind:         TranscriptItemKindMessage,
			Role:         RoleAssistant,
			Streaming:    true,
			SubmissionID: submissionID,
		},
	)

	s.input.Reset()
	s.syncInputHeight()
	s.inputTokens = nil
	s.selectedToken = -1
	s.status = "Submitted"
	s.statusKind = StatusStreaming
	s.streaming = true
	s.streamPulse = 0
	s.interruptArmed = false

	s.pendingActions = append(s.pendingActions, SubmitTurnAction{
		Submission: Submission{
			ID:          submissionID,
			Entries:     entries,
			Text:        strings.TrimSpace(content),
			Tokens:      tokens,
			Source:      source,
			RequestedAt: s.nowOrDefault(),
		},
	})
}

// nowOrDefault returns the time source, defaulting to time.Now when
// unset. The injectable clock keeps tests deterministic without
// committing to a specific clock implementation in the public API.
func (s *state) nowOrDefault() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}
