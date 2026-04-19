package harness

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// DefaultPasteThreshold is the byte length above which pasted content
// triggers a PasteDetectedMsg per Bundle 5 design D4.3.
const DefaultPasteThreshold = 2048

// HistorySource provides cross-session command history entries.
// Implemented by WU-092's BFF-sourced traversal; the input area
// degrades gracefully when no source is set.
type HistorySource interface {
	// Entry returns the content at the given history index, where 0
	// is the most recent. Returns ("", false) when index is past the
	// end of the source.
	Entry(index int) (string, bool)
	// Len returns the total number of available entries.
	Len() int
}

// InputArea wraps a bubbles/textarea with command detection, @file
// extraction, history traversal, and paste detection per design D4.
type InputArea struct {
	state *AppState
	ta    textarea.Model

	historySource HistorySource
	historyIndex  int
	savedDraft    string

	pasteThreshold int

	width int
}

// NewInputArea constructs an empty input area bound to shared state.
// The submitKey argument is reserved for future per-component
// remapping; today the App handles submit at the global key level.
func NewInputArea(state *AppState) InputArea {
	ta := textarea.New()
	ta.Placeholder = "Type a message... (/help for commands, @file to attach)"
	ta.Prompt = "> "
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.Focus()
	return InputArea{
		state:          state,
		ta:             ta,
		pasteThreshold: DefaultPasteThreshold,
	}
}

// SetWidth informs the textarea of the available width.
func (i *InputArea) SetWidth(w int) {
	i.width = w
	i.ta.SetWidth(w)
}

// Height returns the rendered height in rows.
func (i *InputArea) Height() int {
	h := i.ta.LineCount()
	if h < 1 {
		return 1
	}
	return h
}

// CursorAtTop reports whether the cursor sits on the textarea's first
// line — the App uses this to decide whether arrow-up should switch
// focus to the viewport rather than scroll history.
func (i *InputArea) CursorAtTop() bool {
	return i.ta.Line() == 0
}

// Value returns the raw textarea contents.
func (i *InputArea) Value() string { return i.ta.Value() }

// SetValue replaces the textarea contents (used by tests, command
// completion, and history traversal).
func (i *InputArea) SetValue(v string) { i.ta.SetValue(v) }

// ReplacePaste replaces the last occurrence of original in the
// textarea with replacement. Used by the paste-handler resolution
// flow (WU-083). No-op when original is empty or not found.
func (i *InputArea) ReplacePaste(original, replacement string) {
	if original == "" {
		return
	}
	current := i.ta.Value()
	idx := strings.LastIndex(current, original)
	if idx < 0 {
		return
	}
	i.ta.SetValue(current[:idx] + replacement + current[idx+len(original):])
}

// SetHistorySource plugs in a HistorySource. nil clears the source.
func (i *InputArea) SetHistorySource(h HistorySource) { i.historySource = h }

// SetPasteThreshold overrides the default 2048-byte threshold.
func (i *InputArea) SetPasteThreshold(n int) { i.pasteThreshold = n }

// Reset clears the textarea and resets history traversal state.
// Called after a successful submit.
func (i *InputArea) Reset() {
	i.ta.Reset()
	i.historyIndex = 0
	i.savedDraft = ""
}

// Update routes Bubbletea messages into the textarea, handling
// history traversal up/down at the boundaries and paste detection on
// any value-change.
func (i InputArea) Update(msg tea.Msg) (InputArea, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyUp:
			if i.CursorAtTop() {
				if cmd := i.handleHistoryUp(); cmd != nil {
					return i, cmd
				}
			}
		case tea.KeyDown:
			if i.historyIndex > 0 {
				if cmd := i.handleHistoryDown(); cmd != nil {
					return i, cmd
				}
			}
		}
	}

	prev := i.ta.Value()
	updated, cmd := i.ta.Update(msg)
	i.ta = updated
	if pasteCmd := i.detectPasteCmd(prev, i.ta.Value()); pasteCmd != nil {
		return i, tea.Batch(cmd, pasteCmd)
	}
	return i, cmd
}

// View renders the textarea.
func (i *InputArea) View() string {
	return i.ta.View()
}

