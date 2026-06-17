package report

import (
	"bytes"
	"fmt"
	"math"
	"sort"

	"github.com/go-pdf/fpdf"
)

// dimEntry pairs a dimension name with its score for chart rendering.
type dimEntry struct {
	Name  string
	Score float64
}

// ExportProfileReport generates a PDF document with student ability profile data.
func (p *PDFExporter) ExportProfileReport(data *ProfileReportData) ([]byte, error) {
	if len(data.RadarData) == 0 {
		return nil, fmt.Errorf("report: no profile data available for export")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetFontLocation("")
	pdf.SetAutoPageBreak(true, 20)

	chineseFont := "NotoSansSC"
	fontPath := getFontPath()
	if fontPath != "" {
		pdf.AddUTF8Font(chineseFont, "", fontPath)
		pdf.AddUTF8Font(chineseFont, "B", fontPath)
	} else {
		chineseFont = "Helvetica"
	}

	// ── Page 1: Title, Radar Chart, Dimension Table ──
	pdf.AddPage()

	// Title
	pdf.SetFont(chineseFont, "B", 18)
	if chineseFont == "NotoSansSC" {
		pdf.CellFormat(0, 12, "学生能力画像报告", "", 1, "C", false, 0, "")
	} else {
		pdf.CellFormat(0, 12, "Student Ability Profile", "", 1, "C", false, 0, "")
	}

	// Student info
	pdf.SetFont(chineseFont, "", 10)
	pdf.CellFormat(0, 7, fmt.Sprintf("Student: %s (ID: %d)", data.StudentName, data.StudentID), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("Evaluations: %d | Computed: %s", data.SourceEvaluationCount, data.ComputedAt), "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Sort dimensions by name for stable output
	dims := make([]dimEntry, 0, len(data.RadarData))
	for name, score := range data.RadarData {
		dims = append(dims, dimEntry{name, score})
	}
	sort.Slice(dims, func(i, j int) bool { return dims[i].Name < dims[j].Name })

	// Draw radar chart
	drawRadarChart(pdf, dims, chineseFont)
	pdf.Ln(5)

	// Dimension scores table
	pdf.SetFont(chineseFont, "B", 11)
	if chineseFont == "NotoSansSC" {
		pdf.CellFormat(0, 8, "各维度得分", "", 1, "L", false, 0, "")
	} else {
		pdf.CellFormat(0, 8, "Dimension Scores", "", 1, "L", false, 0, "")
	}

	pdf.SetFont(chineseFont, "B", 9)
	pdf.SetFillColor(67, 97, 238)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(120, 7, " "+dimLabel(chineseFont, "Dimension"), "1", 0, "L", true, 0, "")
	pdf.CellFormat(30, 7, dimLabel(chineseFont, "Score"), "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 7, dimLabel(chineseFont, "Level"), "1", 1, "C", true, 0, "")
	pdf.SetTextColor(0, 0, 0)

	pdf.SetFont(chineseFont, "", 9)
	for i, d := range dims {
		if i%2 == 0 {
			pdf.SetFillColor(245, 247, 255)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		level := scoreLevel(d.Score)
		pdf.CellFormat(120, 6, " "+d.Name, "1", 0, "L", true, 0, "")
		pdf.CellFormat(30, 6, fmt.Sprintf("%.1f", d.Score), "1", 0, "C", true, 0, "")
		pdf.CellFormat(30, 6, level, "1", 1, "C", true, 0, "")
	}

	// ── Page 2: Weaknesses & Suggestions + Score Trend ──
	pdf.AddPage()

	// Weaknesses section
	pdf.SetFont(chineseFont, "B", 11)
	if chineseFont == "NotoSansSC" {
		pdf.CellFormat(0, 8, "薄弱维度与学习建议", "", 1, "L", false, 0, "")
	} else {
		pdf.CellFormat(0, 8, "Weaknesses & Suggestions", "", 1, "L", false, 0, "")
	}

	if len(data.WeaknessList) == 0 {
		pdf.SetFont(chineseFont, "", 9)
		msg := "No weaknesses detected — all dimensions above 60."
		if chineseFont == "NotoSansSC" {
			msg = "各维度得分均在 60 分以上，暂无明显薄弱环节。"
		}
		pdf.CellFormat(0, 7, msg, "", 1, "L", false, 0, "")
	} else {
		for _, w := range data.WeaknessList {
			pdf.SetFont(chineseFont, "B", 9)
			pdf.SetFillColor(254, 242, 242)
			pdf.CellFormat(0, 7, fmt.Sprintf("  %s — %.1f", w.Name, w.Score), "", 1, "L", true, 0, "")

			if w.Suggestion != "" {
				pdf.SetFont(chineseFont, "", 8)
				pdf.SetFillColor(255, 255, 255)
				// MultiCell for potentially long suggestion text
				pdf.MultiCell(0, 5, "    "+w.Suggestion, "", "", false)
			}
			pdf.Ln(2)
		}
	}

	pdf.Ln(5)

	// Score trend chart
	if len(data.ScoreTrend) > 1 {
		pdf.SetFont(chineseFont, "B", 11)
		if chineseFont == "NotoSansSC" {
			pdf.CellFormat(0, 8, "成绩趋势", "", 1, "L", false, 0, "")
		} else {
			pdf.CellFormat(0, 8, "Score Trend", "", 1, "L", false, 0, "")
		}
		drawTrendChart(pdf, data.ScoreTrend, chineseFont)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("report: write profile pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func dimLabel(font, key string) string {
	if font == "NotoSansSC" {
		switch key {
		case "Dimension":
			return "维度"
		case "Score":
			return "得分"
		case "Level":
			return "等级"
		}
	}
	return key
}

func scoreLevel(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// drawRadarChart renders a polygon radar chart of dimension scores.
func drawRadarChart(pdf *fpdf.Fpdf, dims []dimEntry, font string) {
	n := len(dims)
	if n < 3 {
		return
	}

	cx, cy := 105.0, 130.0
	radius := 50.0
	angleStep := 2 * math.Pi / float64(n)

	// Draw grid rings
	for _, scale := range []float64{0.25, 0.5, 0.75, 1.0} {
		r := radius * scale
		pdf.SetDrawColor(200, 200, 200)
		pdf.SetLineWidth(0.2)
		for i := 0; i < n; i++ {
			a1 := -math.Pi/2 + float64(i)*angleStep
			a2 := -math.Pi/2 + float64(i+1)*angleStep
			pdf.Line(
				cx+math.Cos(a1)*r, cy+math.Sin(a1)*r,
				cx+math.Cos(a2)*r, cy+math.Sin(a2)*r,
			)
		}
	}

	// Draw axis lines
	pdf.SetDrawColor(180, 180, 180)
	for i := 0; i < n; i++ {
		a := -math.Pi/2 + float64(i)*angleStep
		pdf.Line(cx, cy, cx+math.Cos(a)*radius, cy+math.Sin(a)*radius)
	}

	// Draw data polygon
	pdf.SetDrawColor(67, 97, 238)
	pdf.SetLineWidth(0.6)
	pdf.SetFillColor(67, 97, 238)

	points := make([]fpdf.PointType, n)
	for i, d := range dims {
		a := -math.Pi/2 + float64(i)*angleStep
		r := (d.Score / 100.0) * radius
		if r > radius {
			r = radius
		}
		points[i] = fpdf.PointType{
			X: cx + math.Cos(a)*r,
			Y: cy + math.Sin(a)*r,
		}
	}
	pdf.Polygon(points, "FD")

	// Draw axis labels
	pdf.SetFont(font, "", 8)
	pdf.SetTextColor(80, 80, 80)
	for i, d := range dims {
		a := -math.Pi/2 + float64(i)*angleStep
		lx := cx + math.Cos(a)*(radius+8)
		ly := cy + math.Sin(a)*(radius+8)
		pdf.SetXY(lx-15, ly-3)
		pdf.CellFormat(30, 5, d.Name, "", 0, "C", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)
}

// drawTrendChart renders a simple polyline trend chart.
func drawTrendChart(pdf *fpdf.Fpdf, points []ProfileTrendPoint, font string) {
	if len(points) < 2 {
		return
	}

	startX, startY := 30.0, pdf.GetY()+5.0
	chartW, chartH := 150.0, 50.0

	// Find score range
	minScore, maxScore := points[0].Score, points[0].Score
	for _, pt := range points[1:] {
		if pt.Score < minScore {
			minScore = pt.Score
		}
		if pt.Score > maxScore {
			maxScore = pt.Score
		}
	}
	scoreRange := maxScore - minScore
	if scoreRange < 1 {
		scoreRange = 1
	}

	// Draw axes
	pdf.SetDrawColor(180, 180, 180)
	pdf.SetLineWidth(0.3)
	pdf.Line(startX, startY, startX, startY+chartH)
	pdf.Line(startX, startY+chartH, startX+chartW, startY+chartH)

	// Draw score labels
	pdf.SetFont(font, "", 7)
	pdf.SetXY(startX-15, startY-2)
	pdf.CellFormat(14, 4, fmt.Sprintf("%.0f", maxScore), "", 0, "R", false, 0, "")
	pdf.SetXY(startX-15, startY+chartH-2)
	pdf.CellFormat(14, 4, fmt.Sprintf("%.0f", minScore), "", 0, "R", false, 0, "")

	// Draw data line
	pdf.SetDrawColor(67, 97, 238)
	pdf.SetLineWidth(0.6)

	stepX := chartW / float64(len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		x1 := startX + float64(i)*stepX
		y1 := startY + chartH - ((points[i].Score-minScore)/scoreRange)*chartH
		x2 := startX + float64(i+1)*stepX
		y2 := startY + chartH - ((points[i+1].Score-minScore)/scoreRange)*chartH
		pdf.Line(x1, y1, x2, y2)
	}

	// Draw data points and period labels
	pdf.SetFillColor(67, 97, 238)
	pdf.SetFont(font, "", 6)
	for i, pt := range points {
		x := startX + float64(i)*stepX
		y := startY + chartH - ((pt.Score-minScore)/scoreRange)*chartH
		pdf.Circle(x, y, 1.5, "F")

		// Period label below X axis
		label := pt.Period
		if len(label) > 8 {
			label = label[:8]
		}
		pdf.SetXY(x-12, startY+chartH+1)
		pdf.CellFormat(24, 4, label, "", 0, "C", false, 0, "")
	}
}
