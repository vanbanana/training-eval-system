package handler

import (
	"net/http"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
)

type UploadsHandler struct{ svc *service.UploadService }

func NewUploadsHandler(svc *service.UploadService) *UploadsHandler {
	return &UploadsHandler{svc: svc}
}

func (h *UploadsHandler) ListByTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "taskId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	params := repository.UploadListParams{ListParams: repository.ListParams{Page: 1, PageSize: 100}}
	params.TaskID = &taskID
	claims := middleware.GetClaims(r.Context())
	if claims != nil && claims.Role == "student" {
		params.StudentID = &claims.Sub
	}
	uploads, _, err := h.svc.List(r.Context(), params)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.UploadResponse, 0, len(uploads))
	for _, u := range uploads {
		items = append(items, dto.UploadResponse{
			ID: u.ID, TaskID: u.TaskID, StudentID: u.StudentID, Filename: u.Filename,
			FileType: u.FileType, FileSize: u.FileSize, SHA256: u.SHA256,
			ParseStatus: u.ParseStatus, Version: u.Version,
			CreatedAt: u.CreatedAt.Format(time.RFC3339), UpdatedAt: u.UpdatedAt.Format(time.RFC3339),
		})
	}
	JSON(w, http.StatusOK, items)
}

func (h *UploadsHandler) Upload(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "taskId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		Error(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "File is required")
		return
	}
	defer file.Close()

	claims := middleware.GetClaims(r.Context())
	upload, err := h.svc.Upload(r.Context(), taskID, claims.Sub, header.Filename, header.Size, file)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	JSON(w, http.StatusCreated, dto.UploadResponse{
		ID: upload.ID, TaskID: upload.TaskID, StudentID: upload.StudentID,
		Filename: upload.Filename, FileType: upload.FileType, FileSize: upload.FileSize,
		SHA256: upload.SHA256, ParseStatus: upload.ParseStatus, Version: upload.Version,
		CreatedAt: upload.CreatedAt.Format(time.RFC3339), UpdatedAt: upload.UpdatedAt.Format(time.RFC3339),
	})
}

func (h *UploadsHandler) VerifyResult(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid upload ID")
		return
	}
	upload, err := h.svc.GetByID(r.Context(), id)
	if err != nil || upload == nil {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}
	// Return 404 if no verify result exists yet
	Error(w, http.StatusNotFound, "Verify result not available")
}

func (h *UploadsHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid upload ID")
		return
	}
	upload, err := h.svc.GetByID(r.Context(), id)
	if err != nil || upload == nil {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}
	if upload.ParseStatus != "failed" {
		Error(w, http.StatusConflict, "Can only retry failed uploads")
		return
	}
	// Reset status to pending for re-processing
	if err := h.svc.UpdateStatus(r.Context(), id, "pending"); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Retry submitted"})
}
