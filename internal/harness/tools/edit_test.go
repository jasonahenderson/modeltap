package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newEditToolForTest(t *testing.T) (*EditTool, string, *FileTracker) {
	t.Helper()
	root := t.TempDir()
	tr := NewFileTracker()
	return NewEditTool(root, tr), root, tr
}

func editInput(t *testing.T, path, oldS, newS string, replaceAll bool) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"file_path":   path,
		"old_string":  oldS,
		"new_string":  newS,
		"replace_all": replaceAll,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func seed(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestEdit_RiskLevel(t *testing.T) {
	e, _, _ := newEditToolForTest(t)
	if e.RiskLevel() != RiskWrite {
		t.Errorf("Edit risk = %q, want %q", e.RiskLevel(), RiskWrite)
	}
	if e.Name() != "Edit" {
		t.Errorf("Edit name = %q, want Edit", e.Name())
	}
}

func TestEdit_SingleMatch(t *testing.T) {
	e, root, tr := newEditToolForTest(t)
	path := filepath.Join(root, "f.txt")
	seed(t, path, "alpha beta gamma")
	tr.MarkRead(path)

	res, err := e.Execute(context.Background(), editInput(t, path, "beta", "BETA", false))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "alpha BETA gamma" {
		t.Errorf("content = %q", got)
	}
}

func TestEdit_ReplaceAll(t *testing.T) {
	e, root, tr := newEditToolForTest(t)
	path := filepath.Join(root, "f.txt")
	seed(t, path, "x y x y x")
	tr.MarkRead(path)

	res, err := e.Execute(context.Background(), editInput(t, path, "x", "Z", true))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "Z y Z y Z" {
		t.Errorf("content = %q", got)
	}
	if !strings.Contains(res.Output, "3") {
		t.Errorf("output should report 3 replacements; got %q", res.Output)
	}
}

func TestEdit_NoMatch(t *testing.T) {
	e, root, tr := newEditToolForTest(t)
	path := filepath.Join(root, "f.txt")
	seed(t, path, "hello")
	tr.MarkRead(path)

	res, err := e.Execute(context.Background(), editInput(t, path, "MISSING", "x", false))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	if !strings.Contains(res.Error, "not found") {
		t.Errorf("Error should say not found; got %q", res.Error)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello" {
		t.Errorf("file should be untouched; got %q", got)
	}
}

func TestEdit_AmbiguousMatch(t *testing.T) {
	e, root, tr := newEditToolForTest(t)
	path := filepath.Join(root, "f.txt")
	seed(t, path, "foo bar foo")
	tr.MarkRead(path)

	res, err := e.Execute(context.Background(), editInput(t, path, "foo", "baz", false))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	if !strings.Contains(res.Error, "unique") && !strings.Contains(res.Error, "occurrence") {
		t.Errorf("Error should mention ambiguity; got %q", res.Error)
	}
}

func TestEdit_RequiresRead(t *testing.T) {
	e, root, _ := newEditToolForTest(t)
	path := filepath.Join(root, "f.txt")
	seed(t, path, "hello")
	// deliberately not MarkRead — edit should refuse

	res, err := e.Execute(context.Background(), editInput(t, path, "hello", "world", false))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	if !strings.Contains(strings.ToLower(res.Error), "read") {
		t.Errorf("error should mention Read prerequisite; got %q", res.Error)
	}
}

func TestEdit_DiffOutput(t *testing.T) {
	e, root, tr := newEditToolForTest(t)
	path := filepath.Join(root, "f.txt")
	seed(t, path, "before")
	tr.MarkRead(path)

	res, err := e.Execute(context.Background(), editInput(t, path, "before", "after", false))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q", res.Status)
	}
	if !strings.Contains(res.Output, "before") || !strings.Contains(res.Output, "after") {
		t.Errorf("output should show both old and new; got %q", res.Output)
	}
}

func TestEdit_MissingFile(t *testing.T) {
	e, root, tr := newEditToolForTest(t)
	path := filepath.Join(root, "does-not-exist.txt")
	tr.MarkRead(path) // even tracked reads can't revive a missing file

	res, err := e.Execute(context.Background(), editInput(t, path, "x", "y", false))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestEdit_PathOutsideRoot_Rejected(t *testing.T) {
	e, _, tr := newEditToolForTest(t)
	outside := filepath.Join(os.TempDir(), "outside-edit.txt")
	seed(t, outside, "content")
	defer os.Remove(outside)
	tr.MarkRead(outside)

	res, err := e.Execute(context.Background(), editInput(t, outside, "content", "evil", false))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
	got, _ := os.ReadFile(outside)
	if string(got) != "content" {
		t.Errorf("file outside root was modified: %q", got)
	}
}

func TestEdit_MissingFields(t *testing.T) {
	e, root, _ := newEditToolForTest(t)

	cases := []struct {
		name  string
		input json.RawMessage
	}{
		{"empty", json.RawMessage(`{}`)},
		{"no old_string", json.RawMessage(`{"file_path":"` + filepath.Join(root, "x") + `","new_string":"y"}`)},
		{"no new_string", json.RawMessage(`{"file_path":"` + filepath.Join(root, "x") + `","old_string":"a"}`)},
		{"empty old_string", json.RawMessage(`{"file_path":"` + filepath.Join(root, "x") + `","old_string":"","new_string":"y"}`)},
		{"malformed", json.RawMessage(`{`)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := e.Execute(context.Background(), c.input)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res.Status != StatusError {
				t.Errorf("Status = %q, want error", res.Status)
			}
		})
	}
}
