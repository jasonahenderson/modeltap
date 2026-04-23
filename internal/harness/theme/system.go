// Portions ported from github.com/sst/opencode at commit 5e86c9b
// (parent of f68374a, 2025-10-31). Original source MIT-licensed,
// copyright (c) 2025 opencode. See NOTICE.

package theme

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SystemTheme is a dynamic theme that derives its gray scale colors
// from the terminal's background color at runtime.
type SystemTheme struct {
	BaseTheme
	terminalBg       color.Color
	terminalBgIsDark bool
}

// NewSystemTheme creates a new instance of the dynamic system theme.
func NewSystemTheme(terminalBg color.Color, isDark bool) *SystemTheme {
	theme := &SystemTheme{
		terminalBg:       terminalBg,
		terminalBgIsDark: isDark,
	}
	theme.initializeColors()
	return theme
}

func (t *SystemTheme) Name() string { return "system" }

func (t *SystemTheme) initializeColors() {
	grays := t.generateGrayScale()

	// Brand colors using ANSI indices.
	t.PrimaryColor = lipgloss.AdaptiveColor{Dark: "6", Light: "6"}     // cyan
	t.SecondaryColor = lipgloss.AdaptiveColor{Dark: "5", Light: "5"}   // magenta
	t.AccentColor = lipgloss.AdaptiveColor{Dark: "6", Light: "6"}      // cyan

	// Status colors using ANSI indices.
	t.ErrorColor = lipgloss.AdaptiveColor{Dark: "1", Light: "1"}     // red
	t.WarningColor = lipgloss.AdaptiveColor{Dark: "3", Light: "3"}   // yellow
	t.SuccessColor = lipgloss.AdaptiveColor{Dark: "2", Light: "2"}   // green
	t.InfoColor = lipgloss.AdaptiveColor{Dark: "6", Light: "6"}      // cyan

	// Text colors.
	t.TextColor = lipgloss.AdaptiveColor{Dark: "", Light: ""}
	t.TextMutedColor = t.generateMutedTextColor()

	// Background colors.
	t.BackgroundColor = lipgloss.AdaptiveColor{Dark: "", Light: ""}
	t.BackgroundPanelColor = grays[2]
	t.BackgroundElementColor = grays[3]

	// Border colors.
	t.BorderSubtleColor = grays[6]
	t.BorderColor = grays[7]
	t.BorderActiveColor = grays[8]

	// Diff colors.
	t.DiffAddedColor = lipgloss.AdaptiveColor{Dark: "2", Light: "2"}
	t.DiffRemovedColor = lipgloss.AdaptiveColor{Dark: "1", Light: "1"}
	t.DiffContextColor = grays[7]
	t.DiffHunkHeaderColor = grays[7]
	t.DiffHighlightAddedColor = lipgloss.AdaptiveColor{Dark: "2", Light: "2"}
	t.DiffHighlightRemovedColor = lipgloss.AdaptiveColor{Dark: "1", Light: "1"}
	t.DiffAddedBgColor = grays[2]
	t.DiffRemovedBgColor = grays[2]
	t.DiffContextBgColor = grays[1]
	t.DiffLineNumberColor = grays[6]
	t.DiffAddedLineNumberBgColor = grays[3]
	t.DiffRemovedLineNumberBgColor = grays[3]

	// Markdown colors.
	t.MarkdownTextColor = lipgloss.AdaptiveColor{Dark: "", Light: ""}
	t.MarkdownHeadingColor = lipgloss.AdaptiveColor{Dark: "", Light: ""}
	t.MarkdownLinkColor = lipgloss.AdaptiveColor{Dark: "4", Light: "4"}
	t.MarkdownLinkTextColor = lipgloss.AdaptiveColor{Dark: "6", Light: "6"}
	t.MarkdownCodeColor = lipgloss.AdaptiveColor{Dark: "2", Light: "2"}
	t.MarkdownBlockQuoteColor = lipgloss.AdaptiveColor{Dark: "3", Light: "3"}
	t.MarkdownEmphColor = lipgloss.AdaptiveColor{Dark: "3", Light: "3"}
	t.MarkdownStrongColor = lipgloss.AdaptiveColor{Dark: "", Light: ""}
	t.MarkdownHorizontalRuleColor = t.BorderColor
	t.MarkdownListItemColor = lipgloss.AdaptiveColor{Dark: "4", Light: "4"}
	t.MarkdownListEnumerationColor = lipgloss.AdaptiveColor{Dark: "6", Light: "6"}
	t.MarkdownImageColor = lipgloss.AdaptiveColor{Dark: "4", Light: "4"}
	t.MarkdownImageTextColor = lipgloss.AdaptiveColor{Dark: "6", Light: "6"}
	t.MarkdownCodeBlockColor = lipgloss.AdaptiveColor{Dark: "", Light: ""}

	// Syntax colors.
	t.SyntaxCommentColor = t.TextMutedColor
	t.SyntaxKeywordColor = lipgloss.AdaptiveColor{Dark: "5", Light: "5"}
	t.SyntaxFunctionColor = lipgloss.AdaptiveColor{Dark: "4", Light: "4"}
	t.SyntaxVariableColor = lipgloss.AdaptiveColor{Dark: "", Light: ""}
	t.SyntaxStringColor = lipgloss.AdaptiveColor{Dark: "2", Light: "2"}
	t.SyntaxNumberColor = lipgloss.AdaptiveColor{Dark: "3", Light: "3"}
	t.SyntaxTypeColor = lipgloss.AdaptiveColor{Dark: "6", Light: "6"}
	t.SyntaxOperatorColor = lipgloss.AdaptiveColor{Dark: "6", Light: "6"}
	t.SyntaxPunctuationColor = lipgloss.AdaptiveColor{Dark: "", Light: ""}
}

