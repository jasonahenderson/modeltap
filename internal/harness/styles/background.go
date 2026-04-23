// Portions ported from github.com/sst/opencode at commit 5e86c9b
// (parent of f68374a, 2025-10-31). Original source MIT-licensed,
// copyright (c) 2025 opencode. See NOTICE.

package styles

import "image/color"

// TerminalInfo holds detected terminal background information.
type TerminalInfo struct {
	Background       color.Color
	BackgroundIsDark bool
}

// Terminal is the globally accessible terminal background state.
// It defaults to dark until DetectTerminalBackground runs.
var Terminal = &TerminalInfo{
	Background:       color.Black,
	BackgroundIsDark: true,
}
