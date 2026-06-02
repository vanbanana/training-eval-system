package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ParsePDF extracts text content from a PDF file.
func ParsePDF(reader *bytes.Reader, size int64) (string, error) {
	r, err := pdf.NewReader(reader, size)
	if err != nil {
		return "", fmt.Errorf("parser: open pdf: %w", err)
	}

	var text strings.Builder
	numPages := r.NumPage()

	for i := 1; i <= numPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if err != nil {
			// Skip pages that fail to parse
			continue
		}
		text.WriteString(content)
		text.WriteString("\n")
	}

	return strings.TrimSpace(text.String()), nil
}

// ParsePDFBytes is a convenience wrapper for in-memory PDF data.
func ParsePDFBytes(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	return ParsePDF(reader, int64(len(data)))
}
