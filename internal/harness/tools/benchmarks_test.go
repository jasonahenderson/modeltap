package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WU-095 micro-benchmarks. Each targets a known hot path called out
// by the v0.2.0 design or the WU-094 review. Run via:
//
//	go test ./internal/harness/tools/ -bench=. -benchmem -run=^$
//
// Targets are fast (ns/op) enough that -count=5 gives stable numbers
// on a laptop. Keep fixtures in-test — no external testdata so runs
// are reproducible.

// seedBenchRepo writes a fixed-size synthetic repo for Grep/Glob
// benchmarks. Files=count, lines=linesPerFile, needle appears in
// ~every 10th line so Grep has non-trivial match work to do.
func seedBenchRepo(b *testing.B, count, linesPerFile int) string {
	b.Helper()
	root := b.TempDir()
	for i := 0; i < count; i++ {
		dir := filepath.Join(root, "pkg", string(rune('a'+i%26)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("mkdir: %v", err)
		}
		var sb strings.Builder
		for j := 0; j < linesPerFile; j++ {
			if j%10 == 0 {
				sb.WriteString("findme " + strings.Repeat("x", 40) + "\n")
			} else {
				sb.WriteString("padding " + strings.Repeat("y", 40) + "\n")
			}
		}
		name := filepath.Join(dir, "f.go")
		if err := os.WriteFile(name, []byte(sb.String()), 0o644); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
	return root
}

func BenchmarkGrep_FilesWithMatches(b *testing.B) {
	root := seedBenchRepo(b, 200, 50)
	tool := NewGrepTool(root)
	in, _ := json.Marshal(map[string]any{
		"pattern":     "findme",
		"output_mode": "files_with_matches",
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := tool.Execute(context.Background(), in)
		if err != nil || res.Status != StatusSuccess {
			b.Fatalf("grep: %v / %q", err, res.Error)
		}
	}
}

func BenchmarkGrep_ContentMode(b *testing.B) {
	root := seedBenchRepo(b, 100, 50)
	tool := NewGrepTool(root)
	in, _ := json.Marshal(map[string]any{
		"pattern":     "findme",
		"output_mode": "content",
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := tool.Execute(context.Background(), in)
		if err != nil || res.Status != StatusSuccess {
			b.Fatalf("grep: %v", err)
		}
	}
}

func BenchmarkGlob_DoubleStar(b *testing.B) {
	root := seedBenchRepo(b, 500, 5)
	tool := NewGlobTool(root)
	in, _ := json.Marshal(map[string]any{"pattern": "**/*.go"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := tool.Execute(context.Background(), in)
		if err != nil || res.Status != StatusSuccess {
			b.Fatalf("glob: %v", err)
		}
	}
}

func BenchmarkReadText_SmallFile(b *testing.B) {
	root := b.TempDir()
	path := filepath.Join(root, "small.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("line of text\n", 200)), 0o644); err != nil {
		b.Fatalf("write: %v", err)
	}
	tool := NewReadTool(root, NewFileTracker())
	in, _ := json.Marshal(map[string]any{"file_path": "small.txt"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := tool.Execute(context.Background(), in)
		if err != nil || res.Status != StatusSuccess {
			b.Fatalf("read: %v / %q", err, res.Error)
		}
	}
}

func BenchmarkReadText_LargeFile(b *testing.B) {
	root := b.TempDir()
	path := filepath.Join(root, "large.txt")
	// ~512 KiB text file — within the default maxBytes cap.
	body := strings.Repeat("a line of reasonable length\n", 20_000)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		b.Fatalf("write: %v", err)
	}
	tool := NewReadTool(root, NewFileTracker())
	in, _ := json.Marshal(map[string]any{"file_path": "large.txt"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(context.Background(), in)
	}
}

// BenchmarkPermissionCheck_ReadOnly is cheap but sits on the hot
// path of every tool dispatch — a regression here affects every
// tool call the harness makes.
func BenchmarkPermissionCheck_ReadOnly(b *testing.B) {
	r := NewRegistry()
	r.Register(&stubTool{name: "ReadBench", risk: RiskReadOnly})
	p := NewPermissionEnforcer(PermDefault)
	tool := r.Get("ReadBench")
	in := json.RawMessage(`{}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Check(tool, in)
	}
}

// BenchmarkDangerousPatternMatch exercises the regex catalog in
// dangerous.go against common shell strings. Regression indicator
// when the catalog grows or is rewritten.
func BenchmarkDangerousPatternMatch(b *testing.B) {
	cases := []string{
		"ls -la",
		"echo hello",
		"rm -rf /",
		"curl -d foo http://example.com",
		"git status",
		"grep -r pattern .",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsDangerous(cases[i%len(cases)])
	}
}
