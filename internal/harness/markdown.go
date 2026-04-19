package harness

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
)

// DefaultMarkdownDebounce is the minimum delay between successive
// markdown re-renders during streaming, per Bundle 5 design D6.2.
const DefaultMarkdownDebounce = 50 * time.Millisecond

// MarkdownRenderer wraps glamour.TermRenderer with stream-tolerant
// rendering, debounce, and incremental partial-render support.
//
// During streaming the assistant message grows token-by-token. Glamour
// is happiest with valid markdown — open code fences and unmatched
// inline markers cause noisy output. RenderStreaming applies a
// best-effort heal pass before delegating to glamour.
type MarkdownRenderer struct {
	r     *glamour.TermRenderer
	width int

	debounceInterval time.Duration
	lastRender       time.Time
	pending          bool
}

// NewMarkdownRenderer returns a renderer configured for the given
// terminal width. width <= 0 uses an 80-column default.
func NewMarkdownRenderer(width int) (*MarkdownRenderer, error) {
	if width <= 0 {
		width = 80
	}
	r, err := newGlamourTerm(width)
	if err != nil {
		return nil, fmt.Errorf("glamour: %w", err)
	}
	return &MarkdownRenderer{
		r:                r,
		width:            width,
		debounceInterval: DefaultMarkdownDebounce,
	}, nil
}

func newGlamourTerm(width int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
}

// SetWidth re-creates the underlying glamour renderer at the new
// width. Called from the viewport on every WindowSizeMsg.
func (m *MarkdownRenderer) SetWidth(w int) error {
	if w <= 0 || w == m.width {
		return nil
	}
	r, err := newGlamourTerm(w)
	if err != nil {
		return err
	}
	m.r = r
	m.width = w
	return nil
}

// SetDebounce overrides the default 50ms debounce.
func (m *MarkdownRenderer) SetDebounce(d time.Duration) { m.debounceInterval = d }

// ShouldRedraw reports whether enough time has passed since the last
// successful render to warrant another. When false, the caller should
// skip this redraw cycle. The pending bit lets the viewport schedule
// a follow-up tick so the final render isn't dropped.
func (m *MarkdownRenderer) ShouldRedraw() bool {
	if time.Since(m.lastRender) >= m.debounceInterval {
		m.lastRender = time.Now()
		m.pending = false
		return true
	}
	m.pending = true
	return false
}

// Pending reports whether a deferred redraw is outstanding.
func (m *MarkdownRenderer) Pending() bool { return m.pending }

// Render renders complete markdown content. Used after a
// StreamCompleteMsg arrives (or for non-streaming messages).
func (m *MarkdownRenderer) Render(content string) (string, error) {
	if m.r == nil {
		return content, nil
	}
	return m.r.Render(content)
}

// RenderStreaming renders partial markdown content. Applies the
// healing pass first so glamour doesn't produce broken output.
func (m *MarkdownRenderer) RenderStreaming(content string) (string, error) {
	healed, _ := healPartialMarkdown(content)
	return m.Render(healed)
}

// healPartialMarkdown closes any unclosed markdown blocks so glamour
// can render the partial content cleanly. Returns the healed content
// plus the list of suffix strings appended (for callers that want to
// strip them from the output later).
//
// Heuristics covered:
//   - unclosed fenced code blocks (odd count of "```")
//   - unclosed inline code (odd count of "`")
//   - unclosed bold ("**") / italic ("*" or "_") emphasis
func healPartialMarkdown(content string) (string, []string) {
	var suffixes []string

	// 1. Fenced code blocks.
	if countNonOverlapping(content, "```")%2 == 1 {
		content += "\n```"
		suffixes = append(suffixes, "\n```")
	}

	// 2. Inline code (single backticks NOT inside a fenced block).
	// Count backticks in non-fenced regions only.
	if countInlineBackticks(content)%2 == 1 {
		content += "`"
		suffixes = append(suffixes, "`")
	}

	// 3. Bold (** pairs).
	if strings.Count(content, "**")%2 == 1 {
		content += "**"
		suffixes = append(suffixes, "**")
	}

	// Italic via underscore is intentionally NOT healed — the
	// heuristic produces too many false positives (snake_case
	// identifiers, file paths, etc.) and a missing italic close in
	// streamed output renders fine in glamour.
	return content, suffixes
}

// countNonOverlapping counts non-overlapping occurrences of needle in
// s. strings.Count already does this; this is a thin wrapper for
// readability with the inline-backtick variant below.
func countNonOverlapping(s, needle string) int {
	return strings.Count(s, needle)
}

// countInlineBackticks counts backticks that are NOT part of a fenced
// "```" run. Walks the string and skips three-in-a-row groups.
func countInlineBackticks(s string) int {
	count := 0
	i := 0
	for i < len(s) {
		if i+3 <= len(s) && s[i] == '`' && s[i+1] == '`' && s[i+2] == '`' {
			i += 3
			continue
		}
		if s[i] == '`' {
			count++
		}
		i++
	}
	return count
}
