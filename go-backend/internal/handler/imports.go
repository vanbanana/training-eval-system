package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/service"
	"github.com/xuri/excelize/v2"
)

type ImportsHandler struct {
	svc     *service.ImportService
	userSvc *service.UserService
	taskSvc *service.TaskService
}

func NewImportsHandler(svc *service.ImportService, userSvc *service.UserService, taskSvc *service.TaskService) *ImportsHandler {
	return &ImportsHandler{svc: svc, userSvc: userSvc, taskSvc: taskSvc}
}

const maxImportFileSize = 10 << 20

// DownloadTemplate generates and serves a user import XLSX template.
func (h *ImportsHandler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	headers := []string{"username", "display_name", "role", "password"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	// Example row
	examples := []string{"student001", "张三", "student", "Pass@1234"}
	for i, v := range examples {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, v)
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=user_import_template.xlsx")
	if err := f.Write(w); err != nil {
		slog.Error("write template xlsx", "error", err.Error())
	}
}

// ImportUsers handles POST /api/imports/users — CSV or XLSX upload.
func (h *ImportsHandler) ImportUsers(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	if err := r.ParseMultipartForm(maxImportFileSize); err != nil {
		Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "Missing file field")
		return
	}
	defer file.Close()

	var rows [][]string
	filename := strings.ToLower(header.Filename)

	switch {
	case strings.HasSuffix(filename, ".csv"):
		rows, err = parseCSVFile(file)
	case strings.HasSuffix(filename, ".xlsx") || strings.HasSuffix(filename, ".xls"):
		rows, err = parseXLSXFile(file)
	default:
		Error(w, http.StatusBadRequest, "Unsupported file format. Use .csv or .xlsx")
		return
	}
	if err != nil {
		Error(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse file: %s", err.Error()))
		return
	}

	if len(rows) < 2 {
		Error(w, http.StatusBadRequest, "File has no data rows")
		return
	}

	// Create import job
	job := &model.ImportJob{
		OperatorID: claims.Sub,
		JobType:    "users",
		Status:     "processing",
		TotalCount: len(rows) - 1, // exclude header
		CreatedAt:  time.Now(),
	}

	successCount := 0
	failedCount := 0

	// Process each data row (skip header)
	for i, row := range rows[1:] {
		rowNum := i + 2 // 1-indexed, skip header
		if len(row) < 4 {
			failedCount++
			slog.Warn("import row: insufficient columns", "row", rowNum)
			continue
		}

		username := strings.TrimSpace(row[0])
		displayName := strings.TrimSpace(row[1])
		role := strings.TrimSpace(row[2])
		password := strings.TrimSpace(row[3])

		if username == "" || displayName == "" || role == "" || password == "" {
			failedCount++
			slog.Warn("import row: empty required fields", "row", rowNum)
			continue
		}

		// Validate role
		if role != "admin" && role != "teacher" && role != "student" {
			failedCount++
			slog.Warn("import row: invalid role", "row", rowNum, "role", role)
			continue
		}

		user := &model.User{
			Username:    username,
			DisplayName: displayName,
			Role:        role,
		}
		if err := h.userSvc.Create(r.Context(), user, password); err != nil {
			failedCount++
			slog.Warn("import row: create user failed", "row", rowNum, "error", err.Error())
			continue
		}
		successCount++
	}

	job.SuccessCount = successCount
	job.FailedCount = failedCount
	job.Status = "done"
	now := time.Now()
	job.CompletedAt = &now

	// Persist import job (best-effort — don't fail the response)
	if err := h.svc.CreateJob(r.Context(), job); err != nil {
		slog.Warn("import job persist failed", "error", err.Error())
	}

	JSON(w, http.StatusOK, dto.ImportResultResponse{
		JobID:        job.ID,
		TotalCount:   job.TotalCount,
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Status:       "done",
	})
}

// ImportStudents handles POST /api/imports/students — CSV or XLSX upload.
func (h *ImportsHandler) ImportStudents(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	if err := r.ParseMultipartForm(maxImportFileSize); err != nil {
		Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "Missing file field")
		return
	}
	defer file.Close()

	var rows [][]string
	filename := strings.ToLower(header.Filename)

	switch {
	case strings.HasSuffix(filename, ".csv"):
		rows, err = parseCSVFile(file)
	case strings.HasSuffix(filename, ".xlsx") || strings.HasSuffix(filename, ".xls"):
		rows, err = parseXLSXFile(file)
	default:
		Error(w, http.StatusBadRequest, "Unsupported file format. Use .csv or .xlsx")
		return
	}
	if err != nil {
		Error(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse file: %s", err.Error()))
		return
	}

	if len(rows) < 2 {
		Error(w, http.StatusBadRequest, "File has no data rows")
		return
	}

	job := &model.ImportJob{
		OperatorID: claims.Sub,
		JobType:    "students",
		Status:     "processing",
		TotalCount: len(rows) - 1,
		CreatedAt:  time.Now(),
	}

	successCount := 0
	failedCount := 0

	for _, row := range rows[1:] {
		if len(row) < 4 {
			failedCount++
			continue
		}

		username := strings.TrimSpace(row[0])
		displayName := strings.TrimSpace(row[1])
		password := strings.TrimSpace(row[3])

		if username == "" || displayName == "" {
			failedCount++
			continue
		}

		user := &model.User{
			Username:    username,
			DisplayName: displayName,
			Role:        "student",
		}
		if err := h.userSvc.Create(r.Context(), user, password); err != nil {
			failedCount++
			continue
		}
		successCount++
	}

	job.SuccessCount = successCount
	job.FailedCount = failedCount
	job.Status = "done"
	now := time.Now()
	job.CompletedAt = &now

	if err := h.svc.CreateJob(r.Context(), job); err != nil {
		slog.Warn("import job persist failed", "error", err.Error())
	}

	JSON(w, http.StatusOK, dto.ImportResultResponse{
		JobID:        job.ID,
		TotalCount:   job.TotalCount,
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Status:       "done",
	})
}

// parseCSVFile reads a CSV file and returns rows including header.
func parseCSVFile(r io.Reader) ([][]string, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	return reader.ReadAll()
}

// parseXLSXFile reads an XLSX file and returns rows including header.
func parseXLSXFile(r io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, fmt.Errorf("no sheets in xlsx")
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}
	return rows, nil
}
