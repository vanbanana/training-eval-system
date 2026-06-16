package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
	"github.com/smartedu/training-eval-system/internal/store"
)

type GradingHandler struct {
	evalSvc   *service.EvaluationService
	uploadSvc *service.UploadService
	userSvc   *service.UserService
	db        *store.DB
}

func NewGradingHandler(evalSvc *service.EvaluationService, uploadSvc *service.UploadService, userSvc *service.UserService, db *store.DB) *GradingHandler {
	return &GradingHandler{evalSvc: evalSvc, uploadSvc: uploadSvc, userSvc: userSvc, db: db}
}

func (h *GradingHandler) GetSubmissions(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	ctx := r.Context()

	// Get uploads for this task
	params := repository.UploadListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 200},
	}
	params.TaskID = &taskID
	uploads, _, err := h.uploadSvc.List(ctx, params)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get evaluations for this task
	evalParams := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 200},
	}
	evalParams.TaskID = &taskID
	evals, _, _ := h.evalSvc.List(ctx, evalParams)

	// Build evaluation map by upload_id
	evalMap := make(map[int64]*struct {
		id     int64
		status string
		score  *float64
	})
	for _, e := range evals {
		evalMap[e.UploadID] = &struct {
			id     int64
			status string
			score  *float64
		}{e.ID, e.Status, e.TotalScore}
	}

	// Collect unique student IDs and resolve names
	studentIDs := make(map[int64]struct{})
	for _, u := range uploads {
		studentIDs[u.StudentID] = struct{}{}
	}
	nameMap := h.userSvc.GetDisplayNames(ctx, studentIDs)

	items := make([]dto.SubmissionResponse, 0, len(uploads))
	for _, u := range uploads {
		sub := dto.SubmissionResponse{
			UploadID:    u.ID,
			StudentID:   u.StudentID,
			StudentName: nameMap[u.StudentID],
			Filename:    u.Filename,
			ParseStatus: u.ParseStatus,
			SubmittedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if ev, ok := evalMap[u.ID]; ok {
			sub.EvaluationID = &ev.id
			sub.EvalStatus = &ev.status
			sub.TotalScore = ev.score
			sub.ScoringInProgress = ev.status == "pending"
		}
		items = append(items, sub)
	}
	JSON(w, http.StatusOK, items)
}

func (h *GradingHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	ctx := r.Context()

	// Get uploads for count info
	uploadParams := repository.UploadListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
	}
	uploadParams.TaskID = &taskID
	uploads, _, _ := h.uploadSvc.List(ctx, uploadParams)

	totalUploads := len(uploads)
	parsedCount := 0
	for _, u := range uploads {
		if u.ParseStatus == "parsed" {
			parsedCount++
		}
	}

	// Get evaluations for summary stats
	evalParams := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
	}
	evalParams.TaskID = &taskID
	evals, _, _ := h.evalSvc.List(ctx, evalParams)

	summary := dto.TaskSummaryResponse{TaskID: taskID}
	summary.TotalUploads = totalUploads
	summary.ParsedCount = parsedCount
	summary.SubmittedCount = len(evals)

	var totalScore, highest float64
	lowest := -1.0
	scored := 0
	for _, e := range evals {
		if e.TotalScore != nil {
			scored++
			s := *e.TotalScore
			totalScore += s
			if s > highest {
				highest = s
			}
			if lowest < 0 || s < lowest {
				lowest = s
			}
		}
		if e.Status == "confirmed" {
			summary.ConfirmedCount++
		}
		if e.Status == "rejected" {
			summary.RejectedCount++
		}
	}
	summary.ScoredCount = scored
	if scored > 0 {
		summary.AverageScore = totalScore / float64(scored)
	}
	summary.HighestScore = highest
	if lowest < 0 {
		lowest = 0
	}
	summary.LowestScore = lowest

	// Progress percent
	if totalUploads > 0 {
		summary.ProgressPercent = float64(summary.ConfirmedCount) / float64(totalUploads) * 100
	}

	// Total students from task_classes
	var totalStudents int64
	h.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM class_memberships cm
		 JOIN task_classes tc ON tc.class_id = cm.class_id
		 WHERE tc.task_id = ?`, taskID).Scan(&totalStudents)
	summary.TotalStudents = int(totalStudents)

	JSON(w, http.StatusOK, summary)
}

func (h *GradingHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid evaluation ID")
		return
	}

	// Read optional teacher comment and score overrides from the request body.
	var req dto.ConfirmRequest
	_ = Decode(r, &req) // body is optional; ignore decode errors for backward compat

	ctx := r.Context()
	if req.TeacherComment != "" || len(req.ScoreOverrides) > 0 {
		eval, err := h.evalSvc.GetByID(ctx, id)
		if err != nil {
			Error(w, http.StatusNotFound, "Evaluation not found")
			return
		}
		if req.TeacherComment != "" {
			eval.TeacherComment = req.TeacherComment
		}
		// Apply score overrides (dimension_id -> teacher_score)
		for dimID, score := range req.ScoreOverrides {
			for i, s := range eval.Scores {
				if s.DimensionID == dimID {
					val := score
					eval.Scores[i].TeacherScore = &val
					break
				}
			}
		}
		if len(req.ScoreOverrides) > 0 {
			_ = h.evalSvc.SaveScores(ctx, id, eval.Scores)
		}
		if req.TeacherComment != "" {
			// Persist comment via update (status stays scored until BatchConfirm below)
			eval.TeacherComment = req.TeacherComment
			_ = h.evalSvc.Update(ctx, eval)
		}
	}

	if err := h.evalSvc.BatchConfirm(ctx, []int64{id}); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Evaluation confirmed"})
}

func (h *GradingHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid evaluation ID")
		return
	}

	// Read rejection reason from body.
	var req dto.RejectRequest
	_ = Decode(r, &req)

	eval, err := h.evalSvc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "Evaluation not found")
		return
	}
	eval.Status = "rejected"
	if req.Reason != "" {
		eval.TeacherComment = req.Reason
	}
	if err := h.evalSvc.Update(r.Context(), eval); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Evaluation rejected"})
}
