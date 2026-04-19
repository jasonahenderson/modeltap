package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func writeXLSXFixture(t *testing.T, root string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	if err := f.SetCellValue(sheet, "A1", "name"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := f.SetCellValue(sheet, "B1", "score"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := f.SetCellValue(sheet, "A2", "Alice"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := f.SetCellValue(sheet, "B2", 42); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := f.SetCellValue(sheet, "A3", "Bob"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := f.SetCellValue(sheet, "B3", 7); err != nil {
		t.Fatalf("set: %v", err)
	}

	path := filepath.Join(root, "book.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	return path
}

func TestRead_XLSX(t *testing.T) {
	root := t.TempDir()
	path := writeXLSXFixture(t, root)

	r := NewReadTool(root, NewFileTracker())
	res, err := r.Execute(context.Background(), readInput(t, map[string]any{"file_path": path}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q err=%q", res.Status, res.Error)
	}
	for _, want := range []string{"Sheet1", "name", "score", "Alice", "42", "Bob", "7"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("missing %q in XLSX output:\n%s", want, res.Output)
		}
	}
}
