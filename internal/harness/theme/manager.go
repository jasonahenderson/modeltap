// Portions ported from github.com/sst/opencode at commit 5e86c9b
// (parent of f68374a, 2025-10-31). Original source MIT-licensed,
// copyright (c) 2025 opencode. See NOTICE.

package theme

import (
	"fmt"
	"image/color"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// Manager handles theme registration, selection, and retrieval.
type Manager struct {
	themes      map[string]Theme
	currentName string
	mu          sync.RWMutex
}

// Global instance of the theme manager.
var globalManager = &Manager{
	themes: make(map[string]Theme),
}

// RegisterTheme adds a new theme to the registry.
// If this is the first theme registered, it becomes the default.
func RegisterTheme(name string, theme Theme) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	globalManager.themes[name] = theme

	if globalManager.currentName == "" {
		globalManager.currentName = name
	}
}

// SetTheme changes the active theme to the one with the specified name.
// Returns an error if the theme doesn't exist.
func SetTheme(name string) error {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	_, exists := globalManager.themes[name]
	if !exists {
		return fmt.Errorf("theme '%s' not found", name)
	}

	globalManager.currentName = name
	return nil
}

// CurrentTheme returns the currently active theme.
// If no theme is set, it returns nil.
func CurrentTheme() Theme {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()

	if globalManager.currentName == "" {
		return nil
	}
	return globalManager.themes[globalManager.currentName]
}

// CurrentThemeName returns the name of the currently active theme.
func CurrentThemeName() string {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()
	return globalManager.currentName
}

// AvailableThemes returns a list of all registered theme names.
func AvailableThemes() []string {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()

	names := make([]string, 0, len(globalManager.themes))
	for name := range globalManager.themes {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		if a == "opencode" {
			return -1
		}
		if b == "opencode" {
			return 1
		}
		if a == "system" {
			return -1
		}
		if b == "system" {
			return 1
		}
		return strings.Compare(a, b)
	})
	return names
}

// GetTheme returns a specific theme by name.
// Returns nil if the theme doesn't exist.
func GetTheme(name string) Theme {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()
	return globalManager.themes[name]
}

// UpdateSystemTheme updates the system theme with terminal background info.
func UpdateSystemTheme(terminalBg color.Color, isDark bool) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	dynamicTheme := NewSystemTheme(terminalBg, isDark)
	globalManager.themes["system"] = dynamicTheme
}

// isAnsiColor checks if a color string represents an ANSI 0-16 color.
func isAnsiColor(s string) bool {
	if s == "" {
		return false
	}
	num, err := strconv.Atoi(s)
	return err == nil && num >= 0 && num <= 15
}

// adaptiveColorUsesAnsi checks if an AdaptiveColor uses ANSI colors.
func adaptiveColorUsesAnsi(ac lipgloss.AdaptiveColor) bool {
	return isAnsiColor(ac.Dark) || isAnsiColor(ac.Light)
}

// CurrentThemeUsesAnsiColors returns true if the current theme uses
// any ANSI 0-15 colors.
func CurrentThemeUsesAnsiColors() bool {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()

	t := globalManager.themes[globalManager.currentName]
	if t == nil {
		return false
	}
	// Check a representative subset of colors.
	return adaptiveColorUsesAnsi(t.Primary()) ||
		adaptiveColorUsesAnsi(t.Secondary()) ||
		adaptiveColorUsesAnsi(t.Accent()) ||
		adaptiveColorUsesAnsi(t.Error()) ||
		adaptiveColorUsesAnsi(t.Warning()) ||
		adaptiveColorUsesAnsi(t.Success()) ||
		adaptiveColorUsesAnsi(t.Info()) ||
		adaptiveColorUsesAnsi(t.Text()) ||
		adaptiveColorUsesAnsi(t.TextMuted()) ||
		adaptiveColorUsesAnsi(t.Background()) ||
		adaptiveColorUsesAnsi(t.Border()) ||
		adaptiveColorUsesAnsi(t.BorderActive()) ||
		adaptiveColorUsesAnsi(t.BorderSubtle())
}
