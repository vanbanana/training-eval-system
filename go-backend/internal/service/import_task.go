package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/xuri/excelize/v2"
)

// TaskImportRow is a single parsed task import row.
type TaskImportRow struct {
	Name               string
	Description        string
	Requirements       string
	EvaluationCriteria string
	CourseID           int64
	Deadline           string // RFC3339 or "2006-01-02"
}

// ImportTasks parses an uploaded xlsx/csv file and creates training tasks.
func (s *ImportService) ImportTasks(ctx context.Context, operatorID int64, filename string, data []byte, taskSvc *TaskService) (*ImportResult, error) {
	rows, err := parseTaskImportFile(filename, data)
	if err != nil {
		return nil, err
	}

	job := &model.ImportJob{OperatorID: operatorID, JobType: "task", Status: "processing", TotalCount: len(rows)}
	if err := s.repo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("import_service: create job: %w", err)
	}

	result := &ImportResult{JobID: job.ID, TotalCount: len(rows)}
	for i, row := range rows {
		rowNum := i + 2
		if row.Name == "" {
			result.FailedCount++
			msg := "任务名称不能为空"
			result.Errors = append(result.Errors, ImportRowError{Row: rowNum, Message: msg})
			_ = s.repo.CreateRecord(ctx, &model.ImportRecord{JobID: job.ID, RowNumber: rowNum, Status: "failed", ErrorMessage: msg})
			continue
		}
		if row.CourseID == 0 {
			result.FailedCount++
			msg := "课程 ID 不能为空"
			result.Errors = append(result.Errors, ImportRowError{Row: rowNum, Message: msg})
			_ = s.repo.CreateRecord(ctx, &model.ImportRecord{JobID: job.ID, RowNumber: rowNum, Status: "failed", ErrorMessage: msg})
			continue
		}

		task := &model.TrainingTask{
			Name:               row.Name,
			Description:        row.Description,
			Requirements:       row.Requirements,
			EvaluationCriteria: row.EvaluationCriteria,
			TeacherID:          operatorID,
			CourseID:           row.CourseID,
			Status:             "draft",
		}

		if row.Deadline != "" {
			if t, err := parseDeadline(row.Deadline); err == nil {
				task.Deadline = &t
			}
		}

		if err := taskSvc.Create(ctx, task); err != nil {
			result.FailedCount++
			msg := fmt.Sprintf("创建失败: %s", err.Error())
			result.Errors = append(result.Errors, ImportRowError{Row: rowNum, Message: msg})
			_ = s.repo.CreateRecord(ctx, &model.ImportRecord{JobID: job.ID, RowNumber: rowNum, Status: "failed", ErrorMessage: msg})
			continue
		}
		result.SuccessCount++
		_ = s.repo.CreateRecord(ctx, &model.ImportRecord{JobID: job.ID, RowNumber: rowNum, Status: "success"})
	}

	job.Status = "done"
	job.SuccessCount = result.SuccessCount
	job.FailedCount = result.FailedCount
	now := time.Now()
	job.CompletedAt = &now
	if err := s.repo.Update(ctx, job); err != nil {
		slog.Warn("import_service: update task job failed", "job_id", job.ID, "error", err.Error())
	}

	return result, nil
}

func parseDeadline(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006/01/02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("无法解析日期: %s", s)
}

func parseTaskImportFile(filename string, data []byte) ([]TaskImportRow, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".xlsx"):
		return parseTaskXLSX(data)
	case strings.HasSuffix(lower, ".csv"):
		return parseTaskCSV(data)
	default:
		if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
			return parseTaskXLSX(data)
		}
		return parseTaskCSV(data)
	}
}

func parseTaskXLSX(data []byte) ([]TaskImportRow, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("无法读取 xlsx 文件: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx 文件没有工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取工作表失败: %w", err)
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("文件为空")
	}
	colIdx := mapTaskHeaders(rows[0])
	var out []TaskImportRow
	for _, cells := range rows[1:] {
		if isEmptyRow(cells) {
			continue
		}
		out = append(out, taskRowFromCells(cells, colIdx))
	}
	return out, nil
}

func parseTaskCSV(data []byte) ([]TaskImportRow, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("文件为空")
		}
		return nil, fmt.Errorf("读取 csv 失败: %w", err)
	}
	colIdx := mapTaskHeaders(header)
	var out []TaskImportRow
	for {
		cells, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取 csv 行失败: %w", err)
		}
		if isEmptyRow(cells) {
			continue
		}
		out = append(out, taskRowFromCells(cells, colIdx))
	}
	return out, nil
}

func mapTaskHeaders(header []string) map[string]int {
	idx := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		switch key {
		case "name", "任务名称", "名称":
			idx["name"] = i
		case "description", "描述", "任务描述":
			idx["description"] = i
		case "requirements", "要求", "任务要求":
			idx["requirements"] = i
		case "evaluation_criteria", "评价标准", "评分标准":
			idx["evaluation_criteria"] = i
		case "course_id", "课程id", "课程":
			idx["course_id"] = i
		case "deadline", "截止时间", "截止日期":
			idx["deadline"] = i
		}
	}
	return idx
}

func taskRowFromCells(cells []string, colIdx map[string]int) TaskImportRow {
	get := func(key string) string {
		if i, ok := colIdx[key]; ok && i < len(cells) {
			return strings.TrimSpace(cells[i])
		}
		return ""
	}
	var courseID int64
	if s := get("course_id"); s != "" {
		courseID, _ = strconv.ParseInt(s, 10, 64)
	}
	return TaskImportRow{
		Name:               get("name"),
		Description:        get("description"),
		Requirements:       get("requirements"),
		EvaluationCriteria: get("evaluation_criteria"),
		CourseID:           courseID,
		Deadline:           get("deadline"),
	}
}

// BuildTaskTemplateXLSX returns an xlsx template for task import.
func BuildTaskTemplateXLSX() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	headers := []string{"name", "description", "requirements", "evaluation_criteria", "course_id", "deadline"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	example := []string{"Python 基础实训", "完成一个简单的 Python 程序", "使用 Python 3.10+", "代码质量、功能正确性", "1", "2026-12-31"}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(sheet, cell, v)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
