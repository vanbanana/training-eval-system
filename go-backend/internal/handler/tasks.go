package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
)

type TasksHandler struct {
	svc *service.TaskService
}

func NewTasksHandler(svc *service.TaskService) *TasksHandler {
	return &TasksHandler{svc: svc}
}

func (h *TasksHandler) List(w http.ResponseWriter, r *http.Request) {
	params := repository.TaskListParams{
		ListParams: repository.ListParams{
			Page:     QueryInt(r, "page", 1),
			PageSize: QueryInt(r, "page_size", 20),
			Search:   QueryStr(r, "search", ""),
			SortBy:   QueryStr(r, "sort_by", ""),
			SortDir:  QueryStr(r, "sort_dir", "desc"),
		},
	}
	if s := QueryStr(r, "status", ""); s != "" {
		params.Status = &s
	}
	if c := QueryStr(r, "course_id", ""); c != "" {
		if id, err := strconv.ParseInt(c, 10, 64); err == nil {
			params.CourseID = &id
		}
	}
	// Role-based filtering: teachers see only their own tasks, students see published
	claims := middleware.GetClaims(r.Context())
	if claims != nil {
		switch claims.Role {
		case "teacher":
			params.TeacherID = &claims.Sub
		case "student":
			published := "published"
			params.Status = &published
		}
	}

	tasks, _, err := h.svc.List(r.Context(), params)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, taskToResponse(&t))
	}
	JSON(w, http.StatusOK, items)
}

func (h *TasksHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	task, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "Task not found")
		return
	}
	JSON(w, http.StatusOK, taskToResponse(task))
}

func (h *TasksHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTaskRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	claims := middleware.GetClaims(r.Context())
	task := &model.TrainingTask{
		Name:               req.Name,
		Description:        req.Description,
		Requirements:       req.Requirements,
		EvaluationCriteria: req.EvaluationCriteria,
		TeacherID:          claims.Sub,
		CourseID:           req.CourseID,
		Status:             "draft",
	}
	if req.Deadline != nil {
		if t, err := time.Parse(time.RFC3339, *req.Deadline); err == nil {
			task.Deadline = &t
		}
	}
	if err := h.svc.Create(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(req.ClassIDs) > 0 {
		_ = h.svc.SetClasses(r.Context(), task.ID, req.ClassIDs)
	}
	JSON(w, http.StatusCreated, taskToResponse(task))
}

func (h *TasksHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	task, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "Task not found")
		return
	}
	var req dto.UpdateTaskRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Requirements != nil {
		task.Requirements = *req.Requirements
	}
	if req.EvaluationCriteria != nil {
		task.EvaluationCriteria = *req.EvaluationCriteria
	}
	if req.CourseID != nil {
		task.CourseID = *req.CourseID
	}
	if req.Deadline != nil {
		if t, err := time.Parse(time.RFC3339, *req.Deadline); err == nil {
			task.Deadline = &t
		}
	}
	if err := h.svc.Update(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, taskToResponse(task))
}

func (h *TasksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Task deleted"})
}

func (h *TasksHandler) Publish(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	if err := h.svc.Publish(r.Context(), id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Task published"})
}

func (h *TasksHandler) Close(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	if err := h.svc.Close(r.Context(), id); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Task closed"})
}

func (h *TasksHandler) ReplaceDimensions(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	var req dto.ReplaceDimensionsRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	dims := make([]model.Dimension, len(req.Dimensions))
	for i, d := range req.Dimensions {
		dims[i] = model.Dimension{Name: d.Name, Description: d.Description, Weight: d.Weight, OrderIndex: d.OrderIndex}
	}
	if err := h.svc.SetDimensions(r.Context(), id, dims); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Dimensions updated"})
}

func taskToResponse(t *model.TrainingTask) dto.TaskResponse {
	resp := dto.TaskResponse{
		ID: t.ID, Name: t.Name, Description: t.Description,
		Requirements: t.Requirements, EvaluationCriteria: t.EvaluationCriteria,
		TeacherID: t.TeacherID, CourseID: t.CourseID, Status: t.Status,
		CreatedAt: t.CreatedAt.Format(time.RFC3339), UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
		ClassIDs: t.ClassIDs,
	}
	if t.Deadline != nil {
		s := t.Deadline.Format(time.RFC3339)
		resp.Deadline = &s
	}
	for _, d := range t.Dimensions {
		resp.Dimensions = append(resp.Dimensions, dto.DimensionResponse{
			ID: d.ID, TaskID: d.TaskID, Name: d.Name, Description: d.Description, Weight: d.Weight, OrderIndex: d.OrderIndex,
		})
	}
	if resp.Dimensions == nil {
		resp.Dimensions = []dto.DimensionResponse{}
	}
	if resp.ClassIDs == nil {
		resp.ClassIDs = []int64{}
	}
	return resp
}


