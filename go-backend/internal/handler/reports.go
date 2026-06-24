package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/report"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
	"github.com/smartedu/training-eval-system/internal/store"
)

// errNoScoredData indicates no scored evaluations are available.
var errNoScoredData = errors.New("no scored evaluations available for export")

type ReportsHandler struct {
	evalSvc *service.EvaluationService
	taskSvc *service.TaskService
	userSvc *service.UserService
	db      *store.DB
}

func NewReportsHandler(evalSvc *service.EvaluationService, taskSvc *service.TaskService, userSvc *service.UserService, db *store.DB) *ReportsHandler {
	return &ReportsHandler{evalSvc: evalSvc, taskSvc: taskSvc, userSvc: userSvc, db: db}
}

func (h *ReportsHandler) GetPersonal(w http.ResponseWriter, r *http.Request) {
	evalID, err := PathInt64(r, "evalId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid evaluation ID")
		return
	}

	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	eval, err := h.evalSvc.GetByID(r.Context(), evalID)
	if err != nil || eval == nil {
		Error(w, http.StatusNotFound, "Evaluation not found")
		return
	}

	// Ownership check: students can only view their own evaluations
	if claims.Role == "student" && eval.StudentID != claims.Sub {
		Error(w, http.StatusNotFound, "Evaluation not found")
		return
	}
	// Teachers/admins can view evaluations for tasks they own/manage
	if claims.Role == "teacher" {
		task, err := h.taskSvc.GetByID(r.Context(), eval.TaskID)
		if err != nil || task.TeacherID != claims.Sub {
			Error(w, http.StatusNotFound, "Evaluation not found")
			return
		}
	}

	data, err := h.buildReportData(r, eval.TaskID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	exporter := &report.PDFExporter{}
	pdfBytes, err := exporter.ExportTaskReport(data)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=eval_%d_report.pdf", evalID))
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes)
}

func (h *ReportsHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "taskId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	data, err := h.buildReportData(r, taskID)
	if err != nil {
		if errors.Is(err, errNoScoredData) {
			Error(w, http.StatusNotFound, "No scored evaluations available for export")
			return
		}
		// Task not found or other error
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	// Generate Excel instead of CSV (Excel is more useful, endpoint name is legacy)
	exporter := &report.ExcelExporter{}
	xlsxBytes, err := exporter.ExportTaskReport(data)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=report.xlsx")
	w.WriteHeader(http.StatusOK)
	w.Write(xlsxBytes)
}

func (h *ReportsHandler) ExportStatisticsXLSX(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "taskId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	data, err := h.buildReportData(r, taskID)
	if err != nil {
		if errors.Is(err, errNoScoredData) {
			Error(w, http.StatusNotFound, "No scored evaluations available for export")
			return
		}
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	exporter := &report.ExcelExporter{}
	xlsxBytes, err := exporter.ExportTaskReport(data)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=statistics.xlsx")
	w.WriteHeader(http.StatusOK)
	w.Write(xlsxBytes)
}

func (h *ReportsHandler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "taskId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	data, err := h.buildReportData(r, taskID)
	if err != nil {
		if errors.Is(err, errNoScoredData) {
			Error(w, http.StatusNotFound, "No scored evaluations available for statistics")
			return
		}
		// Task not found or other error
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	// Generate PDF
	exporter := &report.PDFExporter{}
	pdfBytes, err := exporter.ExportTaskReport(data)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=statistics.pdf")
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes)
}

func (h *ReportsHandler) buildReportData(r *http.Request, taskID int64) (*report.TaskReportData, error) {
	ctx := r.Context()

	task, err := h.taskSvc.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	dims, err := h.taskSvc.GetDimensions(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Get all evaluations for task
	evalParams := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
	}
	evalParams.TaskID = &taskID
	evals, _, err := h.evalSvc.List(ctx, evalParams)
	if err != nil {
		return nil, err
	}

	// Filter to scored/confirmed only
	var scored []int64
	evalMap := make(map[int64]struct {
		studentID  int64
		totalScore float64
		comment    string
	})
	for _, e := range evals {
		if (e.Status == "scored" || e.Status == "confirmed") && e.TotalScore != nil {
			scored = append(scored, e.ID)
			evalMap[e.ID] = struct {
				studentID  int64
				totalScore float64
				comment    string
			}{
				studentID: e.StudentID, totalScore: *e.TotalScore, comment: e.TeacherComment,
			}
		}
	}

	if len(scored) == 0 {
		return nil, errNoScoredData
	}

	// Build dimension info
	dimInfos := make([]report.DimensionInfo, len(dims))
	for i, d := range dims {
		dimInfos[i] = report.DimensionInfo{ID: d.ID, Name: d.Name}
	}

	// Build student rows
	studentIDs := make(map[int64]struct{})
	for _, info := range evalMap {
		studentIDs[info.studentID] = struct{}{}
	}
	nameMap := h.userSvc.GetDisplayNames(ctx, studentIDs)

	var students []report.StudentReportRow
	for _, evalID := range scored {
		info := evalMap[evalID]
		full, err := h.evalSvc.GetByID(ctx, evalID)
		if err != nil {
			continue
		}

		dimScores := make(map[int64]float64)
		for _, s := range full.Scores {
			score := 0.0
			if s.TeacherScore != nil {
				score = *s.TeacherScore
			} else if s.AIScore != nil {
				score = *s.AIScore
			}
			dimScores[s.DimensionID] = score
		}

		students = append(students, report.StudentReportRow{
			StudentName:     nameMap[info.studentID],
			TotalScore:      info.totalScore,
			DimensionScores: dimScores,
			TeacherComment:  info.comment,
		})
	}

	return &report.TaskReportData{
		TaskName:   task.Name,
		CourseName: "", // Could resolve from course table if needed
		Dimensions: dimInfos,
		Students:   students,
	}, nil
}
