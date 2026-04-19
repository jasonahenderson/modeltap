package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMinimalPDF builds a tiny but standards-compliant single-page
// PDF containing the supplied ASCII text and writes it to path. The
// byte offsets in the xref table are computed dynamically so the PDF
// is self-consistent regardless of text length.
func writeMinimalPDF(t *testing.T, path, text string) {
	t.Helper()

	escaped := strings.ReplaceAll(text, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, "(", `\(`)
	escaped = strings.ReplaceAll(escaped, ")", `\)`)
	content := fmt.Sprintf("BT\n/F1 18 Tf\n72 720 Td\n(%s) Tj\nET\n", escaped)
	contentBytes := []byte(content)

	var buf bytes.Buffer
	offsets := make([]int, 6) // object 0 (unused) .. object 5

	buf.WriteString("%PDF-1.4\n%\xff\xff\xff\xff\n")

	offsets[1] = buf.Len()
	buf.WriteString("1 0 obj\n<</Type /Catalog /Pages 2 0 R>>\nendobj\n")

	offsets[2] = buf.Len()
	buf.WriteString("2 0 obj\n<</Type /Pages /Kids [3 0 R] /Count 1>>\nendobj\n")

	offsets[3] = buf.Len()
	buf.WriteString("3 0 obj\n<</Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources <</Font <</F1 5 0 R>>>>>>\nendobj\n")

	offsets[4] = buf.Len()
	fmt.Fprintf(&buf, "4 0 obj\n<</Length %d>>\nstream\n", len(contentBytes))
	buf.Write(contentBytes)
	buf.WriteString("endstream\nendobj\n")

	offsets[5] = buf.Len()
	buf.WriteString("5 0 obj\n<</Type /Font /Subtype /Type1 /BaseFont /Helvetica>>\nendobj\n")

	xrefOff := buf.Len()
	buf.WriteString("xref\n0 6\n")
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	buf.WriteString("trailer\n<</Size 6 /Root 1 0 R>>\n")
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefOff)

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
}

func TestRead_PDF(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.pdf")
	writeMinimalPDF(t, path, "Hello PDF world")

	r := NewReadTool(root, NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": path}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	if !strings.Contains(res.Output, "Hello PDF world") {
		t.Errorf("expected extracted text; got %q", res.Output)
	}
	if !strings.Contains(res.Output, "# Page 1") {
		t.Errorf("expected page header; got %q", res.Output)
	}
}

func TestRead_PDF_MagicDetection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "opaque.bin")
	writeMinimalPDF(t, path, "Detect me")
	f, err := detectFormat(path)
	if err != nil {
		t.Fatalf("detectFormat: %v", err)
	}
	if f != FormatPDF {
		t.Errorf("detectFormat = %v, want FormatPDF", f)
	}
}

func TestResolvePDFRange(t *testing.T) {
	cases := []struct {
		expr      string
		total     int
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{expr: "", total: 5, wantStart: 1, wantEnd: 5},
		{expr: "", total: 50, wantStart: 1, wantEnd: pdfMaxPagesRange},
		{expr: "3", total: 5, wantStart: 3, wantEnd: 3},
		{expr: "1-5", total: 20, wantStart: 1, wantEnd: 5},
		{expr: "10-30", total: 40, wantStart: 10, wantEnd: 29}, // capped at +pdfMaxPagesRange-1
		{expr: "5-100", total: 10, wantStart: 5, wantEnd: 10},  // end clamped to total
		{expr: "foo", total: 5, wantErr: true},
		{expr: "5-2", total: 10, wantErr: true},
		{expr: "0", total: 5, wantErr: true},
		{expr: "100", total: 5, wantErr: true},
		{expr: "a-b", total: 5, wantErr: true},
	}
	for _, tc := range cases {
		s, e, err := resolvePDFRange(tc.expr, tc.total)
		if tc.wantErr {
			if err == nil {
				t.Errorf("expr=%q total=%d: want error, got %d-%d", tc.expr, tc.total, s, e)
			}
			continue
		}
		if err != nil {
			t.Errorf("expr=%q total=%d: unexpected error %v", tc.expr, tc.total, err)
			continue
		}
		if s != tc.wantStart || e != tc.wantEnd {
			t.Errorf("expr=%q total=%d: got %d-%d, want %d-%d",
				tc.expr, tc.total, s, e, tc.wantStart, tc.wantEnd)
		}
	}
}
