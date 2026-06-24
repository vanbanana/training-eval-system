package parser

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf16"
)

// ParseDoc extracts text content from a legacy .doc (OLE2/CFBF) binary file.
// This is a best-effort extraction that looks for readable text in the binary stream.
func ParseDoc(data []byte) (string, error) {
	if len(data) < 512 {
		return "", fmt.Errorf("parser: file too small to be a valid .doc")
	}

	// Check OLE2 magic bytes
	if !bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return "", fmt.Errorf("parser: not a valid OLE2 .doc file")
	}

	// Try to extract UTF-16LE text from the Word Document stream
	text := extractUTF16Text(data)
	if text != "" {
		return text, nil
	}

	// Fallback: extract printable ASCII text
	return extractASCIIText(data), nil
}

// extractUTF16Text tries to extract UTF-16LE encoded text from .doc binary data.
func extractUTF16Text(data []byte) string {
	var result strings.Builder
	i := 0
	for i < len(data)-1 {
		lo := data[i]
		hi := data[i+1]
		r := rune(uint16(lo) | uint16(hi)<<8)

		if hi == 0 && lo >= 0x20 && lo < 0x7F {
			result.WriteRune(r)
			i += 2
			continue
		}

		if r >= 0x4E00 && r <= 0x9FFF {
			result.WriteRune(r)
			i += 2
			continue
		}

		if r >= 0x3000 && r <= 0x303F {
			result.WriteRune(r)
			i += 2
			continue
		}

		if hi == 0 && (lo == '\r' || lo == '\n') {
			if result.Len() > 0 {
				last := result.String()
				if !strings.HasSuffix(last, "\n") {
					result.WriteString("\n")
				}
			}
			i += 2
			continue
		}

		i += 2
	}

	cleaned := cleanExtractedText(result.String())
	if len(cleaned) < 10 {
		return ""
	}
	return cleaned
}

// extractASCIIText extracts printable ASCII sequences from binary data.
func extractASCIIText(data []byte) string {
	var result strings.Builder
	var current strings.Builder

	for _, b := range data {
		if b >= 0x20 && b < 0x7F {
			current.WriteByte(b)
		} else if b == '\n' || b == '\r' || b == '\t' {
			if current.Len() > 3 {
				result.WriteString(current.String())
				result.WriteString("\n")
			}
			current.Reset()
		} else {
			if current.Len() > 3 {
				result.WriteString(current.String())
				result.WriteString("\n")
			}
			current.Reset()
		}
	}

	if current.Len() > 3 {
		result.WriteString(current.String())
	}

	return cleanExtractedText(result.String())
}

func cleanExtractedText(s string) string {
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

// ExtractDocImages extracts embedded images from a .doc (OLE2) file.
// Returns a slice of raw image byte slices.
func ExtractDocImages(data []byte) [][]byte {
	if len(data) < 512 {
		return nil
	}

	var images [][]byte

	for i := 0; i < len(data)-8; i++ {
		// JPEG signature: FF D8 FF
		if data[i] == 0xFF && data[i+1] == 0xD8 && data[i+2] == 0xFF {
			end := findJPEGEnd(data[i:])
			if end > 0 {
				images = append(images, data[i:i+end])
				i += end - 1
			}
		}

		// PNG signature: 89 50 4E 47 0D 0A 1A 0A
		if i+7 < len(data) && data[i] == 0x89 && data[i+1] == 0x50 && data[i+2] == 0x4E && data[i+3] == 0x47 {
			end := findPNGEnd(data[i:])
			if end > 0 {
				images = append(images, data[i:i+end])
				i += end - 1
			}
		}
	}

	return images
}

func findJPEGEnd(data []byte) int {
	for i := 2; i < len(data)-1; i++ {
		if data[i] == 0xFF && data[i+1] == 0xD9 {
			return i + 2
		}
	}
	return 0
}

func findPNGEnd(data []byte) int {
	iend := []byte{0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82}
	for i := 8; i < len(data)-7; i++ {
		if bytes.Equal(data[i:i+8], iend) {
			return i + 8
		}
	}
	return 0
}

// Ensure utf16 is available for potential future use.
var _ = utf16.Decode
