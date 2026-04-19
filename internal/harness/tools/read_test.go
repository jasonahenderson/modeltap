package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readInput(t *testing.T, m map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestRead_NameAndRisk(t *testing.T) {
	r := NewReadTool(t.TempDir(), NewFileTracker())
	if r.Name() != ToolNameRead {
		t.Errorf("Name = %q, want %q", r.Name(), ToolNameRead)
	}
	if r.RiskLevel() != RiskReadOnly {
		t.Errorf("RiskLevel = %q, want read_only", r.RiskLevel())
	}
}

func TestRead_TextFile_LineNumbering(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := NewReadTool(root, NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": "note.txt"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	for _, want := range []string{"1\talpha", "2\tbeta", "3\tgamma"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("missing %q in output:\n%s", want, res.Output)
		}
	}
}

func TestRead_TextFile_OffsetLimit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "long.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\nfive\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := NewReadTool(root, NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{
		"file_path": path,
		"offset":    2,
		"limit":     2,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "2\ttwo") || !strings.Contains(res.Output, "3\tthree") {
		t.Errorf("expected lines 2 and 3; got:\n%s", res.Output)
	}
	if strings.Contains(res.Output, "one") || strings.Contains(res.Output, "four") {
		t.Errorf("offset/limit did not clip extras:\n%s", res.Output)
	}
}

func TestRead_TrackerMarks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "x.txt")
	if err := os.WriteFile(path, []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tr := NewFileTracker()
	r := NewReadTool(root, tr)
	if _, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": path})); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !tr.HasRead(path) {
		t.Errorf("Read should mark file in tracker")
	}
}

func TestRead_NonExistent(t *testing.T) {
	r := NewReadTool(t.TempDir(), NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": "nope.txt"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestRead_MissingPath(t *testing.T) {
	r := NewReadTool(t.TempDir(), NewFileTracker())
	res, err := r.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestRead_OutOfRoot(t *testing.T) {
	r := NewReadTool(t.TempDir(), NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": "../../etc/passwd"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusError {
		t.Errorf("Status = %q, want error", res.Status)
	}
}

func TestRead_CSV(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "data.csv")
	if err := os.WriteFile(path, []byte("name,age\nAlice,30\nBob,25\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := NewReadTool(root, NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": path}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	for _, want := range []string{"name", "age", "Alice", "30", "Bob", "25"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("missing %q in CSV output:\n%s", want, res.Output)
		}
	}
}

func TestRead_Image_PNG(t *testing.T) {
	root := t.TempDir()
	// Smallest valid 1x1 PNG: use a hardcoded byte pattern.
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0x99, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x5B, 0xB6, 0xEE,
		0x56, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	path := filepath.Join(root, "tiny.png")
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := NewReadTool(root, NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": path}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if res.OutputType != "image" {
		t.Errorf("OutputType = %q, want image", res.OutputType)
	}
	if !strings.Contains(res.Output, "image/png") {
		t.Errorf("expected MIME marker; got %q", res.Output[:min(120, len(res.Output))])
	}
	// body should be base64 content — quick sanity check: decodes cleanly.
	i := strings.LastIndex(res.Output, "\n")
	if i < 0 {
		t.Fatalf("image output should have header+body; got %q", res.Output)
	}
	body := res.Output[i+1:]
	if _, err := base64.StdEncoding.DecodeString(body); err != nil {
		t.Errorf("base64 body did not decode: %v", err)
	}
}

func TestRead_FormatDetection_MagicBytes(t *testing.T) {
	root := t.TempDir()
	// .dat extension with PNG magic bytes → should still detect as image.
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}
	path := filepath.Join(root, "mystery.dat")
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if f, err := detectFormat(path); err != nil {
		t.Fatalf("detectFormat: %v", err)
	} else if f != FormatImage {
		t.Errorf("detectFormat = %v, want Image", f)
	}
}

func TestRead_FormatDetection_Extension(t *testing.T) {
	cases := map[string]FileFormat{
		"x.pdf":  FormatPDF,
		"x.docx": FormatDOCX,
		"x.png":  FormatImage,
		"x.jpg":  FormatImage,
		"x.xlsx": FormatSpreadsheet,
		"x.csv":  FormatCSV,
		"x.go":   FormatText,
		"x":      FormatText,
	}
	for name, want := range cases {
		f := detectFormatByExt(name)
		if f != want {
			t.Errorf("detectFormatByExt(%q) = %v, want %v", name, f, want)
		}
	}
}