func (t *SystemTheme) generateGrayScale() map[int]lipgloss.AdaptiveColor {
	grays := make(map[int]lipgloss.AdaptiveColor)

	r, g, b, _ := t.terminalBg.RGBA()
	bgR := float64(r >> 8)
	bgG := float64(g >> 8)
	bgB := float64(b >> 8)

	luminance := 0.299*bgR + 0.587*bgG + 0.114*bgB

	for i := 1; i <= 12; i++ {
		var stepColor string
		factor := float64(i) / 12.0

		if t.terminalBgIsDark {
			if luminance < 10 {
				grayValue := int(factor * 0.4 * 255)
				stepColor = fmt.Sprintf("#%02x%02x%02x", grayValue, grayValue, grayValue)
			} else {
				newLum := luminance + (255-luminance)*factor*0.4
				ratio := newLum / luminance
				newR := math.Min(bgR*ratio, 255)
				newG := math.Min(bgG*ratio, 255)
				newB := math.Min(bgB*ratio, 255)
				stepColor = fmt.Sprintf("#%02x%02x%02x", int(newR), int(newG), int(newB))
			}
		} else {
			if luminance > 245 {
				grayValue := int(255 - factor*0.4*255)
				stepColor = fmt.Sprintf("#%02x%02x%02x", grayValue, grayValue, grayValue)
			} else {
				newLum := luminance * (1 - factor*0.4)
				ratio := newLum / luminance
				newR := math.Max(bgR*ratio, 0)
				newG := math.Max(bgG*ratio, 0)
				newB := math.Max(bgB*ratio, 0)
				stepColor = fmt.Sprintf("#%02x%02x%02x", int(newR), int(newG), int(newB))
			}
		}

		grays[i] = lipgloss.AdaptiveColor{Dark: stepColor, Light: stepColor}
	}

	return grays
}

