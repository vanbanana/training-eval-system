// Package parser implements document text extraction (DOCX, PDF, OCR).
package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ParseDocx extracts text content from a DOCX file.
func ParseDocx(reader io.ReaderAt, size int64) (string, error) {
	r, err := zip.NewReader(reader, size)
	if err != nil {
		return "", fmt.Errorf("parser: open docx zip: %w", err)
	}

	// Find word/document.xml
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("parser: open document.xml: %w", err)
			}
			defer rc.Close()
			return extractDocxText(rc)
		}
	}
	return "", fmt.Errorf("parser: word/document.xml not found in docx")
}

func extractDocxText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var text strings.Builder
	var inText bool

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parser: decode xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			} else if t.Name.Local == "p" {
				// New paragraph
				if text.Len() > 0 {
					text.WriteString("\n")
				}
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				text.Write(t)
			}
		}
	}

	return text.String(), nil
}

// ParseDocxBytes is a convenience wrapper for in-memory DOCX data.
func ParseDocxBytes(data []byte) (string, error) {
	return ParseDocx(bytes.NewReader(data), int64(len(data)))
}
