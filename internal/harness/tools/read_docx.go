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
// docxMaxDecompressedBytes bounds how much of word/document.xml the
// reader will decompress. DOCX is a ZIP container; a 1 KB crafted
// `.docx` can expand to gigabytes via a zip bomb. WU-094 H-15.
// 50 MiB is well above realistic document sizes (largest observed
// in the wild is ~15 MiB of plain XML) while keeping the harness
// safe from adversarial inputs.
const docxMaxDecompressedBytes = 50 * 1024 * 1024

func readDOCXFile(path string) (*ToolExecResult, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("docx open: %w", err)
	}
	defer zr.Close()

	var docXML io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			// Reject the entry up front if its declared
			// uncompressed size already exceeds the cap — avoids
			// streaming through megabytes of decompression on an
			// obvious bomb.
			if f.UncompressedSize64 > docxMaxDecompressedBytes {
				return ErrorResult("docx document.xml too large: %d bytes (cap %d)",
					f.UncompressedSize64, docxMaxDecompressedBytes), nil
			}
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

	// Wrap with LimitReader so a lying header (declared small,
	// actual bomb) still hits the cap at read time.
	bounded := io.LimitReader(docXML, docxMaxDecompressedBytes+1)
	text, err := extractDocxText(bounded)
	if err != nil {
		return nil, fmt.Errorf("docx extract: %w", err)
	}
	if len(text) >= docxMaxDecompressedBytes {
		return ErrorResult("docx content exceeded decompression cap (%d bytes)",
			docxMaxDecompressedBytes), nil
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