func (t *SystemTheme) generateMutedTextColor() lipgloss.AdaptiveColor {
	r, g, b, _ := t.terminalBg.RGBA()
	bgRf := float64(r >> 8)
	bgGf := float64(g >> 8)
	bgBf := float64(b >> 8)

	bgLum := 0.299*bgRf + 0.587*bgGf + 0.114*bgBf

	var grayValue int
	if t.terminalBgIsDark {
		if bgLum < 10 {
			grayValue = 180
		} else {
			grayValue = min(int(160+(bgLum*0.3)), 200)
		}
	} else {
		if bgLum > 245 {
			grayValue = 75
		} else {
			grayValue = max(int(100-((255-bgLum)*0.2)), 60)
		}
	}

	mutedColor := fmt.Sprintf("#%02x%02x%02x", grayValue, grayValue, grayValue)
	return lipgloss.AdaptiveColor{Dark: mutedColor, Light: mutedColor}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// DetectTerminalBackground probes the terminal for its background color
// using OSC 11 and falls back to COLORFGBG / dark default.
func DetectTerminalBackground() (bg color.Color, isDark bool) {
	// Try OSC 11 query if stdout is a terminal.
	if fd := os.Stdout.Fd(); fd > 0 {
		// OSC 11 response is not reliably available in all environments;
		// attempt a non-blocking read with a short timeout.
		bg, isDark = tryOSC11()
		if bg != nil {
			return bg, isDark
		}
	}

	// Fallback: COLORFGBG environment variable.
	if fgbg := os.Getenv("COLORFGBG"); fgbg != "" {
		parts := strings.Split(fgbg, ";")
		if len(parts) >= 2 {
			if bgNum, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				return colorInt(bgNum), bgNum < 8
			}
		}
	}

	// Final fallback: assume dark.
	return color.Black, true
}

// tryOSC11 attempts a quick OSC 11 background-color query.
// Returns (nil, false) when the terminal does not respond.
func tryOSC11() (color.Color, bool) {
	// OSC 11 probing requires raw terminal mode manipulation and
	// non-blocking reads. For portability we skip the low-level
	// termios dance and rely on the COLORFGBG / dark fallback.
	// A future improvement could use golang.org/x/term for a proper
	// OSC 11 exchange when the platform supports it.
	return nil, false
}

// colorInt converts an ANSI color number 0-15 to a color.Color.
func colorInt(n int) color.Color {
	ansi := []color.Color{
		color.RGBA{0, 0, 0, 255},         // black
		color.RGBA{128, 0, 0, 255},       // red
		color.RGBA{0, 128, 0, 255},       // green
		color.RGBA{128, 128, 0, 255},     // yellow
		color.RGBA{0, 0, 128, 255},       // blue
		color.RGBA{128, 0, 128, 255},     // magenta
		color.RGBA{0, 128, 128, 255},     // cyan
		color.RGBA{192, 192, 192, 255},   // white
		color.RGBA{128, 128, 128, 255},   // bright black
		color.RGBA{255, 0, 0, 255},       // bright red
		color.RGBA{0, 255, 0, 255},       // bright green
		color.RGBA{255, 255, 0, 255},     // bright yellow
		color.RGBA{0, 0, 255, 255},       // bright blue
		color.RGBA{255, 0, 255, 255},     // bright magenta
		color.RGBA{0, 255, 255, 255},     // bright cyan
		color.RGBA{255, 255, 255, 255},   // bright white
	}
	if n >= 0 && n < len(ansi) {
		return ansi[n]
	}
	return color.Black
}

// InitSystemTheme runs background detection and registers the "system"
// dynamic theme. Call once at harness startup.
func InitSystemTheme() {
	bg, isDark := DetectTerminalBackground()
	UpdateSystemTheme(bg, isDark)
}

// Ensure the system theme is registered on package init as a safe default.
func init() {
	// Defer actual detection to InitSystemTheme() so tests don't
	// block on terminal I/O during package init.
	RegisterTheme("system", NewSystemTheme(color.Black, true))
}
