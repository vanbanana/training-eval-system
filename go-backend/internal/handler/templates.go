package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/service"
)

type TemplatesHandler struct {
	svc     *service.TemplateService
	taskSvc *service.TaskService
}

func NewTemplatesHandler(svc *service.TemplateService, taskSvc *service.TaskService) *TemplatesHandler {
	return &TemplatesHandler{svc: svc, taskSvc: taskSvc}
}

func (h *TemplatesHandler) List(w http.ResponseWriter, r *http.Request) {
	templates, err := h.svc.List(r.Context(), nil, nil, nil)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if templates == nil {
		templates = []model.EvalTemplate{}
	}
	JSON(w, http.StatusOK, templates)
}

func (h *TemplatesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTemplateRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	t := &model.EvalTemplate{Name: req.Name, Description: req.Description, Visibility: "private"}
	if req.Visibility != "" {
		t.Visibility = req.Visibility
	}
	if err := h.svc.Create(r.Context(), t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusCreated, t)
}

func (h *TemplatesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid template ID")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusNotFound, "Template not found")
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Template deleted"})
}

func (h *TemplatesHandler) CreateFromTask(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateFromTaskRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	JSON(w, http.StatusCreated, dto.SuccessResponse{Message: "Template created from task"})
}
