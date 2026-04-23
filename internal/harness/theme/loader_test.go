// Portions ported from github.com/sst/opencode at commit 5e86c9b
// (parent of f68374a, 2025-10-31). Original source MIT-licensed,
// copyright (c) 2025 opencode. See NOTICE.

package theme

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadEmbeddedThemes(t *testing.T) {
	err := LoadEmbeddedThemes()
	if err != nil {
		t.Fatalf("Failed to load themes: %v", err)
	}

	themes := AvailableThemes()
	if len(themes) == 0 {
		t.Fatal("No themes were loaded")
	}

	expectedThemes := []string{"tokyonight", "opencode", "everforest", "ayu"}
	for _, expected := range expectedThemes {
		if !slices.Contains(themes, expected) {
			t.Errorf("Expected theme %s not found", expected)
		}
	}

	tokyonight := GetTheme("tokyonight")
	if tokyonight == nil {
		t.Fatal("Failed to get tokyonight theme")
	}

	primary := tokyonight.Primary()
	if primary.Dark == "" || primary.Light == "" {
		t.Error("Primary color not properly set")
	}
}

func TestColorReferenceResolution(t *testing.T) {
	err := LoadEmbeddedThemes()
	if err != nil {
		t.Fatalf("Failed to load themes: %v", err)
	}

	solarized := GetTheme("solarized")
	if solarized == nil {
		t.Fatal("Failed to get solarized theme")
	}

	primary := solarized.Primary()
	if primary.Dark == "" || primary.Light == "" {
		t.Error("Primary color reference not resolved")
	}

	text := solarized.Text()
	if text.Dark == "" || text.Light == "" {
		t.Error("Text color reference not resolved")
	}
}

func TestLoadThemesFromDirectories(t *testing.T) {
	tempDir := t.TempDir()

	userConfig := filepath.Join(tempDir, "config")
	projectRoot := filepath.Join(tempDir, "project")
	cwd := filepath.Join(tempDir, "cwd")

	os.MkdirAll(filepath.Join(userConfig, "themes"), 0755)
	os.MkdirAll(filepath.Join(projectRoot, ".modeltap", "themes"), 0755)
	os.MkdirAll(filepath.Join(cwd, ".modeltap", "themes"), 0755)

	testTheme1 := `{
		"theme": {
			"primary": "#111111",
			"secondary": "#222222",
			"accent": "#333333",
			"text": "#ffffff",
			"textMuted": "#cccccc",
			"background": "#000000"
		}
	}`

	testTheme2 := `{
		"theme": {
			"primary": "#444444",
			"secondary": "#555555",
			"accent": "#666666",
			"text": "#ffffff",
			"textMuted": "#cccccc",
			"background": "#000000"
		}
	}`

	testTheme3 := `{
		"theme": {
			"primary": "#777777",
			"secondary": "#888888",
			"accent": "#999999",
			"text": "#ffffff",
			"textMuted": "#cccccc",
			"background": "#000000"
		}
	}`

	os.WriteFile(filepath.Join(userConfig, "themes", "override-test.json"), []byte(testTheme1), 0644)
	os.WriteFile(filepath.Join(projectRoot, ".modeltap", "themes", "override-test.json"), []byte(testTheme2), 0644)
	os.WriteFile(filepath.Join(cwd, ".modeltap", "themes", "override-test.json"), []byte(testTheme3), 0644)

	err := LoadThemesFromDirectories(userConfig, projectRoot, cwd)
	if err != nil {
		t.Fatalf("Failed to load themes from directories: %v", err)
	}

	overrideTheme := GetTheme("override-test")
	if overrideTheme == nil {
		t.Fatal("Failed to get override-test theme")
	}

	primary := overrideTheme.Primary()
	if primary.Dark == "" || primary.Light == "" {
		t.Error("Override theme not properly loaded")
	}
}