// Submit produces a SubmitMsg from the current input contents and
// resets the textarea. Returns nil when the input is whitespace-only.
// The App calls this when the global submit key fires while focus is
// on the input.
func (i *InputArea) Submit() tea.Cmd {
	content := strings.TrimSpace(i.ta.Value())
	if content == "" {
		return nil
	}
	msg := SubmitMsg{Content: content}
	if strings.HasPrefix(content, "/") {
		msg.IsCommand = true
		parts := strings.SplitN(content[1:], " ", 2)
		msg.Command = parts[0]
		if len(parts) > 1 {
			msg.CommandArgs = parts[1]
		}
	}
	msg.Attachments = ExtractFileRefs(content)
	i.Reset()
	return func() tea.Msg { return msg }
}

// handleHistoryUp walks one step further back into history and
// returns a no-op cmd; nil if there's no source or no further entry.
func (i *InputArea) handleHistoryUp() tea.Cmd {
	if i.historySource == nil || i.historySource.Len() == 0 {
		return nil
	}
	if i.historyIndex == 0 {
		i.savedDraft = i.ta.Value()
	}
	if i.historyIndex >= i.historySource.Len() {
		return nil
	}
	entry, ok := i.historySource.Entry(i.historyIndex)
	if !ok {
		return nil
	}
	i.ta.SetValue(entry)
	i.historyIndex++
	return func() tea.Msg { return historyAdvancedMsg{} }
}

// handleHistoryDown walks one step toward the most-recent / current
// draft. Returns nil if not currently in history traversal.
func (i *InputArea) handleHistoryDown() tea.Cmd {
	if i.historyIndex <= 0 {
		return nil
	}
	i.historyIndex--
	if i.historyIndex == 0 {
		i.ta.SetValue(i.savedDraft)
		return func() tea.Msg { return historyAdvancedMsg{} }
	}
	entry, ok := i.historySource.Entry(i.historyIndex - 1)
	if ok {
		i.ta.SetValue(entry)
	}
	return func() tea.Msg { return historyAdvancedMsg{} }
}

// historyAdvancedMsg is an internal marker used by tests to verify
// that a history step actually fired. Not consumed by the App.
type historyAdvancedMsg struct{}

// detectPasteCmd compares pre and post values; if the new content is
// at least pasteThreshold bytes longer (a single Update tick rarely
// types that much), emit a PasteDetectedMsg.
func (i *InputArea) detectPasteCmd(prev, current string) tea.Cmd {
	if i.pasteThreshold <= 0 {
		return nil
	}
	delta := len(current) - len(prev)
	if delta < i.pasteThreshold {
		return nil
	}
	pasted := current
	if strings.HasPrefix(pasted, prev) {
		pasted = pasted[len(prev):]
	}
	lines := strings.Split(pasted, "\n")
	previewN := 5
	if len(lines) < previewN {
		previewN = len(lines)
	}
	preview := strings.Join(lines[:previewN], "\n")
	return func() tea.Msg {
		return PasteDetectedMsg{
			Content:   pasted,
			ByteSize:  len(pasted),
			LineCount: len(lines),
			Preview:   preview,
		}
	}
}

// fileRefRE matches @path tokens. The path is a non-whitespace,
// non-quote run that doesn't start with another @ — covers `@a/b.go`,
// `@./foo`, `@~/notes.md`, `@*.md` (glob).
var fileRefRE = regexp.MustCompile(`(?:^|\s)@([^\s@'"]+)`)

// ExtractFileRefs returns @file references found in content. The
// leading @ is stripped from each result.
func ExtractFileRefs(content string) []string {
	matches := fileRefRE.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 && m[1] != "" {
			out = append(out, m[1])
		}
	}
	return out
}

// DetectDragDrop returns a slice of file paths if content looks like
// a drag-and-drop payload (one or more absolute paths, separated by
// newlines or whitespace) and reports true. Heuristic per design D4.4.
func DetectDragDrop(content string) ([]string, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, false
	}
	tokens := strings.FieldsFunc(content, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t'
	})
	if len(tokens) == 0 {
		return nil, false
	}
	paths := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if !strings.HasPrefix(t, "/") && !strings.HasPrefix(t, "~") {
			return nil, false
		}
		paths = append(paths, t)
	}
	return paths, true
}
