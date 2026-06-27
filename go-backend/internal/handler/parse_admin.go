package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/pipeline"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/vision"
)

// ParseAdminHandler provides the parse management dashboard for admins.
type ParseAdminHandler struct {
	uploadRepo repository.UploadRepo
	taskRepo   repository.TaskRepo
	evalRepo   repository.EvaluationRepo
	orch       *pipeline.Orchestrator
	glmAPIKey  string
}

func NewParseAdminHandler(
	uploadRepo repository.UploadRepo,
	taskRepo repository.TaskRepo,
	evalRepo repository.EvaluationRepo,
	orch *pipeline.Orchestrator,
	glmAPIKey string,
) *ParseAdminHandler {
	return &ParseAdminHandler{
		uploadRepo: uploadRepo,
		taskRepo:   taskRepo,
		evalRepo:   evalRepo,
		orch:       orch,
		glmAPIKey:  glmAPIKey,
	}
}

// Dashboard returns parse pipeline status summary.
func (h *ParseAdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Count by status
	type statusCount struct {
		status string
		count  int
	}
	var counts []statusCount
	for _, s := range []string{"pending", "parsing", "parsed", "failed"} {
		st := s
		list, _, err := h.uploadRepo.List(ctx, repository.UploadListParams{
			ParseStatus: &st,
			ListParams:  repository.ListParams{Page: 1, PageSize: 1000},
		})
		if err != nil {
			continue
		}
		counts = append(counts, statusCount{s, len(list)})
	}

	byStatus := map[string]int{}
	total := 0
	for _, c := range counts {
		byStatus[c.status] = c.count
		total += c.count
	}

	// Get failed uploads for retry
	failedSt := "failed"
	failedUploads, _, _ := h.uploadRepo.List(ctx, repository.UploadListParams{
		ParseStatus: &failedSt,
		ListParams:  repository.ListParams{Page: 1, PageSize: 200},
	})

	// Get pending uploads
	pendingSt := "pending"
	pendingUploads, _, _ := h.uploadRepo.List(ctx, repository.UploadListParams{
		ParseStatus: &pendingSt,
		ListParams:  repository.ListParams{Page: 1, PageSize: 200},
	})

	JSON(w, http.StatusOK, map[string]any{
		"status": map[string]any{
			"glm_enabled":    h.glmAPIKey != "",
			"tools":          vision.ToolsAvailable(),
			"by_status":      byStatus,
			"total_uploads":  total,
		},
		"failed_uploads":  uploadsToDTO(failedUploads),
		"pending_uploads": uploadsToDTO(pendingUploads),
	})
}

// RetryFailed retries a single failed upload.
func (h *ParseAdminHandler) RetryFailed(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 upload ID")
		return
	}

	if err := h.orch.TriggerRetry(r.Context(), id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]string{"status": "retry_triggered"})
}

// RetryAllFailed retries all failed uploads.
func (h *ParseAdminHandler) RetryAllFailed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	failedSt := "failed"
	failedUploads, _, err := h.uploadRepo.List(ctx, repository.UploadListParams{
		ParseStatus: &failedSt,
		ListParams:  repository.ListParams{Page: 1, PageSize: 200},
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	queued := 0
	for _, u := range failedUploads {
		if err := h.orch.TriggerRetry(ctx, u.ID); err != nil {
			slog.Warn("retry failed", "upload_id", u.ID, "error", err)
			continue
		}
		queued++
	}

	JSON(w, http.StatusOK, map[string]any{
		"status": "batch_retry",
		"total":  len(failedUploads),
		"queued": queued,
	})
}

func uploadsToDTO(uploads []model.Upload) []map[string]any {
	result := make([]map[string]any, 0, len(uploads))
	for _, u := range uploads {
		result = append(result, map[string]any{
			"id":          u.ID,
			"student_id":  u.StudentID,
			"task_id":     u.TaskID,
			"filename":    u.Filename,
			"file_type":   u.FileType,
			"file_size":   u.FileSize,
			"status":      u.ParseStatus,
			"created_at":  u.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return result
}