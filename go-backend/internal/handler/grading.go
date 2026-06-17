package handler

import (
	"context"
	"fmt"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/pipeline"
	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
	"github.com/smartedu/training-eval-system/internal/store"
)

type GradingHandler struct {
	evalSvc   *service.EvaluationService
	uploadSvc *service.UploadService
	userSvc   *service.UserService
	db        *store.DB
	orch      *pipeline.Orchestrator
	llmClient *llm.Client
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
	// Return updated evaluation state so frontend can display the confirmed score.
	updated, err := h.evalSvc.GetByID(ctx, id)
	if err != nil {
		// BatchConfirm succeeded but re-fetch failed; return a minimal success response.
		JSON(w, http.StatusOK, map[string]any{"message": "Evaluation confirmed"})
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"message":     "Evaluation confirmed",
		"total_score": updated.TotalScore,
		"status":      updated.Status,
	})
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
	JSON(w, http.StatusOK, map[string]any{
		"message":     "Evaluation rejected",
		"total_score": eval.TotalScore,
		"status":      eval.Status,
	})
}

// AutoScore triggers AI scoring for unscored submissions (T3.1).
func (h *GradingHandler) AutoScore(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	var req struct {
		Mode      string    `json:"mode"`
		UploadIDs []int64   `json:"upload_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Mode == "" {
		req.Mode = "unscored"
	}
	ctx := r.Context()

	// Find all uploads for this task
	params := repository.UploadListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
	}
	params.TaskID = &taskID
	uploads, _, err := h.uploadSvc.List(ctx, params)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get existing evaluations for this task
	evalParams := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
	}
	evalParams.TaskID = &taskID
	evals, _, _ := h.evalSvc.List(ctx, evalParams)

	// Map existing evaluations by upload_id
	existingEval := make(map[int64]string)
	for _, e := range evals {
		existingEval[e.UploadID] = e.Status
	}

	// Filter uploads to score
	result := dto.AutoScoreResponse{
		TaskID: taskID,
		Items:  make([]dto.AutoScoreItem, 0),
	}
	for _, u := range uploads {
		if req.Mode == "selected" {
			found := false
			for _, id := range req.UploadIDs {
				if id == u.ID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if u.ParseStatus != "parsed" {
			result.Skipped++
			result.Items = append(result.Items, dto.AutoScoreItem{UploadID: u.ID, Status: "skipped", Reason: "not_parsed"})
			continue
		}
		status, exists := existingEval[u.ID]
		if exists && (status == "scored" || status == "confirmed") {
			result.Skipped++
			result.Items = append(result.Items, dto.AutoScoreItem{UploadID: u.ID, Status: "skipped", Reason: "already_scored"})
			continue
		}
		if exists && status == "pending" {
			result.Skipped++
			result.Items = append(result.Items, dto.AutoScoreItem{UploadID: u.ID, Status: "skipped", Reason: "already_queued"})
			continue
		}

		// Create pending evaluation
		_ = middleware.GetClaims(ctx)
		eval := &model.Evaluation{
			TaskID:    taskID,
			UploadID:  u.ID,
			StudentID: u.StudentID,
			Status:    "pending",
		}
		if err := h.evalSvc.Create(ctx, eval); err != nil {
			result.Failed++
			result.Items = append(result.Items, dto.AutoScoreItem{UploadID: u.ID, Status: "failed", Reason: err.Error()})
			continue
		}
		result.Queued++
		result.Requested++
		result.Items = append(result.Items, dto.AutoScoreItem{UploadID: u.ID, Status: "queued"})
	}
	JSON(w, http.StatusOK, result)
}

// Workbench returns the teacher's grading workbench data (T4.2).
func (h *GradingHandler) Workbench(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims.Role != "teacher" && claims.Role != "admin" {
		Error(w, http.StatusForbidden, "Only teachers and admins can access workbench")
		return
	}
	ctx := r.Context()

	// Get courses for this teacher (admin sees all)
	var courses []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	}
	var query string
	if claims.Role == "admin" {
		query = "SELECT id, name, code FROM courses WHERE is_archived=0 ORDER BY name"
	} else {
		query = `SELECT DISTINCT c.id, c.name, c.code FROM courses c
			JOIN classes cl ON cl.course_id = c.id
			WHERE cl.teacher_id = ? AND c.is_archived=0 ORDER BY c.name`
	}
	var rows *sql.Rows
	var err error
	if claims.Role == "admin" {
		rows, err = h.db.Reader.QueryContext(ctx, query)
	} else {
		rows, err = h.db.Reader.QueryContext(ctx, query, claims.Sub)
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var c struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Code string `json:"code"`
		}
		if err := rows.Scan(&c.ID, &c.Name, &c.Code); err != nil {
			continue
		}
		courses = append(courses, c)
	}

	type classInfo struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		StudentCount int    `json:"student_count"`
	}
	type taskInfo struct {
		ID               int64   `json:"id"`
		Name             string  `json:"name"`
		Status           string  `json:"status"`
		PendingAICount   int     `json:"pending_ai_count"`
		ScoredCount      int     `json:"scored_count"`
		ConfirmedCount   int     `json:"confirmed_count"`
		RejectedCount    int     `json:"rejected_count"`
	}

	workbench := struct {
		Courses []struct {
			ID      int64       `json:"id"`
			Name    string      `json:"name"`
			Code    string      `json:"code"`
			Classes []struct {
				classInfo
				Tasks []taskInfo `json:"tasks"`
			} `json:"classes"`
		} `json:"courses"`
		Summary struct {
			PendingAICount      int `json:"pending_ai_count"`
			ScoredUnconfirmed   int `json:"scored_unconfirmed_count"`
			SuspiciousCount     int `json:"suspicious_count"`
			ConfirmedTodayCount int `json:"confirmed_today_count"`
		} `json:"summary"`
	}{}

	for _, c := range courses {
		courseEntry := struct {
			ID      int64       `json:"id"`
			Name    string      `json:"name"`
			Code    string      `json:"code"`
			Classes []struct {
				classInfo
				Tasks []taskInfo `json:"tasks"`
			} `json:"classes"`
		}{ID: c.ID, Name: c.Name, Code: c.Code}

		// Get classes for this course
		var teacherID *int64
		if claims.Role == "teacher" {
			teacherID = &claims.Sub
		}
		classes, _ := h.db.Reader.QueryContext(ctx,
			`SELECT id, name, student_count FROM classes WHERE course_id=? AND is_archived=0
			 AND (? IS NULL OR teacher_id=?) ORDER BY name`,
			c.ID, teacherID, claims.Sub)
		for classes.Next() {
			var cl struct {
				classInfo
				Tasks []taskInfo `json:"tasks"`
			}
			classes.Scan(&cl.ID, &cl.Name, &cl.StudentCount)

			// Get tasks for this class
			tasks, _ := h.db.Reader.QueryContext(ctx,
				`SELECT t.id, t.name, t.status FROM training_tasks t
				 JOIN task_classes tc ON tc.task_id = t.id
				 WHERE tc.class_id = ? AND t.course_id = ? AND t.status != 'draft'
				 ORDER BY t.created_at DESC`, cl.ID, c.ID)
			for tasks.Next() {
				var tk taskInfo
				tasks.Scan(&tk.ID, &tk.Name, &tk.Status)

				// Count eval statuses for this task
				h.db.Reader.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM evaluations e
					 JOIN uploads u ON u.id = e.upload_id
					 WHERE e.task_id = ? AND e.status = 'pending'`, tk.ID).Scan(&tk.PendingAICount)
				h.db.Reader.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM evaluations WHERE task_id = ? AND status = 'scored'`, tk.ID).Scan(&tk.ScoredCount)
				h.db.Reader.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM evaluations WHERE task_id = ? AND status = 'confirmed'`, tk.ID).Scan(&tk.ConfirmedCount)
				h.db.Reader.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM evaluations WHERE task_id = ? AND status = 'rejected'`, tk.ID).Scan(&tk.RejectedCount)

				cl.Tasks = append(cl.Tasks, tk)
				workbench.Summary.PendingAICount += tk.PendingAICount
				workbench.Summary.ScoredUnconfirmed += tk.ScoredCount
				workbench.Summary.ConfirmedTodayCount += tk.ConfirmedCount
			}
			tasks.Close()
			if cl.Tasks == nil {
				cl.Tasks = []taskInfo{}
			}
			courseEntry.Classes = append(courseEntry.Classes, cl)
		}
		classes.Close()
		if courseEntry.Classes == nil {
			courseEntry.Classes = []struct {
				classInfo
				Tasks []taskInfo `json:"tasks"`
			}{}
		}
		workbench.Courses = append(workbench.Courses, courseEntry)
	}

	JSON(w, http.StatusOK, workbench)
}

// ReportView returns the report view for an upload (T5.1).
func (h *GradingHandler) ReportView(w http.ResponseWriter, r *http.Request) {
	uploadID, err := PathInt64(r, "uploadId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid upload ID")
		return
	}
	ctx := r.Context()

	upload, err := h.uploadSvc.GetByID(ctx, uploadID)
	if err != nil {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}

	pr, err := h.uploadSvc.GetParseResult(ctx, uploadID)
	if err != nil || pr == nil {
		JSON(w, http.StatusOK, map[string]any{
			"upload_id":   uploadID,
			"filename":    upload.Filename,
			"file_type":   upload.FileType,
			"render_mode": "unavailable",
			"is_readable": false,
			"warnings":    []string{"not_parsed"},
		})
		return
	}

	analysis := service.AnalyzeReadability(pr.RawText)
	renderMode := "plain_text"
	if !analysis.IsReadable {
		renderMode = "unavailable"
	} else if len(analysis.Sections) > 1 {
		renderMode = "structured_text"
	}

	resp := map[string]any{
		"upload_id":   uploadID,
		"filename":    upload.Filename,
		"file_type":   upload.FileType,
		"render_mode": renderMode,
		"content":     analysis.CleanText,
		"is_readable": analysis.IsReadable,
		"warnings":    analysis.Warnings,
	}
	if len(analysis.Sections) > 0 {
		resp["sections"] = analysis.Sections
	}

	JSON(w, http.StatusOK, resp)
}

// canAccessTask returns error if the current user cannot access the given task.
func (h *GradingHandler) canAccessTask(ctx context.Context, r *http.Request, taskID int64) error {
	claims := middleware.GetClaims(ctx)
	if claims.Role == "admin" {
		return nil
	}
	if claims.Role != "teacher" {
		return fmt.Errorf("access denied")
	}
	var teacherID int64
	err := h.db.Reader.QueryRowContext(ctx,
		"SELECT teacher_id FROM training_tasks WHERE id=?", taskID).Scan(&teacherID)
	if err != nil {
		return fmt.Errorf("task not found")
	}
	if teacherID != claims.Sub {
		return fmt.Errorf("access denied")
	}
	return nil
}

// TriggerScoreForUpload creates a pending evaluation and queues scoring (T3.2).
func (h *GradingHandler) TriggerScoreForUpload(w http.ResponseWriter, r *http.Request) {
	uploadID, err := PathInt64(r, "uploadId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid upload ID")
		return
	}
	ctx := r.Context()

	upload, err := h.uploadSvc.GetByID(ctx, uploadID)
	if err != nil {
		Error(w, http.StatusNotFound, "Upload not found")
		return
	}

	// Check existing evaluations
	params := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 5},
	}
	params.UploadID = &uploadID
	evals, _, _ := h.evalSvc.List(ctx, params)
	for _, e := range evals {
		if e.Status == "scored" || e.Status == "confirmed" {
			JSON(w, http.StatusOK, map[string]any{
				"status": "skipped", "reason": "already_scored",
			})
			return
		}
		if e.Status == "pending" {
			JSON(w, http.StatusOK, map[string]any{
				"status": "skipped", "reason": "already_queued",
			})
			return
		}
	}

	// Create pending evaluation
	eval := &model.Evaluation{
		TaskID:    upload.TaskID,
		UploadID:  uploadID,
		StudentID: upload.StudentID,
		Status:    "pending",
	}
	if err := h.evalSvc.Create(ctx, eval); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"status": "queued", "evaluation_id": eval.ID,
	})
}
