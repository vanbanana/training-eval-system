package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/xuri/excelize/v2"
)

// ImportResult holds the outcome of an import operation.
type ImportResult struct {
	JobID        int64
	TotalCount   int
	SuccessCount int
	FailedCount  int
	Errors       []ImportRowError
}

// ImportRowError describes a single failed import row.
type ImportRowError struct {
	Row     int
	Message string
}

// ImportUsers parses an uploaded xlsx/csv file and creates user accounts.
func (s *ImportService) ImportUsers(ctx context.Context, operatorID int64, jobType string, filename string, data []byte, userSvc *UserService) (*ImportResult, error) {
	rows, err := parseUserImportFile(filename, data)
	if err != nil {
		return nil, err
	}

	job := &model.ImportJob{OperatorID: operatorID, JobType: jobType, Status: "processing", TotalCount: len(rows)}
	if err := s.repo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("import_service: create job: %w", err)
	}

	result := &ImportResult{JobID: job.ID, TotalCount: len(rows)}
	for i, row := range rows {
		rowNum := i + 2
		if row.Username == "" {
			result.FailedCount++
			msg := "用户名不能为空"
			result.Errors = append(result.Errors, ImportRowError{Row: rowNum, Message: msg})
			_ = s.repo.CreateRecord(ctx, &model.ImportRecord{JobID: job.ID, RowNumber: rowNum, Status: "failed", ErrorMessage: msg})
			continue
		}

		role := row.Role
		if role == "" {
			if jobType == "student" {
				role = "student"
			} else {
				role = "student"
			}
		}

		password := row.Password
		if password == "" {
			password = "123456"
		}

		user := &model.User{
			Username:    row.Username,
			DisplayName: row.DisplayName,
			Role:        role,
			IsActive:    true,
		}

		if err := userSvc.Create(ctx, user, password); err != nil {
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
		slog.Warn("import_service: update job failed", "job_id", job.ID, "error", err.Error())
	}

	return result, nil
}

// UserImportRow is a single parsed user import row.
type UserImportRow struct {
	Username    string
	DisplayName string
	Role        string
	Password    string
}

func parseUserImportFile(filename string, data []byte) ([]UserImportRow, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".xlsx"):
		return parseUserXLSX(data)
	case strings.HasSuffix(lower, ".csv"):
		return parseUserCSV(data)
	default:
		if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
			return parseUserXLSX(data)
		}
		return parseUserCSV(data)
	}
}

func parseUserXLSX(data []byte) ([]UserImportRow, error) {
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
	colIdx := mapUserHeaders(rows[0])
	var out []UserImportRow
	for _, cells := range rows[1:] {
		if isEmptyRow(cells) {
			continue
		}
		out = append(out, userRowFromCells(cells, colIdx))
	}
	return out, nil
}

func parseUserCSV(data []byte) ([]UserImportRow, error) {
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
	colIdx := mapUserHeaders(header)
	var out []UserImportRow
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
		out = append(out, userRowFromCells(cells, colIdx))
	}
	return out, nil
}

func mapUserHeaders(header []string) map[string]int {
	idx := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		switch key {
		case "username", "用户名", "账号":
			idx["username"] = i
		case "display_name", "姓名", "名称":
			idx["display_name"] = i
		case "role", "角色":
			idx["role"] = i
		case "password", "密码":
			idx["password"] = i
		}
	}
	return idx
}

func userRowFromCells(cells []string, colIdx map[string]int) UserImportRow {
	get := func(key string) string {
		if i, ok := colIdx[key]; ok && i < len(cells) {
			return strings.TrimSpace(cells[i])
		}
		return ""
	}
	return UserImportRow{
		Username:    get("username"),
		DisplayName: get("display_name"),
		Role:        get("role"),
		Password:    get("password"),
	}
}

// isEmptyRow returns true if all cells in a row are empty or whitespace.
func isEmptyRow(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// BuildUserTemplateXLSX returns an xlsx template for user import.
func BuildUserTemplateXLSX() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	headers := []string{"username", "display_name", "role", "password"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	example := []string{"student01", "张三", "student", "123456"}
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

// ListParams re-export for import handler convenience.
var _ repository.ListParams
