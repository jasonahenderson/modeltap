package tools

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ReadTool reads files from the project tree with automatic format
// detection. Text / CSV output is line-numbered table text; images are
// base64-encoded with a MIME header; PDF / DOCX / XLSX return extracted
// plain text. A successful read marks the file in the shared
// FileTracker so Edit can modify it later.
type ReadTool struct {
	projectRoot string
	tracker     *FileTracker
	maxBytes    int // text / CSV / image body cap; prevents runaway output
}

// NewReadTool constructs a Read tool bound to projectRoot and a
// FileTracker (typically the Executor's tracker).
func NewReadTool(projectRoot string, tracker *FileTracker) *ReadTool {
	return &ReadTool{projectRoot: projectRoot, tracker: tracker, maxBytes: 1 * 1024 * 1024}
}

// SetMaxBytes overrides the body-size cap (tests use this).
func (r *ReadTool) SetMaxBytes(n int) { r.maxBytes = n }

// ToolNameRead is the canonical registered name.
const ToolNameRead = "Read"

func (r *ReadTool) Name() string { return ToolNameRead }
func (r *ReadTool) Description() string {
	return "Read a file (text, PDF, DOCX, image, spreadsheet, CSV)"
}
func (r *ReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "file_path": {"type":"string","description":"Absolute or project-relative path to the file"},
    "offset":    {"type":"integer","description":"Line number to start from (text only)"},
    "limit":     {"type":"integer","description":"Max lines to read (text only)"},
    "pages":     {"type":"string","description":"Page range for PDFs, e.g. '1-5', '3', '10-20' (max 20 pages)"}
  },
  "required": ["file_path"]
}`)
}
func (r *ReadTool) OutputEnvelope() string { return "text" }
func (r *ReadTool) RiskLevel() RiskLevel   { return RiskReadOnly }

type readArgs struct {
	FilePath *string `json:"file_path"`
	Offset   int     `json:"offset"`
	Limit    int     `json:"limit"`
	Pages    string  `json:"pages"`
}

func (r *ReadTool) Execute(_ context.Context, input json.RawMessage) (*ToolExecResult, error) {
	var in readArgs
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorResult("invalid input: %v", err), nil
	}
	if in.FilePath == nil || *in.FilePath == "" {
		return ErrorResult("file_path is required"), nil
	}

	abs, err := resolveInRoot(r.projectRoot, *in.FilePath)
	if err != nil {
		return ErrorResult("%v", err), nil
	}
	if _, err := os.Stat(abs); err != nil {
		return ErrorResult("%v", err), nil
	}

	format, err := detectFormat(abs)
	if err != nil {
		return ErrorResult("detect format: %v", err), nil
	}

	var result *ToolExecResult
	switch format {
	case FormatText:
		result, err = readText(abs, in.Offset, in.Limit, r.maxBytes)
	case FormatCSV:
		result, err = readCSVFile(abs, r.maxBytes)
	case FormatImage:
		result, err = readImage(abs, r.maxBytes)
	case FormatPDF:
		result, err = readPDF(abs, in.Pages)
	case FormatDOCX:
		result, err = readDOCX(abs)
	case FormatSpreadsheet:
		result, err = readXLSX(abs)
	default:
		return ErrorResult("unsupported format"), nil
	}
	if err != nil {
		return ErrorResult("%v", err), nil
	}

	if result != nil && result.Status == StatusSuccess {
		r.tracker.MarkRead(abs)
	}
	return result, nil
}

// FileFormat identifies the family of reader used for a given file.
type FileFormat int

const (
	FormatText FileFormat = iota
	FormatPDF
	FormatDOCX
	FormatImage
	FormatSpreadsheet
	FormatCSV
)

// detectFormat returns the FileFormat for path. Extension is consulted
// first, and if the extension is absent or maps to FormatText we peek
// at the magic bytes to catch mislabelled files.
func detectFormat(path string) (FileFormat, error) {
	byExt := detectFormatByExt(path)
	if byExt != FormatText {
		return byExt, nil
	}
	magic, err := detectFormatByMagic(path)
	if err != nil {
		return FormatText, err
	}
	if magic != FormatText {
		return magic, nil
	}
	return FormatText, nil
}

// detectFormatByExt maps common extensions to FileFormat. Unknown
// extensions (and files without an extension) default to FormatText.
func detectFormatByExt(path string) FileFormat {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return FormatPDF
	case ".docx":
		return FormatDOCX
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return FormatImage
	case ".xlsx", ".xls":
		return FormatSpreadsheet
	case ".csv":
		return FormatCSV
	}
	return FormatText
}

// detectFormatByMagic reads the leading bytes of path to infer format
// when the extension is uninformative. Returns FormatText for anything
// that doesn't match a known signature.
func detectFormatByMagic(path string) (FileFormat, error) {
	f, err := os.Open(path)
	if err != nil {
		return FormatText, err
	}
	defer f.Close()

	head := make([]byte, 512)
	n, _ := f.Read(head)
	head = head[:n]

	switch {
	case len(head) >= 4 && string(head[:4]) == "%PDF":
		return FormatPDF, nil
	case len(head) >= 8 && head[0] == 0x89 && head[1] == 0x50 && head[2] == 0x4E && head[3] == 0x47:
		return FormatImage, nil
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF: // JPEG
		return FormatImage, nil
	case len(head) >= 6 && (string(head[:6]) == "GIF87a" || string(head[:6]) == "GIF89a"):
		return FormatImage, nil
	case len(head) >= 4 && head[0] == 'P' && head[1] == 'K' && head[2] == 0x03 && head[3] == 0x04:
		// ZIP container — could be DOCX or XLSX; distinguish by scanning the
		// zip directory. Done by the readers themselves.
		if zipContains(path, "word/document.xml") {
			return FormatDOCX, nil
		}
		if zipContains(path, "xl/") {
			return FormatSpreadsheet, nil
		}
	}
	return FormatText, nil
}

// readText reads a text file, returning line-numbered output bounded by
// offset (1-based start line) and limit (max lines). Lines past
// maxBytes in total size are truncated with a marker.
func readText(path string, offset, limit, maxBytes int) (*ToolExecResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil {
		return nil, err
	}
	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		truncated = true
	}

	lines := strings.Split(string(data), "\n")
	// Drop trailing empty line when file ended with \n.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	start := 0
	if offset > 0 {
		start = offset - 1
	}
	if start > len(lines) {
		start = len(lines)
	}
	end := len(lines)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&b, "%d\t%s\n", i+1, lines[i])
	}
	if truncated {
		fmt.Fprintf(&b, "[truncated at %d bytes]\n", maxBytes)
	}
	return SuccessResult(strings.TrimRight(b.String(), "\n"), "text"), nil
}

// readCSVFile reads a CSV file and formats it as a tab-separated text
// table. Header row (if present) is emitted verbatim; subsequent rows
// follow. Capped at maxBytes worth of raw file data.
func readCSVFile(path string, maxBytes int) (*ToolExecResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(maxBytes)+1)
	reader := csv.NewReader(limited)
	reader.FieldsPerRecord = -1 // permissive: uneven columns ok

	var rows [][]string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv: %w", err)
		}
		rows = append(rows, row)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d row(s):\n", len(rows))
	for _, row := range rows {
		b.WriteString(strings.Join(row, "\t"))
		b.WriteByte('\n')
	}
	return SuccessResult(strings.TrimRight(b.String(), "\n"), "text"), nil
}

// readImage returns the image as base64 with a MIME header line. The
// tool emits OutputType "image" so callers can distinguish the
// envelope. Vision-capability gating is the caller's job — this tool
// always emits the bytes when the file is readable.
func readImage(path string, maxBytes int) (*ToolExecResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && len(data) > maxBytes {
		return ErrorResult("image too large: %d bytes (cap %d)", len(data), maxBytes), nil
	}
	mime := http.DetectContentType(data)
	encoded := base64.StdEncoding.EncodeToString(data)
	out := fmt.Sprintf("%s\n%s", mime, encoded)
	return &ToolExecResult{Status: StatusSuccess, Output: out, OutputType: "image"}, nil
}

// readPDF extracts text from a PDF file. Delegates to read_pdf.go
// which wraps ledongthuc/pdf (BSD-3) — a pure text-extraction lib
// compatible with modeltap's Apache 2.0 license.
func readPDF(path, pages string) (*ToolExecResult, error) {
	return readPDFFile(path, pages)
}

// readDOCX extracts text from a DOCX file. Delegates to read_docx.go
// which implements the extractor using archive/zip + encoding/xml so
// the package has no heavy document-library dep.
func readDOCX(path string) (*ToolExecResult, error) {
	return readDOCXFile(path)
}

// readXLSX reads a spreadsheet as a text table. Delegates to
// read_xlsx.go which owns the excelize/v2 integration.
func readXLSX(path string) (*ToolExecResult, error) {
	return readXLSXFile(path)
}

// zipContains reports whether the ZIP at path contains any entry whose
// name starts with prefix. Returns false on any error — detection is a
// best-effort path and real parsing happens in the format readers.
func zipContains(path, prefix string) bool {
	r, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, prefix) {
			return true
		}
	}
	return false
}
