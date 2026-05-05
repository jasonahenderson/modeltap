package tools

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

// pdfAutoPageLimit is the page-count threshold above which callers must
// supply an explicit `pages` range. The design (D6.2) caps a single
// read at 20 pages and requires a range for anything over 10 pages to
// nudge the model toward deliberate paging.
const (
	pdfAutoPageLimit = 10
	pdfMaxPagesRange = 20
)

// readPDFFile extracts plain text from a PDF. When `pages` is empty
// and the document has more than pdfAutoPageLimit pages, the reader
// refuses with a message telling the caller to supply a range.
func readPDFFile(path, pages string) (*ToolExecResult, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("pdf open: %w", err)
	}
	defer f.Close()

	total := r.NumPage()
	start, end, err := resolvePDFRange(pages, total)
	if err != nil {
		return ErrorResult("%v", err), nil
	}
	if pages == "" && total > pdfAutoPageLimit {
		return ErrorResult(
			"pdf has %d pages; pass `pages` (e.g. \"1-10\") to read a specific range (max %d pages per call)",
			total, pdfMaxPagesRange,
		), nil
	}

	var b strings.Builder
	for i := start; i <= end; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		txt, err := page.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("pdf page %d: %w", i, err)
		}
		if i > start {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "# Page %d\n", i)
		b.WriteString(txt)
	}

	fullText := strings.TrimRight(b.String(), "\n")
	if fullText == "" {
		// A PDF with no extractable text (e.g. image-only scans) isn't
		// an error — surface it so the caller can fall back to other
		// approaches.
		fullText = "[no extractable text]"
	}
	return SuccessResult(fullText, "text"), nil
}

// resolvePDFRange parses a user-supplied `pages` expression into a
// 1-indexed, inclusive [start, end] pair clamped to [1, total].
// Accepted forms: "", "3", "1-5", "10-20". Anything else is an error.
// Ranges larger than pdfMaxPagesRange are capped.
func resolvePDFRange(expr string, total int) (int, int, error) {
	if total == 0 {
		return 0, 0, fmt.Errorf("pdf has no pages")
	}
	if expr == "" {
		end := total
		if end > pdfMaxPagesRange {
			end = pdfMaxPagesRange
		}
		return 1, end, nil
	}
	if strings.Contains(expr, "-") {
		parts := strings.SplitN(expr, "-", 2)
		s, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return 0, 0, fmt.Errorf("pages: invalid start %q", parts[0])
		}
		e, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, fmt.Errorf("pages: invalid end %q", parts[1])
		}
		if s < 1 || e < s {
			return 0, 0, fmt.Errorf("pages: bad range %q", expr)
		}
		if s > total {
			return 0, 0, fmt.Errorf("pages: start %d exceeds total %d", s, total)
		}
		if e > total {
			e = total
		}
		if e-s+1 > pdfMaxPagesRange {
			e = s + pdfMaxPagesRange - 1
		}
		return s, e, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(expr))
	if err != nil {
		return 0, 0, fmt.Errorf("pages: %q is not a page number or range", expr)
	}
	if n < 1 || n > total {
		return 0, 0, fmt.Errorf("pages: %d out of range [1, %d]", n, total)
	}
	return n, n, nil
}
