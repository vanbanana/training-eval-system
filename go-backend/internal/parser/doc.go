package parser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

// ParseDoc extracts text from a legacy .doc (OLE2/CFB) binary file.
// It attempts to find UTF-16LE encoded text runs within the binary data,
// which is how Word stores text in the old .doc format.
func ParseDoc(data []byte) (string, error) {
	// Strategy: scan the binary data for sequences of valid UTF-16LE characters.
	// Word .doc files store text as UTF-16LE in the WordDocument stream.
	// We look for runs of printable UTF-16LE characters and concatenate them.

	var result []byte
	i := 0
	dataLen := len(data)

	for i < dataLen-1 {
		// Read a UTF-16LE code unit
		r16 := binary.LittleEndian.Uint16(data[i : i+2])

		// Check if this is a printable character in UTF-16LE
		if isPrintableUTF16(r16) {
			// Start collecting a run of printable UTF-16LE characters
			runStart := i
			runEnd := i + 2

			for runEnd < dataLen-1 {
				nextR16 := binary.LittleEndian.Uint16(data[runEnd : runEnd+2])
				if isPrintableUTF16(nextR16) {
					runEnd += 2
				} else if nextR16 == 0x000D || nextR16 == 0x000A {
					// CR or LF
					runEnd += 2
				} else {
					break
				}
			}

			// Only include runs of sufficient length (at least 8 chars = 16 bytes)
			// to avoid random binary noise. Short runs of CJK characters from OLE2
			// binary structures (like "撘抛撘抛") are typically 2-6 chars.
			runLen := (runEnd - runStart) / 2
			if runLen >= 8 {
				// Decode UTF-16LE to UTF-8
				decoded := decodeUTF16LERun(data[runStart:runEnd])
				if len(decoded) > 0 {
					result = append(result, decoded...)
					result = append(result, '\n')
				}
			}

			i = runEnd
		} else {
			i += 2
		}
	}

	text := string(result)

	// Post-process: clean up common OLE2 binary artifacts
	text = cleanDocText(text)

	// If we got very little text, try a different approach:
	// scan for ASCII text runs (some .doc files have ASCII text too)
	if len(text) < 50 {
		asciiText := extractASCIIText(data)
		if len(asciiText) > len(text) {
			text = asciiText
		}
	}

	if len(text) == 0 {
		return "", fmt.Errorf("parser: could not extract text from .doc file")
	}

	return text, nil
}

// isPrintableUTF16 checks if a UTF-16 code unit represents a printable character.
func isPrintableUTF16(r uint16) bool {
	// CJK Unified Ideographs and other common CJK ranges
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// Common punctuation and symbols
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	// Basic Latin letters and digits
	if (r >= 0x0041 && r <= 0x005A) || (r >= 0x0061 && r <= 0x007A) || (r >= 0x0030 && r <= 0x0039) {
		return true
	}
	// Basic punctuation
	if r >= 0x0020 && r <= 0x007E {
		return true
	}
	// Fullwidth forms
	if r >= 0xFF01 && r <= 0xFF5E {
		return true
	}
	// Chinese punctuation
	if r >= 0xFF00 && r <= 0xFFEF {
		return true
	}
	return false
}

// decodeUTF16LERun decodes a byte slice of UTF-16LE data to UTF-8.
func decodeUTF16LERun(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}

	runes := make([]uint16, len(data)/2)
	for i := range runes {
		runes[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}

	// Convert []uint16 to []rune using utf16.Decode
	u16s := make([]uint16, 0, len(runes))
	for _, r := range runes {
		// Skip surrogates for simplicity (they're rare in .doc text)
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		// Skip null characters
		if r == 0 {
			continue
		}
		u16s = append(u16s, r)
	}

	decoded := utf16.Decode(u16s)

	// Convert to UTF-8
	buf := make([]byte, 4)
	var result bytes.Buffer
	for _, r := range decoded {
		n := utf8.EncodeRune(buf, r)
		result.Write(buf[:n])
	}

	return result.String()
}

// cleanDocText removes common OLE2 binary artifacts from extracted .doc text.
// These include short runs of CJK Extension-A characters that are actually
// binary data misinterpreted as text (e.g., "撘抛撘抛", "肉观观观观", "㔀脈䩃4䩏").
func cleanDocText(text string) string {
	var result bytes.Buffer
	lines := bytes.Split([]byte(text), []byte("\n"))
	for _, line := range lines {
		lineStr := string(line)
		if isLikelyArtifact(lineStr) {
			continue
		}
		if len(result.Bytes()) > 0 {
			result.WriteByte('\n')
		}
		result.Write(line)
	}
	return string(result.Bytes())
}

