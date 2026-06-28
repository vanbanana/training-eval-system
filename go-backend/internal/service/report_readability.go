package service

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ReadabilityResult holds the analysis of a text chunk.
type ReadabilityResult struct {
	IsReadable bool      `json:"is_readable"`
	Warnings   []string  `json:"warnings,omitempty"`
	CleanText  string    `json:"clean_text,omitempty"`
	Sections   []Section `json:"sections,omitempty"`
}

// Section holds a titled section from structured text.
type Section struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// AnalyzeReadability checks whether rawText is human-readable.
func AnalyzeReadability(rawText string) ReadabilityResult {
	result := ReadabilityResult{CleanText: CleanText(rawText)}

	if !utf8.ValidString(rawText) {
		result.Warnings = append(result.Warnings, "invalid_utf8")
		result.CleanText = strings.ToValidUTF8(rawText, "")
	}

	// Count replacement characters
	replacementRune := '�'
	replacementCount := strings.Count(rawText, string(replacementRune))
	textLen := len([]rune(rawText))
	if textLen == 0 {
		return ReadabilityResult{IsReadable: false, Warnings: []string{"empty_text"}}
	}

	if float64(replacementCount)/float64(textLen) > 0.1 {
		result.Warnings = append(result.Warnings, "high_replacement_char_ratio")
		result.IsReadable = false
	} else if replacementCount > 5 {
		result.Warnings = append(result.Warnings, "partial_replacement_chars")
	}

	// Count control characters (keep \n, \t, \r)
	ctrlCount := 0
	for _, r := range rawText {
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			ctrlCount++
		}
	}
	if float64(ctrlCount)/float64(textLen) > 0.05 {
		result.Warnings = append(result.Warnings, "high_control_char_ratio")
		result.IsReadable = false
	}

	// Count printable chars
	printable := 0
	for _, r := range result.CleanText {
		if unicode.IsPrint(r) || r == '\n' || r == '\t' || r == '\r' {
			printable++
		}
	}
	if float64(printable)/float64(len([]rune(result.CleanText))) < 0.5 {
		result.Warnings = append(result.Warnings, "low_printable_ratio")
		result.IsReadable = false
	}

	if len(result.Warnings) == 0 {
		result.IsReadable = true
	}
	if result.IsReadable {
		// Drop binary-decoding garbage lines that survive UTF-8 validation
		// (legacy .doc extraction misreads embedded XML/binary as CJK).
		// Informational only: does not flip IsReadable.
		cleaned, garbled := stripGarbledLines(result.CleanText)
		result.CleanText = cleaned
		if garbled > 0 {
			result.Warnings = append(result.Warnings, "garbled_segments_removed")
		}
		result.Sections = ExtractSections(result.CleanText)
	}
	return result
}

// stripGarbledLines removes lines that are byte-decoding garbage produced by
// legacy binary .doc extraction (embedded XML/binary runs misread as CJK).
// It is conservative — a line is dropped only when it is pure CJK (no ASCII
// letters/digits, no spaces) AND shows a strong corruption signal:
//   - it contains CJK Extension-A characters (real modern Chinese text does not), or
//   - >=60% of its CJK characters byte-swap into printable ASCII (UTF-16LE misread).
//
// Real Chinese prose tops out around 38% byte-swappable, well below the threshold.
func stripGarbledLines(text string) (string, int) {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	dropped := 0
	for _, line := range lines {
		if isGarbledLine(line) {
			dropped++
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n"), dropped
}

func isGarbledLine(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" {
		return false
	}
	var cjk, extA, swappable, asciiWord int
	hasSpace := strings.ContainsRune(s, ' ')
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			asciiWord++
		case r >= 0x3400 && r <= 0x4DBF: // CJK Extension-A
			extA++
			cjk++
			if byteSwapsToASCII(r) {
				swappable++
			}
		case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
			cjk++
			if byteSwapsToASCII(r) {
				swappable++
			}
		}
	}
	// Signal A: CJK Extension-A density. Real modern Chinese text never uses
	// Ext-A, so even a 10% share among CJK chars is a corruption fingerprint —
	// this holds even when binary junk is interspersed with ASCII/punctuation.
	if extA >= 1 && float64(extA)/float64(cjk) >= 0.1 {
		return true
	}
	if cjk == 0 || asciiWord > 0 || hasSpace {
		return false
	}
	// Signal B: pure-CJK line where most chars byte-swap to printable ASCII
	// (UTF-16LE text read in the wrong byte order, e.g. embedded OOXML).
	if float64(swappable)/float64(cjk) >= 0.6 {
		return true
	}
	return false
}

// byteSwapsToASCII reports whether a BMP rune, with its two bytes swapped,
// becomes two printable ASCII bytes (the fingerprint of UTF-16LE text read in
// the wrong byte order).
func byteSwapsToASCII(r rune) bool {
	if r > 0xFFFF {
		return false
	}
	hi := byte(r >> 8)
	lo := byte(r & 0xFF)
	return lo >= 0x20 && lo <= 0x7E && hi >= 0x20 && hi <= 0x7E
}

// CleanText removes control chars, compresses blank lines.
func CleanText(rawText string) string {
	// Normalize line endings so line-based processing (sections, garbled-line
	// filtering) works — legacy .doc extraction often emits CR-only separators.
	rawText = strings.ReplaceAll(rawText, "\r\n", "\n")
	rawText = strings.ReplaceAll(rawText, "\r", "\n")

	cleaned := strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			return -1
		}
		return r
	}, rawText)

	// Compress 3+ consecutive newlines to 2
	for strings.Contains(cleaned, "\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(cleaned)
}

// ExtractSections splits text by common heading patterns.
func ExtractSections(rawText string) []Section {
	lines := strings.Split(rawText, "\n")
	var sections []Section
	var current Section

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isHeading(trimmed) {
			if current.Title != "" || current.Content != "" {
				current.Content = strings.TrimSpace(current.Content)
				sections = append(sections, current)
			}
			current = Section{Title: trimmed}
		} else {
			if current.Content != "" {
				current.Content += "\n"
			}
			current.Content += line
		}
	}

	if current.Title != "" || current.Content != "" {
		current.Content = strings.TrimSpace(current.Content)
		sections = append(sections, current)
	}

	if len(sections) == 0 && strings.TrimSpace(rawText) != "" {
		return []Section{{Content: strings.TrimSpace(rawText)}}
	}
	return sections
}

func isHeading(line string) bool {
	if len(line) > 100 {
		return false
	}
	// Check common heading patterns
	if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
		return true
	}
	if strings.HasPrefix(line, "实验") || strings.HasPrefix(line, "第") && strings.Contains(line, "章") {
		return true
	}
	if strings.HasPrefix(line, "一、") || strings.HasPrefix(line, "二、") || strings.HasPrefix(line, "三、") ||
		strings.HasPrefix(line, "四、") || strings.HasPrefix(line, "五、") || strings.HasPrefix(line, "六、") {
		return true
	}
	if strings.HasPrefix(line, "1.") && len(line) < 60 || strings.HasPrefix(line, "2.") && len(line) < 60 {
		return true
	}
	return false
}
