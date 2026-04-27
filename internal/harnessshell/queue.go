package harnessshell

// Queued submission lifecycle and merge rules. Stage C wires the
// queuedSubmissions / pendingSubmissions invariants from WU-098.

import (
	"fmt"
	"strings"
)

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