// isLikelyArtifact checks if a line is likely a binary artifact rather than real text.
func isLikelyArtifact(line string) bool {
	if len(line) == 0 {
		return false
	}

	var cjkExtA, commonCJK, ascii int
	for _, r := range line {
		switch {
		case r >= 0x3400 && r <= 0x4DBF: // CJK Extension A (rare, usually artifacts)
			cjkExtA++
		case r >= 0x4E00 && r <= 0x9FFF: // Common CJK
			commonCJK++
		case (r >= 0x0041 && r <= 0x005A) || (r >= 0x0061 && r <= 0x007A) || (r >= 0x0030 && r <= 0x0039):
			ascii++
		}
	}

	total := len([]rune(line))
	if total == 0 {
		return false
	}

	// If more than 40% of characters are CJK Extension A, it's likely an artifact
	if float64(cjkExtA)/float64(total) > 0.4 {
		return true
	}

	// If the line is short (<= 6 chars) and contains any CJK Extension A, skip it
	if total <= 6 && cjkExtA > 0 {
		return true
	}

	// If the line has no common CJK and no ASCII, but has CJK Extension A, skip it
	if commonCJK == 0 && ascii == 0 && cjkExtA > 0 {
		return true
	}

	return false
}

// extractASCIIText extracts ASCII text runs from binary data as a fallback.
func extractASCIIText(data []byte) string {
	var result bytes.Buffer
	var currentRun bytes.Buffer

	for _, b := range data {
		if b >= 0x20 && b < 0x7F {
			currentRun.WriteByte(b)
		} else if b == 0x0A || b == 0x0D {
			if currentRun.Len() > 3 {
				result.Write(currentRun.Bytes())
				result.WriteByte('\n')
			}
			currentRun.Reset()
		} else {
			if currentRun.Len() > 3 {
				result.Write(currentRun.Bytes())
				result.WriteByte(' ')
			}
			currentRun.Reset()
		}
	}
	if currentRun.Len() > 3 {
		result.Write(currentRun.Bytes())
	}

	return result.String()
}

// ExtractDocImages extracts embedded PNG and JPEG images from a .doc (OLE2/CFB) binary file.
// It scans for PNG and JPEG file signatures and extracts the complete image data.
// Returns a slice of image data bytes and their MIME types.
func ExtractDocImages(data []byte) [][]byte {
	var images [][]byte
	dataLen := len(data)

	// Extract PNG images (signature: 89 50 4E 47 0D 0A 1A 0A, end: 49 45 4E 44 AE 42 60 82)
	i := 0
	for i < dataLen-8 {
		if data[i] == 0x89 && data[i+1] == 0x50 && data[i+2] == 0x4E && data[i+3] == 0x47 &&
			data[i+4] == 0x0D && data[i+5] == 0x0A && data[i+6] == 0x1A && data[i+7] == 0x0A {
			// Found PNG signature, find IEND chunk
			end := findPNGEnd(data[i:])
			if end > 0 {
				imgData := make([]byte, end)
				copy(imgData, data[i:i+end])
				images = append(images, imgData)
				i += end
				continue
			}
		}
		// Extract JPEG images (signature: FF D8 FF, end: FF D9)
		if data[i] == 0xFF && data[i+1] == 0xD8 && data[i+2] == 0xFF {
			end := findJPEGEnd(data[i:])
			if end > 0 {
				imgData := make([]byte, end)
				copy(imgData, data[i:i+end])
				images = append(images, imgData)
				i += end
				continue
			}
		}
		i++
	}

	return images
}

// findPNGEnd finds the end of a PNG file by locating the IEND chunk.
func findPNGEnd(data []byte) int {
		if len(data) < 8 {
			return -1
		}
		// Skip PNG signature (8 bytes)
		i := int64(8)
		dataLen := int64(len(data))
		for i < dataLen-12 {
			// Read chunk length (4 bytes big-endian) — use int64 to avoid 32-bit overflow
			chunkLen := int64(binary.BigEndian.Uint32(data[i : i+4]))
			if chunkLen > dataLen-i {
				break
			}

			chunkType := string(data[i+4 : i+8])

			// Chunk total = 4 (length) + 4 (type) + chunkLen (data) + 4 (CRC)
			chunkTotal := int64(4 + 4 + 4) + chunkLen
			if chunkTotal > dataLen-i {
				// Chunk extends beyond data, something is wrong
				break
			}

			if chunkType == "IEND" {
				return int(i + chunkTotal)
			}
			i += chunkTotal
		}
		return -1
	}

// findJPEGEnd finds the end of a JPEG file by locating the EOI marker (FF D9).
func findJPEGEnd(data []byte) int {
	if len(data) < 4 {
		return -1
	}
	// Skip the SOI marker (FF D8)
	i := 2
	for i < len(data)-1 {
		if data[i] == 0xFF {
			// Check for EOI marker
			if data[i+1] == 0xD9 {
				return i + 2
			}
			// Skip non-marker bytes (0xFF00 is escaped FF)
			if data[i+1] == 0x00 {
				i += 2
				continue
			}
			// For SOS marker, skip until next marker
			if data[i+1] == 0xDA {
				i += 2
				for i < len(data)-1 {
					if data[i] == 0xFF && data[i+1] != 0x00 {
						break
					}
					i++
				}
				continue
			}
			// Read segment length for other markers
			if i+3 < len(data) && data[i+1] >= 0xC0 && data[i+1] <= 0xFE {
				segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
				i += 2 + segLen
				continue
			}
			i += 2
		} else {
			i++
		}
	}
	return -1
}
