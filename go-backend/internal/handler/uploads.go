package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/pipeline"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"

	"context"
)

type UploadsHandler struct {
	svc  *service.UploadService
	orch *pipeline.Orchestrator
}

func NewUploadsHandler(svc *service.UploadService, orch *pipeline.Orchestrator) *UploadsHandler {
	return &UploadsHandler{svc: svc, orch: orch}
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
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	upload, err := h.svc.Upload(r.Context(), taskID, claims.Sub, header.Filename, header.Size, file)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Trigger parse pipeline asynchronously after successful upload
	if h.orch != nil {
		uploadID := upload.ID
		go func() {
			bgCtx := context.Background()
			if err := h.orch.TriggerParse(bgCtx, uploadID); err != nil {
				slog.Error("failed to trigger parse pipeline", "upload_id", uploadID, "error", err)
			}
		}()
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
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	upload, err := h.svc.GetByID(r.Context(), id)
	if err != nil || upload == nil {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}
	// Ownership check: students can only verify their own uploads
	if claims.Role == "student" && upload.StudentID != claims.Sub {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}
	vr, err := h.svc.GetVerifyResult(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if vr == nil {
		Error(w, http.StatusNotFound, "Verify result not available")
		return
	}
	JSON(w, http.StatusOK, vr)
}

func (h *UploadsHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid upload ID")
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	upload, err := h.svc.GetByID(r.Context(), id)
	if err != nil || upload == nil {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}
	// Ownership check: students can only retry their own uploads
	if claims.Role == "student" && upload.StudentID != claims.Sub {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}
	if upload.ParseStatus != "failed" {
		Error(w, http.StatusConflict, "Can only retry failed uploads")
		return
	}

	// Re-trigger parse pipeline (pipeline will reset status to pending internally)
	if h.orch != nil {
		go func() {
			bgCtx := context.Background()
			if err := h.orch.TriggerRetry(bgCtx, id); err != nil {
				slog.Error("failed to trigger retry pipeline", "upload_id", id, "error", err)
			}
		}()
	}

	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Retry submitted"})
}
