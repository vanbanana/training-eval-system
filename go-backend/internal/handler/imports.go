package handler

import (
	"io"
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/service"
)

type ImportsHandler struct {
	svc     *service.ImportService
	userSvc *service.UserService
	taskSvc *service.TaskService
}

func NewImportsHandler(svc *service.ImportService, userSvc *service.UserService, taskSvc *service.TaskService) *ImportsHandler {
	return &ImportsHandler{svc: svc, userSvc: userSvc, taskSvc: taskSvc}
}

// maxImportFileSize caps the in-memory import upload at 10MB.
const maxImportFileSize = 10 << 20

func (h *ImportsHandler) ImportUsers(w http.ResponseWriter, r *http.Request) {
	h.handleImport(w, r, "user")
}

func (h *ImportsHandler) ImportStudents(w http.ResponseWriter, r *http.Request) {
	h.handleImport(w, r, "student")
}

func (h *ImportsHandler) handleImport(w http.ResponseWriter, r *http.Request, jobType string) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := r.ParseMultipartForm(maxImportFileSize); err != nil {
		Error(w, http.StatusBadRequest, "Invalid multipart form or file too large")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "Missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImportFileSize))
	if err != nil {
		Error(w, http.StatusBadRequest, "Failed to read file")
		return
	}

	result, err := h.svc.ImportUsers(r.Context(), claims.Sub, jobType, header.Filename, data, h.userSvc)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	status := "done"
	if result.FailedCount > 0 && result.SuccessCount == 0 {
		status = "failed"
	}
	JSON(w, http.StatusOK, dto.ImportResultResponse{
		JobID:        result.JobID,
		TotalCount:   result.TotalCount,
		SuccessCount: result.SuccessCount,
		FailedCount:  result.FailedCount,
		Status:       status,
	})
}

func (h *ImportsHandler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	data, err := service.BuildUserTemplateXLSX()
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to build template")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=user_template.xlsx")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ImportTasks handles POST /api/imports/tasks — bulk task creation from xlsx/csv.
func (h *ImportsHandler) ImportTasks(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := r.ParseMultipartForm(maxImportFileSize); err != nil {
		Error(w, http.StatusBadRequest, "Invalid multipart form or file too large")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "Missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImportFileSize))
	if err != nil {
		Error(w, http.StatusBadRequest, "Failed to read file")
		return
	}

	result, err := h.svc.ImportTasks(r.Context(), claims.Sub, header.Filename, data, h.taskSvc)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	status := "done"
	if result.FailedCount > 0 && result.SuccessCount == 0 {
		status = "failed"
	}
	JSON(w, http.StatusOK, dto.ImportResultResponse{
		JobID:        result.JobID,
		TotalCount:   result.TotalCount,
		SuccessCount: result.SuccessCount,
		FailedCount:  result.FailedCount,
		Status:       status,
	})
}

// DownloadTaskTemplate handles GET /api/imports/template/task.xlsx.
func (h *ImportsHandler) DownloadTaskTemplate(w http.ResponseWriter, r *http.Request) {
	data, err := service.BuildTaskTemplateXLSX()
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to build task template")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=task_template.xlsx")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
