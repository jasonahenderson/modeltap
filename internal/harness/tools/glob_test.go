package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func globInput(t *testing.T, pattern, path string) json.RawMessage {
	t.Helper()
	m := map[string]any{"pattern": pattern}
	if path != "" {
		m["path"] = path
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func writeGlobFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return abs
}

func TestGlob_NameAndRisk(t *testing.T) {
	g := NewGlobTool(t.TempDir())
	if g.Name() != ToolNameGlob {
		t.Errorf("Name = %q, want %q", g.Name(), ToolNameGlob)
	}
	if g.RiskLevel() != RiskReadOnly {
		t.Errorf("RiskLevel = %q, want read_only", g.RiskLevel())
	}
}

func TestGlob_SimplePattern(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "a.go", "")
	writeGlobFile(t, root, "b.go", "")
	writeGlobFile(t, root, "c.txt", "")

	g := NewGlobTool(root)
	res, err := g.Execute(context.Background(), globInput(t, "*.go", ""))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "a.go") || !strings.Contains(res.Output, "b.go") {
		t.Errorf("expected a.go + b.go; got %q", res.Output)
	}
	if strings.Contains(res.Output, "c.txt") {
		t.Errorf("c.txt should not match *.go: %q", res.Output)
	}
}

func TestGlob_DoubleStarPattern(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "top.go", "")
	writeGlobFile(t, root, "pkg/sub.go", "")
	writeGlobFile(t, root, "pkg/deep/nested.go", "")

	g := NewGlobTool(root)
	res, err := g.Execute(context.Background(), globInput(t, "**/*.go", ""))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	for _, want := range []string{"top.go", "sub.go", "nested.go"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("missing %s in output:\n%s", want, res.Output)
		}
	}
}

func TestGlob_NoMatches(t *testing.T) {
	root := t.TempDir()
	g := NewGlobTool(root)
	res, err := g.Execute(context.Background(), globInput(t, "*.nope", ""))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "no matches") {
		t.Errorf("expected empty-result marker; got %q", res.Output)
	}
}

func TestGlob_SortOrderByMtime(t *testing.T) {
	root := t.TempDir()
	older := writeGlobFile(t, root, "old.go", "")
	newer := writeGlobFile(t, root, "new.go", "")

	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatalf("chtimes older: %v", err)
	}
	future := time.Now()
	if err := os.Chtimes(newer, future, future); err != nil {
		t.Fatalf("chtimes newer: %v", err)
	}

	g := NewGlobTool(root)
	res, err := g.Execute(context.Background(), globInput(t, "*.go", ""))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	newIdx := strings.Index(res.Output, "new.go")
	oldIdx := strings.Index(res.Output, "old.go")
	if newIdx < 0 || oldIdx < 0 {
		t.Fatalf("both files should appear; got %q", res.Output)
	}
	if newIdx > oldIdx {
		t.Errorf("newer file should come first; got:\n%s", res.Output)
	}
}

func TestGlob_RelativePaths(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "pkg/a.go", "")

	g := NewGlobTool(root)
	res, err := g.Execute(context.Background(), globInput(t, "**/*.go", ""))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if strings.Contains(res.Output, root) {
		t.Errorf("output should contain relative path, not absolute root %q:\n%s", root, res.Output)
	}
	if !strings.Contains(res.Output, filepath.Join("pkg", "a.go")) {
		t.Errorf("expected relative pkg/a.go in output: %q", res.Output)
	}
}

func TestGlob_MissingPattern(t *testing.T) {
	g := NewGlobTool(t.TempDir())
	res, err := g.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestGlob_PathConfined(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "inside")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	g := NewGlobTool(root)
	res, err := g.Execute(context.Background(), globInput(t, "*.go", "../outside"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("expected error for out-of-root path; got status=%q err=%q", res.Status, res.Error)
	}
}
