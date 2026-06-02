package report

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExcel_ValidData(t *testing.T) {
	data := &TaskReportData{
		TaskName:   "并发编程实训",
		CourseName: "软件工程实训",
		Dimensions: []DimensionInfo{{ID: 1, Name: "代码规范"}, {ID: 2, Name: "功能完整"}},
		Students: []StudentReportRow{
			{StudentName: "张三", TotalScore: 85.5, DimensionScores: map[int64]float64{1: 80, 2: 90}, TeacherComment: "Good"},
			{StudentName: "李四", TotalScore: 72.0, DimensionScores: map[int64]float64{1: 70, 2: 74}},
			{StudentName: "王五", TotalScore: 55.0, DimensionScores: map[int64]float64{1: 50, 2: 60}},
		},
	}

	exporter := &ExcelExporter{}
	xlsxBytes, err := exporter.ExportTaskReport(data)
	if err != nil {
		t.Fatalf("ExportTaskReport failed: %v", err)
	}
	if len(xlsxBytes) == 0 {
		t.Fatal("empty xlsx output")
	}

	// Verify it's a valid xlsx
	f, err := excelize.OpenReader(strings.NewReader(string(xlsxBytes)))
	if err != nil {
		t.Fatalf("invalid xlsx: %v", err)
	}
	defer f.Close()

	// Check sheets exist
	sheets := f.GetSheetList()
	if len(sheets) < 2 {
		t.Fatalf("expected 2 sheets, got %d: %v", len(sheets), sheets)
	}
	if sheets[0] != "成绩分布" {
		t.Errorf("expected sheet '成绩分布', got '%s'", sheets[0])
	}
	if sheets[1] != "学生明细" {
		t.Errorf("expected sheet '学生明细', got '%s'", sheets[1])
	}

	// Check student detail rows (header + 3 students)
	rows, err := f.GetRows("学生明细")
	if err != nil {
		t.Fatalf("get rows: %v", err)
	}
	if len(rows) != 4 { // 1 header + 3 data rows
		t.Errorf("expected 4 rows in 学生明细, got %d", len(rows))
	}
}

func TestExcel_EmptyData(t *testing.T) {
	data := &TaskReportData{
		TaskName: "空任务",
		Students: nil,
	}

	exporter := &ExcelExporter{}
	_, err := exporter.ExportTaskReport(data)
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
}

func TestExcel_ScoreDistribution(t *testing.T) {
	data := &TaskReportData{
		TaskName:   "Test",
		Dimensions: []DimensionInfo{{ID: 1, Name: "D1"}},
		Students: []StudentReportRow{
			{StudentName: "A", TotalScore: 55, DimensionScores: map[int64]float64{1: 55}},
			{StudentName: "B", TotalScore: 65, DimensionScores: map[int64]float64{1: 65}},
			{StudentName: "C", TotalScore: 75, DimensionScores: map[int64]float64{1: 75}},
			{StudentName: "D", TotalScore: 85, DimensionScores: map[int64]float64{1: 85}},
			{StudentName: "E", TotalScore: 95, DimensionScores: map[int64]float64{1: 95}},
		},
	}

	dist := ComputeDistribution(data.Students)
	if dist.Range0to59 != 1 || dist.Range60to69 != 1 || dist.Range70to79 != 1 || dist.Range80to89 != 1 || dist.Range90to100 != 1 {
		t.Errorf("unexpected distribution: %+v", dist)
	}
}

func TestPDF_ValidData(t *testing.T) {
	data := &TaskReportData{
		TaskName:   "Test Task",
		CourseName: "SE101",
		Dimensions: []DimensionInfo{{ID: 1, Name: "Quality"}, {ID: 2, Name: "Completeness"}},
		Students: []StudentReportRow{
			{StudentName: "Student A", TotalScore: 88.5, DimensionScores: map[int64]float64{1: 85, 2: 92}},
			{StudentName: "Student B", TotalScore: 72.0, DimensionScores: map[int64]float64{1: 70, 2: 74}},
		},
	}

	exporter := &PDFExporter{}
	pdfBytes, err := exporter.ExportTaskReport(data)
	if err != nil {
		t.Fatalf("ExportTaskReport failed: %v", err)
	}
	if len(pdfBytes) < 1000 {
		t.Fatalf("PDF too small: %d bytes", len(pdfBytes))
	}
	// Verify PDF magic bytes
	if string(pdfBytes[:4]) != "%PDF" {
		t.Fatalf("not a PDF file, starts with: %q", string(pdfBytes[:4]))
	}
}

func TestPDF_EmptyData(t *testing.T) {
	data := &TaskReportData{TaskName: "Empty", Students: nil}
	exporter := &PDFExporter{}
	_, err := exporter.ExportTaskReport(data)
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
}

func TestComputeDistribution_AllRanges(t *testing.T) {
	students := []StudentReportRow{
		{TotalScore: 0}, {TotalScore: 30}, {TotalScore: 59.9},     // 0-59
		{TotalScore: 60}, {TotalScore: 69.9},                       // 60-69
		{TotalScore: 70}, {TotalScore: 79},                         // 70-79
		{TotalScore: 80}, {TotalScore: 89.9},                       // 80-89
		{TotalScore: 90}, {TotalScore: 100},                        // 90-100
	}
	dist := ComputeDistribution(students)
	if dist.Range0to59 != 3 {
		t.Errorf("Range0to59: expected 3, got %d", dist.Range0to59)
	}
	if dist.Range60to69 != 2 {
		t.Errorf("Range60to69: expected 2, got %d", dist.Range60to69)
	}
	if dist.Range70to79 != 2 {
		t.Errorf("Range70to79: expected 2, got %d", dist.Range70to79)
	}
	if dist.Range80to89 != 2 {
		t.Errorf("Range80to89: expected 2, got %d", dist.Range80to89)
	}
	if dist.Range90to100 != 2 {
		t.Errorf("Range90to100: expected 2, got %d", dist.Range90to100)
	}
}
