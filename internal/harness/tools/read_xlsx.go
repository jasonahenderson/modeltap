package tools

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// xlsxMaxCellsPerSheet caps per-sheet output to keep large spreadsheets
// from dominating tool output. Rows beyond the cap are dropped with a
// trailing marker; sheets are processed in workbook order.
const xlsxMaxCellsPerSheet = 500

// readXLSXFile extracts cells from every sheet in an XLSX workbook and
// formats them as tab-separated text tables, one section per sheet.
// Columns are left ragged — no alignment — because the output is
// consumed by an LLM that doesn't need pretty columns.
func readXLSXFile(path string) (*ToolExecResult, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("xlsx open: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return SuccessResult("empty workbook", "text"), nil
	}

	var b strings.Builder
	for si, name := range sheets {
		rows, err := f.GetRows(name)
		if err != nil {
			return nil, fmt.Errorf("xlsx rows %q: %w", name, err)
		}
		if si > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "# Sheet: %s (%d rows)\n", name, len(rows))
		cellCount := 0
		truncated := false
		for _, row := range rows {
			if cellCount+len(row) > xlsxMaxCellsPerSheet {
				truncated = true
				break
			}
			b.WriteString(strings.Join(row, "\t"))
			b.WriteByte('\n')
			cellCount += len(row)
		}
		if truncated {
			fmt.Fprintf(&b, "[truncated at %d cells]\n", xlsxMaxCellsPerSheet)
		}
	}
	return SuccessResult(strings.TrimRight(b.String(), "\n"), "text"), nil
}
