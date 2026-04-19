package tools

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// readDOCXFile extracts plain text from a DOCX file. DOCX is a ZIP
// container; the narrative content lives in `word/document.xml` as an
// Office Open XML WordProcessingML stream. This reader walks the XML
// events directly (via encoding/xml.Decoder.Token) so we only depend
// on the standard library — no UniDoc/unioffice, no AGPL/commercial
// licensing concerns.
//
// Extraction rules:
//   - Text inside <w:t> elements is appended verbatim (including
//     preserved whitespace when `xml:space="preserve"` is set).
//   - <w:br/> and <w:tab/> emit `\n` and `\t` respectively.
//   - End of <w:p> (paragraph) emits `\n`, and end of <w:tbl> (table)
//     adds a trailing blank line to keep tables visually separated.
//   - Everything else (styling, comments, revision metadata) is
//     ignored.
func readDOCXFile(path string) (*ToolExecResult, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("docx open: %w", err)
	}
	defer zr.Close()

	var docXML io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("docx open document.xml: %w", err)
			}
			docXML = rc
			break
		}
	}
	if docXML == nil {
		return nil, fmt.Errorf("docx: missing word/document.xml")
	}
	defer docXML.Close()

	text, err := extractDocxText(docXML)
	if err != nil {
		return nil, fmt.Errorf("docx extract: %w", err)
	}
	return SuccessResult(text, "text"), nil
}

func extractDocxText(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var out strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return "", err
				}
				out.WriteString(s)
			case "tab":
				out.WriteByte('\t')
			case "br":
				out.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				out.WriteByte('\n')
			case "tbl":
				out.WriteString("\n\n")
			}
		}
	}
	// Collapse 3+ consecutive newlines into 2.
	s := out.String()
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimRight(s, "\n"), nil
}
