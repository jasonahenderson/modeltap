package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func grepInput(t *testing.T, m map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestGrep_NameAndRisk(t *testing.T) {
	g := NewGrepTool(t.TempDir())
	if g.Name() != ToolNameGrep {
		t.Errorf("Name = %q, want %q", g.Name(), ToolNameGrep)
	}
	if g.RiskLevel() != RiskReadOnly {
		t.Errorf("RiskLevel = %q, want read_only", g.RiskLevel())
	}
}

func TestGrep_SimplePattern_FilesWithMatches(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "a.go", "package foo\n\nfunc Hello() {}\n")
	writeGlobFile(t, root, "b.go", "package bar\n")
	writeGlobFile(t, root, "c.txt", "Hello there\n")

	g := NewGrepTool(root)
	res, err := g.Execute(context.Background(), grepInput(t, map[string]any{"pattern": "Hello"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "a.go") || !strings.Contains(res.Output, "c.txt") {
		t.Errorf("expected a.go and c.txt; got %q", res.Output)
	}
	if strings.Contains(res.Output, "b.go") {
		t.Errorf("b.go should not match: %q", res.Output)
	}
}

func TestGrep_Regex(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "x.go", "var foo123 = 1\nvar foo42 = 2\n")

	g := NewGrepTool(root)
	res, err := g.Execute(context.Background(), grepInput(t, map[string]any{
		"pattern":     `foo\d+`,
		"output_mode": "content",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "foo123") || !strings.Contains(res.Output, "foo42") {
		t.Errorf("expected both matches; got %q", res.Output)
	}
}

func TestGrep_CaseInsensitive(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "x.txt", "Hello World\n")

	g := NewGrepTool(root)
	res, err := g.Execute(context.Background(), grepInput(t, map[string]any{
		"pattern":          "hello",
		"case_insensitive": true,
		"output_mode":      "content",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "Hello World") {
		t.Errorf("expected hit; got %q", res.Output)
	}
}

func TestGrep_Context(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "x.txt", "before\ntarget\nafter\n")

	g := NewGrepTool(root)
	res, err := g.Execute(context.Background(), grepInput(t, map[string]any{
		"pattern":     "target",
		"output_mode": "content",
		"context":     1,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	for _, want := range []string{"before", "target", "after"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("context missing %q: %q", want, res.Output)
		}
	}
}

func TestGrep_OutputMode_Count(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "x.txt", "hit\nhit\nmiss\n")
	writeGlobFile(t, root, "y.txt", "hit\n")

	g := NewGrepTool(root)
	res, err := g.Execute(context.Background(), grepInput(t, map[string]any{
		"pattern":     "hit",
		"output_mode": "count",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "x.txt:2") {
		t.Errorf("expected x.txt:2 in count output: %q", res.Output)
	}
	if !strings.Contains(res.Output, "y.txt:1") {
		t.Errorf("expected y.txt:1 in count output: %q", res.Output)
	}
}

func TestGrep_GlobFilter(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "keep.go", "needle\n")
	writeGlobFile(t, root, "skip.txt", "needle\n")

	g := NewGrepTool(root)
	res, err := g.Execute(context.Background(), grepInput(t, map[string]any{
		"pattern": "needle",
		"glob":    "*.go",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "keep.go") {
		t.Errorf("expected keep.go; got %q", res.Output)
	}
	if strings.Contains(res.Output, "skip.txt") {
		t.Errorf("skip.txt should be filtered out: %q", res.Output)
	}
}

func TestGrep_NoMatches(t *testing.T) {
	root := t.TempDir()
	writeGlobFile(t, root, "x.txt", "nothing here\n")

	g := NewGrepTool(root)
	res, err := g.Execute(context.Background(), grepInput(t, map[string]any{"pattern": "missing"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "no matches") {
		t.Errorf("expected no-matches marker; got %q", res.Output)
	}
}

func TestGrep_MissingPattern(t *testing.T) {
	g := NewGrepTool(t.TempDir())
	res, err := g.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestGrep_InvalidRegex(t *testing.T) {
	g := NewGrepTool(t.TempDir())
	res, err := g.Execute(context.Background(), grepInput(t, map[string]any{"pattern": "("}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}
