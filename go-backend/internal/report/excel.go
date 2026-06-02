package report

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// ExcelExporter generates .xlsx reports.
type ExcelExporter struct{}

// ExportTaskReport generates an Excel workbook with score distribution and student breakdown.
func (e *ExcelExporter) ExportTaskReport(data *TaskReportData) ([]byte, error) {
	if len(data.Students) == 0 {
		return nil, fmt.Errorf("report: no student data available for export")
	}

	f := excelize.NewFile()
	defer f.Close()

	// Sheet 1: Score Distribution
	distSheet := "成绩分布"
	f.SetSheetName("Sheet1", distSheet)

	dist := ComputeDistribution(data.Students)

	// Headers
	f.SetCellValue(distSheet, "A1", "分数段")
	f.SetCellValue(distSheet, "B1", "人数")

	// Data rows
	ranges := []struct {
		label string
		count int
	}{
		{"0-59", dist.Range0to59},
		{"60-69", dist.Range60to69},
		{"70-79", dist.Range70to79},
		{"80-89", dist.Range80to89},
		{"90-100", dist.Range90to100},
	}
	for i, r := range ranges {
		row := i + 2
		f.SetCellValue(distSheet, fmt.Sprintf("A%d", row), r.label)
		f.SetCellValue(distSheet, fmt.Sprintf("B%d", row), r.count)
	}

	// Add bar chart
	chart := &excelize.Chart{
		Type: excelize.Bar,
		Series: []excelize.ChartSeries{
			{
				Name:       fmt.Sprintf("%s!$B$1", distSheet),
				Categories: fmt.Sprintf("%s!$A$2:$A$6", distSheet),
				Values:     fmt.Sprintf("%s!$B$2:$B$6", distSheet),
			},
		},
		Title: []excelize.RichTextRun{{Text: "成绩分布"}},
	}
	f.AddChart(distSheet, "D2", chart)

	// Sheet 2: Student Detail
	detailSheet := "学生明细"
	f.NewSheet(detailSheet)

	// Headers
	headers := []string{"姓名", "总分"}
	for _, d := range data.Dimensions {
		headers = append(headers, d.Name)
	}
	headers = append(headers, "教师评语")

	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(detailSheet, cell, h)
	}

	// Student rows
	for row, s := range data.Students {
		r := row + 2
		f.SetCellValue(detailSheet, fmt.Sprintf("A%d", r), s.StudentName)
		f.SetCellValue(detailSheet, fmt.Sprintf("B%d", r), s.TotalScore)

		for col, d := range data.Dimensions {
			cell, _ := excelize.CoordinatesToCellName(col+3, r)
			score := s.DimensionScores[d.ID]
			f.SetCellValue(detailSheet, cell, score)
		}

		lastCol := len(data.Dimensions) + 3
		cell, _ := excelize.CoordinatesToCellName(lastCol, r)
		f.SetCellValue(detailSheet, cell, s.TeacherComment)
	}

	// Write to buffer
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("report: write excel: %w", err)
	}

	return buf.Bytes(), nil
}
