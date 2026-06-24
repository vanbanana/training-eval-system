package report

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-pdf/fpdf"
)

// defaultFontPath is the path to the Chinese TTF font used for PDF generation.
// Set via environment variable TES_PDF_FONT_PATH, or downloaded by `make setup-fonts`.
const defaultFontPath = "./fonts/NotoSansSC-Regular.ttf"

// getFontPath returns the path to the Chinese TTF font file, or empty if not available.
func getFontPath() string {
	path := os.Getenv("TES_PDF_FONT_PATH")
	if path != "" {
		return path
	}
	if _, err := os.Stat(defaultFontPath); err == nil {
		return defaultFontPath
	}
	return ""
}

// PDFExporter generates .pdf reports with optional Chinese text support.
type PDFExporter struct{}

// ExportTaskReport generates a PDF document with score distribution and student table.
func (p *PDFExporter) ExportTaskReport(data *TaskReportData) ([]byte, error) {
	if len(data.Students) == 0 {
		return nil, fmt.Errorf("report: no student data available for export")
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetFontLocation("")

	// Try to register Chinese font; fall back to Helvetica if not available
	chineseFont := "NotoSansSC"
	fontPath := getFontPath()
	if fontPath != "" {
		pdf.AddUTF8Font(chineseFont, "", fontPath)
		pdf.AddUTF8Font(chineseFont, "B", fontPath)
		slog.Info("pdf: Chinese font loaded", "path", fontPath)
	} else {
		chineseFont = "Helvetica"
		slog.Warn("pdf: Chinese font not found, using Helvetica (Chinese text will not render). "+
			"Run 'make setup-fonts' or set TES_PDF_FONT_PATH", "path", defaultFontPath)
	}

	// Page 1: Title + Score Distribution Chart
	pdf.AddPage()
	if chineseFont == "NotoSansSC" {
		pdf.SetFont(chineseFont, "B", 16)
		pdf.CellFormat(0, 12, fmt.Sprintf("评价报告: %s", data.TaskName), "", 1, "C", false, 0, "")
		pdf.SetFont(chineseFont, "", 10)
		pdf.CellFormat(0, 8, fmt.Sprintf("课程: %s | 学生人数: %d", data.CourseName, len(data.Students)), "", 1, "C", false, 0, "")
	} else {
		pdf.SetFont(chineseFont, "B", 16)
		pdf.CellFormat(0, 12, fmt.Sprintf("Report: %s", data.TaskName), "", 1, "C", false, 0, "")
		pdf.SetFont(chineseFont, "", 10)
		pdf.CellFormat(0, 8, fmt.Sprintf("Course: %s | Students: %d", data.CourseName, len(data.Students)), "", 1, "C", false, 0, "")
	}
	pdf.Ln(10)

	// Draw score distribution bar chart
	dist := ComputeDistribution(data.Students)
	drawBarChart(pdf, dist, len(data.Students), chineseFont)

	// Page 2: Student table
	pdf.AddPage()
	pdf.SetFont(chineseFont, "B", 12)
	if chineseFont == "NotoSansSC" {
		pdf.CellFormat(0, 10, "学生成绩表", "", 1, "L", false, 0, "")
	} else {
		pdf.CellFormat(0, 10, "Student Scores", "", 1, "L", false, 0, "")
	}
	pdf.SetFont(chineseFont, "", 9)

	colWidths := []float64{40, 20}
	headers := []string{"Name", "Total"}
	for _, d := range data.Dimensions {
		colWidths = append(colWidths, 25)
		headers = append(headers, d.Name)
	}

	pdf.SetFont(chineseFont, "B", 8)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont(chineseFont, "", 8)
	for _, s := range data.Students {
		pdf.CellFormat(colWidths[0], 6, s.StudentName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 6, fmt.Sprintf("%.1f", s.TotalScore), "1", 0, "C", false, 0, "")
		for i, d := range data.Dimensions {
			score := s.DimensionScores[d.ID]
			pdf.CellFormat(colWidths[i+2], 6, fmt.Sprintf("%.0f", score), "1", 0, "C", false, 0, "")
		}
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("report: write pdf: %w", err)
	}

	return buf.Bytes(), nil
}

func drawBarChart(pdf *fpdf.Fpdf, dist ScoreDistribution, total int, font string) {
	barLabels := []string{"0-59", "60-69", "70-79", "80-89", "90-100"}
	counts := []int{dist.Range0to59, dist.Range60to69, dist.Range70to79, dist.Range80to89, dist.Range90to100}

	maxCount := 1
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	startX := 50.0
	startY := 50.0
	barWidth := 30.0
	maxHeight := 60.0
	gap := 10.0

	pdf.SetFont(font, "B", 10)
	pdf.SetXY(startX, startY-15)
	if font == "NotoSansSC" {
		pdf.CellFormat(200, 8, "分数分布", "", 1, "L", false, 0, "")
	} else {
		pdf.CellFormat(200, 8, "Score Distribution", "", 1, "L", false, 0, "")
	}

	for i := range barLabels {
		x := startX + float64(i)*(barWidth+gap)
		height := (float64(counts[i]) / float64(maxCount)) * maxHeight
		if height < 2 {
			height = 2
		}
		y := startY + maxHeight - height

		pdf.SetFillColor(67, 97, 238)
		pdf.Rect(x, y, barWidth, height, "F")

		pdf.SetFont(font, "B", 8)
		pdf.SetXY(x, y-6)
		pdf.CellFormat(barWidth, 5, fmt.Sprintf("%d", counts[i]), "", 0, "C", false, 0, "")

		pdf.SetFont(font, "", 7)
		pdf.SetXY(x, startY+maxHeight+2)
		pdf.CellFormat(barWidth, 5, barLabels[i], "", 0, "C", false, 0, "")
	}
}

