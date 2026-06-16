package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
)

type EvaluationsHandler struct {
	svc       *service.EvaluationService
	taskSvc   *service.TaskService
	uploadSvc *service.UploadService
}

func NewEvaluationsHandler(svc *service.EvaluationService, taskSvc *service.TaskService, uploadSvc *service.UploadService) *EvaluationsHandler {
	return &EvaluationsHandler{svc: svc, taskSvc: taskSvc, uploadSvc: uploadSvc}
}

func (h *EvaluationsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid evaluation ID")
		return
	}
	eval, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "Evaluation not found")
		return
	}
	JSON(w, http.StatusOK, h.evalToFullDTO(r.Context(), eval))
}

func (h *EvaluationsHandler) GetMy(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	params := repository.EvalListParams{ListParams: repository.ListParams{Page: 1, PageSize: 100}}
	params.StudentID = &claims.Sub
	evals, _, err := h.svc.List(r.Context(), params)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.EvaluationResponse, 0, len(evals))
	for _, e := range evals {
		items = append(items, evalToDTO(&e))
	}
	JSON(w, http.StatusOK, items)
}

func (h *EvaluationsHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	uploadID, err := PathInt64(r, "uploadId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid upload ID")
		return
	}

	// Verify upload exists
	upload, err := h.uploadSvc.GetByID(r.Context(), uploadID)
	if err != nil || upload == nil {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}

	// Check for existing evaluation on this upload
	existingParams := repository.EvalListParams{ListParams: repository.ListParams{Page: 1, PageSize: 1}}
	existingParams.UploadID = &uploadID
	existing, _, _ := h.svc.List(r.Context(), existingParams)
	if len(existing) > 0 {
		Error(w, http.StatusConflict, "Evaluation already triggered for this upload")
		return
	}

	// Create a pending evaluation for this upload
	claims := middleware.GetClaims(r.Context())
	eval := &model.Evaluation{
		TaskID:    upload.TaskID,
		UploadID:  uploadID,
		StudentID: claims.Sub,
		Status:    "pending",
	}
	if err := h.svc.Create(r.Context(), eval); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.TriggerResponse{EvaluationID: eval.ID, TotalScore: 0})
}

func (h *EvaluationsHandler) BulkAction(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkActionRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	ctx := r.Context()
	switch req.Action {
	case "confirm":
		if err := h.svc.BatchConfirm(ctx, req.EvaluationIDs); err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "reject":
		for _, id := range req.EvaluationIDs {
			eval, err := h.svc.GetByID(ctx, id)
			if err != nil {
				continue
			}
			eval.Status = "rejected"
			if req.Reason != "" {
				eval.TeacherComment = req.Reason
			}
			_ = h.svc.Update(ctx, eval)
		}
	default:
		Error(w, http.StatusBadRequest, "Unknown action: "+req.Action)
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Bulk action completed"})
}

func (h *EvaluationsHandler) UpdateDimensionScore(w http.ResponseWriter, r *http.Request) {
	evalID, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid evaluation ID")
		return
	}
	dimID, err := PathInt64(r, "dimId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid dimension ID")
		return
	}

	var req dto.UpdateDimensionScoreRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate score range
	if req.SubjScore != nil && (*req.SubjScore < 0 || *req.SubjScore > 100) {
		Error(w, http.StatusBadRequest, "Score must be between 0 and 100")
		return
	}

	// Load evaluation with scores
	eval, err := h.svc.GetByID(r.Context(), evalID)
	if err != nil {
		Error(w, http.StatusNotFound, "Evaluation not found")
		return
	}

	// Find and update the dimension score
	var beforeScore *float64
	found := false
	for i, s := range eval.Scores {
		if s.DimensionID == dimID {
			beforeScore = s.TeacherScore
			eval.Scores[i].TeacherScore = req.SubjScore
			eval.Scores[i].Rationale = req.Comment
			found = true
			break
		}
	}

	if !found {
		// Create new score entry if dimension exists in task
		eval.Scores = append(eval.Scores, model.DimensionScore{
			EvaluationID: evalID,
			DimensionID:  dimID,
			TeacherScore: req.SubjScore,
			Rationale:    req.Comment,
		})
	}

	// Save all scores
	if err := h.svc.SaveScores(r.Context(), evalID, eval.Scores); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Append history
	claims := middleware.GetClaims(r.Context())
	_ = h.svc.AppendHistory(r.Context(), &model.EvaluationHistory{
		EvaluationID: evalID,
		OperatorID:   &claims.Sub,
		Action:       "teacher_scored",
		BeforeValue:  beforeScore,
		AfterValue:   req.SubjScore,
	})

	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Score updated"})
}

func (h *EvaluationsHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid evaluation ID")
		return
	}
	history, err := h.svc.GetHistory(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.EvaluationHistoryResp, 0, len(history))
	for _, h := range history {
		items = append(items, dto.EvaluationHistoryResp{
			ID:          h.ID,
			Action:      h.Action,
			BeforeValue: h.BeforeValue,
			AfterValue:  h.AfterValue,
			ChangedAt:   h.ChangedAt.Format(time.RFC3339),
			OperatorID:  h.OperatorID,
		})
	}
	JSON(w, http.StatusOK, items)
}

func evalToDTO(e *model.Evaluation) dto.EvaluationResponse {
	resp := dto.EvaluationResponse{
		ID: e.ID, TaskID: e.TaskID, StudentID: e.StudentID, UploadID: e.UploadID,
		Status: e.Status, TotalScore: e.TotalScore, TeacherComment: e.TeacherComment,
		CreatedAt: e.CreatedAt.Format(time.RFC3339), UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}
	for _, s := range e.Scores {
		resp.Scores = append(resp.Scores, dto.DimensionScoreResp{
			ID: s.ID, EvaluationID: s.EvaluationID, DimensionID: s.DimensionID,
			AIScore: s.AIScore, TeacherScore: s.TeacherScore, Rationale: s.Rationale,
			ObjScore: s.AIScore, SubjScore: s.TeacherScore, Comment: s.Rationale,
		})
	}
	if resp.Scores == nil {
		resp.Scores = []dto.DimensionScoreResp{}
	}
	return resp
}

// evalToFullDTO enriches with dimension name and weight from task dimensions.
func (h *EvaluationsHandler) evalToFullDTO(ctx context.Context, e *model.Evaluation) dto.EvaluationResponse {
	resp := evalToDTO(e)
	// Enrich with dimension metadata
	dims, err := h.taskSvc.GetDimensions(ctx, e.TaskID)
	if err == nil {
		dimMap := make(map[int64]struct {
			Name   string
			Weight int
		})
		for _, d := range dims {
			dimMap[d.ID] = struct {
				Name   string
				Weight int
			}{d.Name, d.Weight}
		}
		for i, s := range resp.Scores {
			if info, ok := dimMap[s.DimensionID]; ok {
				resp.Scores[i].DimensionName = info.Name
				resp.Scores[i].Weight = info.Weight
			}
		}
	}
	return resp
}
