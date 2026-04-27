package harnessshell

// Shell-local key handling and command-history behavior. Stage C moves the
// composer history, paste/file detection, and input-mutation hooks into
// shell-owned state.

import "strings"

// pushHistory records a submission in the command history (deduplicating
// consecutive duplicates) and resets the in-progress draft pointer.
func (s *state) pushHistory(content string) {
	s.historyIndex = -1
	s.historyDraft = ""
	if content == "" {
		return
	}
	if n := len(s.commandHistory); n > 0 && s.commandHistory[n-1] == content {
		return
	}
	s.commandHistory = append(s.commandHistory, content)
}

// recallPreviousCommand walks the command history one step back, capturing
// the in-progress draft on the first walk so it can be restored later.
func (s *state) recallPreviousCommand() {
	if len(s.commandHistory) == 0 {
		return
	}
	if s.historyIndex == -1 {
		s.historyDraft = s.input.Value()
		s.historyIndex = len(s.commandHistory) - 1
	} else if s.historyIndex > 0 {
		s.historyIndex--
	} else {
		return
	}
	s.input.SetValue(s.commandHistory[s.historyIndex])
	s.input.CursorEnd()
	s.syncInputHeight()
}

// recallNextCommand walks the command history one step forward, restoring
// the in-progress draft when the user steps past the end.
func (s *state) recallNextCommand() {
	if s.historyIndex == -1 {
		return
	}
	if s.historyIndex < len(s.commandHistory)-1 {
		s.historyIndex++
		s.input.SetValue(s.commandHistory[s.historyIndex])
	} else {
		s.historyIndex = -1
		s.input.SetValue(s.historyDraft)
		s.historyDraft = ""
	}
	s.input.CursorEnd()
	s.syncInputHeight()
}

// syncInputHeight resizes the textarea to fit the current line count
// (single-line by default, growing as the buffer adds newlines).
func (s *state) syncInputHeight() {
	lines := strings.Count(s.input.Value(), "\n") + 1
	if lines < 1 {
		lines = 1
	}
	s.input.SetHeight(lines)
}

// handleInputMutation watches for paste-large-buffer and dropped-path
// patterns when the textarea contents change, capturing them as composer
// tokens and clearing the buffer when matched.
func (s *state) handleInputMutation(prev, current string) {
	if current == prev {
		return
	}
	if normalizedPath, ok := normalizeDroppedPath(current); ok {
		s.addToken(TokenKindFile, normalizedPath)
		s.input.Reset()
		s.status = "Dropped path captured as file entry"
		return
	}
	delta := len(current) - len(prev)
	if delta >= 120 {
		trimmed := strings.TrimSpace(current)
		if trimmed != "" {
			s.addToken(TokenKindPaste, trimmed)
			s.input.Reset()
			s.status = "Large paste captured as compact entry"
			return
		}
	}
}

// nextFocus returns the next focus zone in the Tab cycle, skipping the
// sidebar zone when the sidebar is closed.
func nextFocus(current FocusZone, sidebarOpen bool) FocusZone {
	switch current {
	case FocusInput:
		return FocusTranscript
	case FocusTranscript:
		if !sidebarOpen {
			return FocusInput
		}
		return FocusSidebar
	default:
		return FocusInput
	}
}

// moveTranscriptRef shifts the active transcript-token selection by delta,
// clamping to the available refs and updating the status to reflect the
// new selection.
func (s *state) moveTranscriptRef(delta int) {
	if len(s.transcriptRefs) == 0 {
		return
	}
	s.selectedTranscriptRef += delta
	if s.selectedTranscriptRef < 0 {
		s.selectedTranscriptRef = 0
	}
	if s.selectedTranscriptRef >= len(s.transcriptRefs) {
		s.selectedTranscriptRef = len(s.transcriptRefs) - 1
	}
	ref := s.transcriptRefs[s.selectedTranscriptRef]
	if ref.MessageIndex < len(s.transcriptItems) {
		item := s.transcriptItems[ref.MessageIndex]
		if ref.TokenIndex < len(item.Tokens) {
			s.status = "Transcript item: " + item.Tokens[ref.TokenIndex].Label
		}
	}
}
