package harness

import (
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// defaultPasteTruncateLimit caps the content kept by the truncate
// strategy. Chosen to match the input-area paste threshold — by the
// time a paste is this big the model benefits from an explicit
// summarize / truncate / cancel decision, not more raw text.
const defaultPasteTruncateLimit = 2048

// PasteHandler drives the modal overlay that appears when the input
// area detects a large paste. The handler is a pure state machine —
// the App routes PasteDetectedMsg and keystrokes into it and renders
// the returned tea.Cmd (banner + resolution messages).
//
// States:
//
//   - idle          → no pending paste
//   - awaiting      → banner up, waiting on one of s/f/t/c/esc
//   - summarizing   → user picked [s]; handler is waiting for a
//     PasteResolvedMsg{Strategy=PasteStrategySummarize} from the
//     connection manager (content.transform round-trip) or a
//     Complete() call. Stays active so the user can't accidentally
//     dismiss the overlay mid-flight.
type PasteHandler struct {
	mu             sync.Mutex
	active         bool
	awaitingSum    bool
	pending        PasteDetectedMsg
	truncateLimit  int
}

// NewPasteHandler returns an inactive handler.
func NewPasteHandler() *PasteHandler {
	return &PasteHandler{truncateLimit: defaultPasteTruncateLimit}
}

// SetTruncateLimit overrides the byte cap for the truncate strategy.
func (h *PasteHandler) SetTruncateLimit(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.truncateLimit = n
}

// Active reports whether a paste decision is pending.
func (h *PasteHandler) Active() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.active
}

// HandlePaste captures the paste and returns a BannerMsg describing
// the pending choice. The App should also track that the handler is
// active so subsequent keystrokes route here instead of the input
// area.
func (h *PasteHandler) HandlePaste(msg PasteDetectedMsg) tea.Cmd {
	h.mu.Lock()
	h.active = true
	h.awaitingSum = false
	h.pending = msg
	h.mu.Unlock()

	banner := buildPasteBanner(msg)
	return func() tea.Msg {
		return BannerMsg{Text: banner, Duration: 0}
	}
}

// HandleKey processes one keystroke while the handler is active.
// Returns a tea.Cmd that emits either a PasteResolvedMsg (full /
// truncate / cancel), a PasteSummarizeRequestMsg (summarize — kicks
// off the BFF round-trip), or nil for ignored keys.
func (h *PasteHandler) HandleKey(k tea.KeyMsg) tea.Cmd {
	h.mu.Lock()
	if !h.active || h.awaitingSum {
		h.mu.Unlock()
		return nil
	}
	pending := h.pending

	if k.Type == tea.KeyEsc {
		h.active = false
		h.mu.Unlock()
		return func() tea.Msg {
			return PasteResolvedMsg{Strategy: PasteStrategyCancel, Original: pending.Content}
		}
	}

	if k.Type != tea.KeyRunes || len(k.Runes) != 1 {
		h.mu.Unlock()
		return nil
	}
	choice := k.Runes[0]

	switch choice {
	case 's', 'S':
		h.awaitingSum = true
		h.mu.Unlock()
		return func() tea.Msg { return PasteSummarizeRequestMsg{Content: pending.Content} }
	case 'f', 'F':
		h.active = false
		h.mu.Unlock()
		return func() tea.Msg {
			return PasteResolvedMsg{Strategy: PasteStrategyFull, Content: pending.Content, Original: pending.Content}
		}
	case 't', 'T':
		h.active = false
		limit := h.truncateLimit
		h.mu.Unlock()
		trimmed := pending.Content
		if len(trimmed) > limit {
			trimmed = trimmed[:limit]
		}
		return func() tea.Msg {
			return PasteResolvedMsg{Strategy: PasteStrategyTruncate, Content: trimmed, Original: pending.Content}
		}
	case 'c', 'C':
		h.active = false
		h.mu.Unlock()
		return func() tea.Msg { return PasteResolvedMsg{Strategy: PasteStrategyCancel, Original: pending.Content} }
	}

	h.mu.Unlock()
	return nil
}

// Complete clears the handler. The connection manager calls this
// after the content.transform round-trip lands and it has injected a
// PasteResolvedMsg with the summarized content.
func (h *PasteHandler) Complete() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = false
	h.awaitingSum = false
}

// buildPasteBanner composes the human-readable banner shown to the
// user while a large paste is pending. Preview lines are indented two
// spaces so they visually nest under the banner body.
func buildPasteBanner(p PasteDetectedMsg) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Large paste detected (%d bytes, %d lines)\n", p.ByteSize, p.LineCount)
	if p.Preview != "" {
		for _, line := range strings.Split(p.Preview, "\n") {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	b.WriteString("Choose: [s]ummarize  [f]ull  [t]runcate  [c]ancel")
	return b.String()
}
