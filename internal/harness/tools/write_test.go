package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newWriteToolForTest builds a WriteTool rooted at t.TempDir() with a
// fresh FileTracker. Returned as (tool, root, tracker) so tests can
// assert tracker state.
func newWriteToolForTest(t *testing.T) (*WriteTool, string, *FileTracker) {
	t.Helper()
	root := t.TempDir()
	tr := NewFileTracker()
	return NewWriteTool(root, tr), root, tr
}

func writeInput(t *testing.T, path, content string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]string{
		"file_path": path,
		"content":   content,
	})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	return b
}

func TestWrite_RiskLevel(t *testing.T) {
	w, _, _ := newWriteToolForTest(t)
	if w.RiskLevel() != RiskWrite {
		t.Errorf("Write risk = %q, want %q", w.RiskLevel(), RiskWrite)
	}
	if w.Name() != "Write" {
		t.Errorf("Write name = %q, want %q", w.Name(), "Write")
	}
	if w.OutputEnvelope() != "text" {
		t.Errorf("Write output envelope = %q, want text", w.OutputEnvelope())
	}
}

func TestWrite_NewFile(t *testing.T) {
	w, root, tr := newWriteToolForTest(t)
	path := filepath.Join(root, "hello.txt")
	content := "hello world"

	res, err := w.Execute(context.Background(), writeInput(t, path, content))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q, want success; err=%q", res.Status, res.Error)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != content {
		t.Errorf("file contents = %q, want %q", got, content)
	}
	if !strings.Contains(res.Output, path) {
		t.Errorf("output should mention path; got %q", res.Output)
	}
	if !tr.HasRead(path) {
		t.Errorf("tracker should record write as a read so Edit can follow up")
	}
}

func TestWrite_Overwrite_IncludesSnapshot(t *testing.T) {
	w, root, _ := newWriteToolForTest(t)
	path := filepath.Join(root, "existing.txt")
	if err := os.WriteFile(path, []byte("original contents"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	res, err := w.Execute(context.Background(), writeInput(t, path, "new contents"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "original contents") {
		t.Errorf("overwrite output should carry previous snapshot; got %q", res.Output)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new contents" {
		t.Errorf("file not updated: got %q", got)
	}
}

func TestWrite_CreateDirs(t *testing.T) {
	w, root, _ := newWriteToolForTest(t)
	path := filepath.Join(root, "a", "b", "c", "nested.txt")

	res, err := w.Execute(context.Background(), writeInput(t, path, "nested"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected nested file to exist: %v", err)
	}
}

func TestWrite_PathOutsideRoot_Rejected(t *testing.T) {
	w, _, _ := newWriteToolForTest(t)
	outside := filepath.Join(os.TempDir(), "not-in-root.txt")

	res, err := w.Execute(context.Background(), writeInput(t, outside, "x"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error for outside-root path", res.Status)
	}
	if res.Error == "" {
		t.Errorf("error message should be set")
	}
	if _, err := os.Stat(outside); err == nil {
		os.Remove(outside)
		t.Errorf("write should not have created file outside root")
	}
}

func TestWrite_MissingFields(t *testing.T) {
	w, root, _ := newWriteToolForTest(t)

	cases := []struct {
		name  string
		input json.RawMessage
	}{
		{"empty input", json.RawMessage(`{}`)},
		{"no content", json.RawMessage(`{"file_path":"` + filepath.Join(root, "x.txt") + `"}`)},
		{"no file_path", json.RawMessage(`{"content":"x"}`)},
		{"malformed", json.RawMessage(`{`)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := w.Execute(context.Background(), c.input)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res.Status != StatusError {
				t.Errorf("Status = %q, want error", res.Status)
			}
		})
	}
}

func TestWrite_RelativePath_ResolvesToRoot(t *testing.T) {
	w, root, _ := newWriteToolForTest(t)

	res, err := w.Execute(context.Background(), writeInput(t, "rel.txt", "rel"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	got, err := os.ReadFile(filepath.Join(root, "rel.txt"))
	if err != nil {
		t.Fatalf("relative path should resolve under root: %v", err)
	}
	if string(got) != "rel" {
		t.Errorf("content mismatch: %q", got)
	}
}

// TestResolveInRoot_RejectsSymlinkEscape pins WU-094 C-4: the previous
// build's resolveInRoot did Clean + Rel only. A symlink inside the
// project (e.g. a checked-in `./secret → /etc/passwd`) would pass
// the textual check and let Read/Write/Edit follow through to an
// out-of-root target. The fix walks EvalSymlinks; symlinked
// descendants that escape the evaluated root are now rejected.
func TestResolveInRoot_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	// Create a symlink inside the root that targets /etc/passwd.
	linkPath := filepath.Join(root, "secret")
	if err := os.Symlink("/etc/passwd", linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, err := resolveInRoot(root, "secret")
	if err == nil {
		t.Fatalf("expected symlink escape to be rejected")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should name symlink escape; got %v", err)
	}
}

// TestResolveInRoot_AllowsSymlinkedProjectRoot confirms the fix
// doesn't break the common case where the project root itself is
// reached via a symlink (e.g. /tmp → /private/tmp on macOS).
// EvalSymlinks the root and allow descendants under the resolved
// form.
func TestResolveInRoot_AllowsSymlinkedProjectRoot(t *testing.T) {
	realRoot := t.TempDir()
	// Place a file in the root; resolveInRoot should accept it even
	// when called via a symlink path.
	if err := os.WriteFile(filepath.Join(realRoot, "inside.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := resolveInRoot(realRoot, "inside.txt"); err != nil {
		t.Errorf("legitimate in-root path should resolve: %v", err)
	}
}

// TestResolveInRoot_AllowsNonexistentPath confirms Write / Edit can
// still create a new file inside the root — the fix must not break
// paths that don't exist yet (common case).
func TestResolveInRoot_AllowsNonexistentPath(t *testing.T) {
	root := t.TempDir()
	got, err := resolveInRoot(root, "nested/new.txt")
	if err != nil {
		t.Fatalf("nonexistent path under root should resolve: %v", err)
	}
	if !strings.HasPrefix(got, root) {
		t.Errorf("resolved path %q should be under root %q", got, root)
	}
}

// TestResolveInRoot_RejectsSymlinkedParent covers a subtler case:
// a file whose parent directory is a symlink pointing outside the
// root. Creating through such a parent would land outside scope.
func TestResolveInRoot_RejectsSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	// Inside the root, create a symlink directory pointing outside.
	linkDir := filepath.Join(root, "escape")
	if err := os.Symlink(outsideDir, linkDir); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, err := resolveInRoot(root, "escape/new.txt")
	if err == nil {
		t.Fatalf("expected escape-through-symlinked-parent to be rejected")
	}
}
