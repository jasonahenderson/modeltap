package tools

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDOCXFixture creates a minimal-but-valid DOCX package containing
// a single document.xml with two paragraphs and a <w:tab/> between
// two runs. Enough to exercise the extractor without pulling a real
// Word-generated fixture into the repo.
func writeDOCXFixture(t *testing.T, root, name, docXML string) string {
	t.Helper()
	path := filepath.Join(root, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"word/document.xml": docXML,
	}
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return path
}

func TestRead_DOCX(t *testing.T) {
	root := t.TempDir()
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t xml:space="preserve">Hello </w:t></w:r>
      <w:r><w:t>world.</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>Column1</w:t><w:tab/><w:t>Column2</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`
	path := writeDOCXFixture(t, root, "note.docx", doc)

	r := NewReadTool(root, NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": path}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "Hello world.") {
		t.Errorf("expected joined run text; got %q", res.Output)
	}
	if !strings.Contains(res.Output, "Column1\tColumn2") {
		t.Errorf("expected tab between runs; got %q", res.Output)
	}
	// Two paragraphs → a newline between them.
	if !strings.Contains(res.Output, "world.\nColumn1") {
		t.Errorf("expected paragraph break; got %q", res.Output)
	}
}

func TestRead_DOCX_MagicDetection(t *testing.T) {
	root := t.TempDir()
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Detected</w:t></w:r></w:p></w:body>
</w:document>`
	// Use a non-.docx extension so detection must fall back to magic bytes.
	path := writeDOCXFixture(t, root, "opaque.bin", doc)
	f, err := detectFormat(path)
	if err != nil {
		t.Fatalf("detectFormat: %v", err)
	}
	if f != FormatDOCX {
		t.Errorf("detectFormat = %v, want FormatDOCX", f)
	}
}
