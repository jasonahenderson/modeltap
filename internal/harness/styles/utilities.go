// Portions ported from github.com/sst/opencode at commit 5e86c9b
// (parent of f68374a, 2025-10-31). Original source MIT-licensed,
// copyright (c) 2025 opencode. See NOTICE.

package styles

import (
	"image/color"

	"github.com/charmbracelet/lipgloss"
)

// IsNoColor reports whether c is lipgloss's no-color sentinel.
func IsNoColor(c color.Color) bool {
	_, ok := c.(lipgloss.NoColor)
	return ok
}

// Style wraps lipgloss.Style to provide a fluent API.
// Most methods delegate directly to the underlying style.
type Style struct {
	lipgloss.Style
}

// NewFluentStyle creates a new Style.
func NewFluentStyle() Style {
	return Style{lipgloss.NewStyle()}
}

// Foreground sets the foreground color.
func (s Style) Foreground(c lipgloss.TerminalColor) Style {
	return Style{s.Style.Foreground(c)}
}

// Background sets the background color.
func (s Style) Background(c lipgloss.TerminalColor) Style {
	return Style{s.Style.Background(c)}
}

// Bold enables or disables bold rendering.
func (s Style) Bold(v bool) Style {
	return Style{s.Style.Bold(v)}
}

// Italic enables or disables italic rendering.
func (s Style) Italic(v bool) Style {
	return Style{s.Style.Italic(v)}
}

// Faint enables or disables faint rendering.
func (s Style) Faint(v bool) Style {
	return Style{s.Style.Faint(v)}
}

// Width sets the width.
func (s Style) Width(i int) Style {
	return Style{s.Style.Width(i)}
}

// Height sets the height.
func (s Style) Height(i int) Style {
	return Style{s.Style.Height(i)}
}

// Padding sets padding on all sides.
func (s Style) Padding(i ...int) Style {
	return Style{s.Style.Padding(i...)}
}

// Margin sets margin on all sides.
func (s Style) Margin(i ...int) Style {
	return Style{s.Style.Margin(i...)}
}

// Border sets the border.
func (s Style) Border(b lipgloss.Border, sides ...bool) Style {
	return Style{s.Style.Border(b, sides...)}
}

// BorderStyle sets the border style.
func (s Style) BorderStyle(b lipgloss.Border) Style {
	return Style{s.Style.BorderStyle(b)}
}

// BorderForeground sets the border foreground color.
func (s Style) BorderForeground(c lipgloss.TerminalColor) Style {
	return Style{s.Style.BorderForeground(c)}
}

// Align sets alignment.
func (s Style) Align(p ...lipgloss.Position) Style {
	return Style{s.Style.Align(p...)}
}

// Inline sets inline mode.
func (s Style) Inline(v bool) Style {
	return Style{s.Style.Inline(v)}
}

// MaxWidth sets the max width.
func (s Style) MaxWidth(n int) Style {
	return Style{s.Style.MaxWidth(n)}
}

// Render applies the style to a string.
func (s Style) Render(str string) string {
	return s.Style.Render(str)
}
