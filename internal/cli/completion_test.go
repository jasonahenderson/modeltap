package cli

import (
	"bytes"
	"strings"
	"testing"
)

func executeCompletion(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("completion %v returned error: %v", args, err)
	}
	return buf.String()
}

func TestCompletionBash(t *testing.T) {
	output := executeCompletion(t, "completion", "bash")
	if output == "" {
		t.Fatal("expected bash completion output, got empty string")
	}
	// Cobra bash completions contain a bash-specific marker.
	if !strings.Contains(output, "bash") && !strings.Contains(output, "complete") {
		t.Errorf("expected bash completion markers in output, got: %.200s...", output)
	}
}

func TestCompletionZsh(t *testing.T) {
	output := executeCompletion(t, "completion", "zsh")
	if output == "" {
		t.Fatal("expected zsh completion output, got empty string")
	}
	if !strings.Contains(output, "zsh") && !strings.Contains(output, "compdef") {
		t.Errorf("expected zsh completion markers in output, got: %.200s...", output)
	}
}

func TestCompletionFish(t *testing.T) {
	output := executeCompletion(t, "completion", "fish")
	if output == "" {
		t.Fatal("expected fish completion output, got empty string")
	}
	if !strings.Contains(output, "fish") && !strings.Contains(output, "complete") {
		t.Errorf("expected fish completion markers in output, got: %.200s...", output)
	}
}

func TestCompletionPowershell(t *testing.T) {
	output := executeCompletion(t, "completion", "powershell")
	if output == "" {
		t.Fatal("expected powershell completion output, got empty string")
	}
	if !strings.Contains(output, "powershell") && !strings.Contains(output, "Register-ArgumentCompleter") {
		t.Errorf("expected powershell completion markers in output, got: %.200s...", output)
	}
}

func TestCompletionHelpText(t *testing.T) {
	output := executeCompletion(t, "completion", "--help")
	expected := []string{
		"bash",
		"zsh",
		"fish",
		"powershell",
	}
	for _, s := range expected {
		if !strings.Contains(output, s) {
			t.Errorf("completion help missing %q", s)
		}
	}
}
