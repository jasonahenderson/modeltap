package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func bashInput(t *testing.T, cmd string, timeoutMS int) json.RawMessage {
	t.Helper()
	m := map[string]any{"command": cmd}
	if timeoutMS > 0 {
		m["timeout_ms"] = timeoutMS
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func skipIfWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Bash tool requires a POSIX shell")
	}
}

func TestBash_NameAndRisk(t *testing.T) {
	b := NewBashTool(t.TempDir())
	if b.Name() != ToolNameBash {
		t.Errorf("Name = %q, want %q", b.Name(), ToolNameBash)
	}
	if b.RiskLevel() != RiskExecute {
		t.Errorf("RiskLevel = %q, want %q", b.RiskLevel(), RiskExecute)
	}
}

func TestBash_SimpleCommand(t *testing.T) {
	skipIfWindows(t)
	b := NewBashTool(t.TempDir())

	res, err := b.Execute(context.Background(), bashInput(t, "echo hello", 0))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q output=%q", res.Status, res.Error, res.Output)
	}
	if !strings.Contains(res.Output, "hello") {
		t.Errorf("output = %q", res.Output)
	}
}

func TestBash_StderrCapture(t *testing.T) {
	skipIfWindows(t)
	b := NewBashTool(t.TempDir())

	res, err := b.Execute(context.Background(), bashInput(t, "echo err >&2", 0))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "err") {
		t.Errorf("stderr should be captured: %q", res.Output)
	}
}

func TestBash_NonZeroExit(t *testing.T) {
	skipIfWindows(t)
	b := NewBashTool(t.TempDir())

	res, err := b.Execute(context.Background(), bashInput(t, "exit 7", 0))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	if !strings.Contains(res.Error, "exit 7") {
		t.Errorf("error should include exit code: %q", res.Error)
	}
}

func TestBash_Timeout(t *testing.T) {
	skipIfWindows(t)
	b := NewBashTool(t.TempDir())

	start := time.Now()
	res, err := b.Execute(context.Background(), bashInput(t, "sleep 10", 100))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	if !strings.Contains(res.Error, "timed out") {
		t.Errorf("error should mention timeout: %q", res.Error)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("timeout did not kill the process quickly: %v", elapsed)
	}
}

func TestBash_OutputTruncation(t *testing.T) {
	skipIfWindows(t)
	b := NewBashTool(t.TempDir())
	b.SetMaxOutput(50)

	res, err := b.Execute(context.Background(),
		bashInput(t, `printf 'x%.0s' {1..500}`, 0))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "truncated") {
		t.Errorf("output should announce truncation: %q", res.Output)
	}
	if len(res.Output) > 200 {
		t.Errorf("truncated output still too large: %d bytes", len(res.Output))
	}
}

func TestBash_WorkingDirectory(t *testing.T) {
	skipIfWindows(t)
	root := t.TempDir()
	// pwd returns the canonical path; resolve through os.Readlink-ish
	// by asking Go's filepath.EvalSymlinks.
	canon, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	b := NewBashTool(root)

	res, err := b.Execute(context.Background(), bashInput(t, "pwd", 0))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	got := strings.TrimSpace(res.Output)
	if got != canon && got != root {
		t.Errorf("cwd = %q, want %q or %q", got, canon, root)
	}
}

func TestBash_MissingCommand(t *testing.T) {
	b := NewBashTool(t.TempDir())
	cases := []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`{"command":""}`),
		json.RawMessage(`{`),
	}
	for i, in := range cases {
		res, err := b.Execute(context.Background(), in)
		if err != nil {
			t.Fatalf("case %d: Execute: %v", i, err)
		}
		if res.Status != StatusError {
			t.Errorf("case %d: Status = %q, want error", i, res.Status)
		}
	}
}

func TestBash_TimeoutCap(t *testing.T) {
	skipIfWindows(t)
	// A user-supplied absurd timeout must be capped rather than passed
	// straight through to context.WithTimeout.
	b := NewBashTool(t.TempDir())
	in := json.RawMessage(`{"command":"true","timeout_ms":9999999999}`)
	res, err := b.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Errorf("Status = %q err=%q", res.Status, res.Error)
	}
}
