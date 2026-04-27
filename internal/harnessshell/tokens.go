package harnessshell

// Paste/file token normalization, summaries, selection, and inline expansion
// rules. Stage C moves token logic from the spike's App methods into shell-
// owned state on [Model].

import (
	"fmt"
	"strings"
)

// addToken appends a new token of the given kind to the composer's input
// tokens and selects it as the active one.
func (s *state) addToken(kind TokenKind, payload string) {
	n := len(s.inputTokens) + 1
	label := string(kind) + "-" + fmt.Sprintf("%d", n)
	if kind == TokenKindFile {
		label = detectFileLabel(payload, n)
	}
	s.inputTokens = append(s.inputTokens, InputToken{
		ID:      fmt.Sprintf("%s-%d", kind, n),
		Kind:    kind,
		Label:   label,
		Payload: payload,
	})
	s.selectedToken = len(s.inputTokens) - 1
}

// moveTokenSelection adjusts the active composer token by delta, clamping
// to the token list bounds, and updates the status to reflect the
// selection.
func (s *state) moveTokenSelection(delta int) {
	if len(s.inputTokens) == 0 {
		return
	}
	s.selectedToken += delta
	if s.selectedToken < 0 {
		s.selectedToken = 0
	}
	if s.selectedToken >= len(s.inputTokens) {
		s.selectedToken = len(s.inputTokens) - 1
	}
	s.status = "Selected " + s.inputTokens[s.selectedToken].Label
}

// normalizeDroppedPath inspects a textarea value to see if it represents a
// drag-and-drop path style payload. It returns the normalized path and true
// when the value looks like a single dropped path; otherwise returns "".
func normalizeDroppedPath(v string) (string, bool) {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return "", false
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) != 1 {
		return "", false
	}
	path := strings.TrimSpace(lines[0])
	if len(path) >= 2 {
		if (path[0] == '"' && path[len(path)-1] == '"') || (path[0] == '\'' && path[len(path)-1] == '\'') {
			path = path[1 : len(path)-1]
		}
	}
	path = strings.ReplaceAll(path, "\\ ", " ")
	path = strings.ReplaceAll(path, "\\(", "(")
	path = strings.ReplaceAll(path, "\\)", ")")
	path = strings.ReplaceAll(path, "\\[", "[")
	path = strings.ReplaceAll(path, "\\]", "]")
	if looksLikeDroppedPath(path) {
		return path, true
	}
	return "", false
}

// looksLikeDroppedPath returns true when the candidate string parses as a
// reasonable absolute-or-tilde path with at least one trailing segment.
func looksLikeDroppedPath(path string) bool {
	if path == "" || path == "/" || strings.HasPrefix(path, "//") {
		return false
	}
	if strings.HasPrefix(path, "~/") {
		return len(path) > 2
	}
	if !strings.HasPrefix(path, "/") {
		return false
	}
	rest := path[1:]
	if rest == "" {
		return false
	}
	if !strings.Contains(rest, "/") {
		return false
	}
	return true
}

// previewSelectedComposerToken opens a preview for the active composer
// token. Paste tokens are previewed locally because the shell owns the
// payload; file tokens emit a [LoadPreviewAction] so the host can
// supply the content via [PreviewLoadedEvent].
func (s *state) previewSelectedComposerToken() {
	if len(s.inputTokens) == 0 || s.selectedToken < 0 || s.selectedToken >= len(s.inputTokens) {
		return
	}
	tok := s.inputTokens[s.selectedToken]
	switch tok.Kind {
	case TokenKindPaste:
		s.preview = &PreviewDialog{Title: tok.Label, Content: tok.Payload}
		s.status = "Previewing " + tok.Label
	case TokenKindFile:
		s.pendingActions = append(s.pendingActions, LoadPreviewAction{
			Target: PreviewTarget{TokenID: tok.ID, Source: "composer"},
		})
		s.status = "Loading preview for " + tok.Label
	}
}

// previewSelectedTranscriptRef opens a preview for the currently
// selected transcript-token reference. Paste tokens render locally;
// file/reference tokens emit a [LoadPreviewAction].
func (s *state) previewSelectedTranscriptRef() {
	if len(s.transcriptRefs) == 0 || s.selectedTranscriptRef < 0 || s.selectedTranscriptRef >= len(s.transcriptRefs) {
		return
	}
	ref := s.transcriptRefs[s.selectedTranscriptRef]
	if ref.MessageIndex < 0 || ref.MessageIndex >= len(s.transcriptItems) {
		return
	}
	item := s.transcriptItems[ref.MessageIndex]
	if ref.TokenIndex < 0 || ref.TokenIndex >= len(item.Tokens) {
		return
	}
	tok := item.Tokens[ref.TokenIndex]
	switch tok.Kind {
	case TokenKindPaste:
		s.preview = &PreviewDialog{Title: tok.Label, Content: tok.Payload}
		s.status = "Previewing " + tok.Label
	case TokenKindFile:
		s.pendingActions = append(s.pendingActions, LoadPreviewAction{
			Target: PreviewTarget{
				TokenID:      tok.ID,
				Source:       "transcript",
				MessageIndex: ref.MessageIndex,
				TokenIndex:   ref.TokenIndex,
			},
		})
		s.status = "Loading preview for " + tok.Label
	}
}

// detectFileLabel chooses a display label for a file token based on the
// filename extension, distinguishing image from generic file references.
func detectFileLabel(path string, n int) string {
	name := path
	if idx := strings.LastIndexAny(path, "/\\"); idx >= 0 && idx+1 < len(path) {
		name = path[idx+1:]
	}
	if name == "" {
		return fmt.Sprintf("file-%d", n)
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".gif"), strings.HasSuffix(lower, ".webp"):
		return fmt.Sprintf("image-%d (%s)", n, name)
	default:
		return fmt.Sprintf("file-%d (%s)", n, name)
	}
}
