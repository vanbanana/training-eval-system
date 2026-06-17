package service

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ReadabilityResult holds the analysis of a text chunk.
type ReadabilityResult struct {
	IsReadable bool     `json:"is_readable"`
	Warnings   []string `json:"warnings,omitempty"`
	CleanText  string   `json:"clean_text,omitempty"`
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
		result.Sections = ExtractSections(result.CleanText)
	}
	return result
}

// CleanText removes control chars, compresses blank lines.
func CleanText(rawText string) string {
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
