// Portions ported from github.com/sst/opencode at commit 5e86c9b
// (parent of f68374a, 2025-10-31). Original source MIT-licensed,
// copyright (c) 2025 opencode. See NOTICE.

package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// NewStyle creates a new lipgloss.Style. This is a thin wrapper so
// theme-aware code can build styles consistently.
func NewStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}
