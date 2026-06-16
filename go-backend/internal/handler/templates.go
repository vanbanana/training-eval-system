package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
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

// toTemplateResponse maps a model template (with items) to the response DTO,
// exposing dimensions under the "dimensions" key the frontend expects.
func toTemplateResponse(t *model.EvalTemplate) dto.TemplateResponse {
	dims := make([]dto.TemplateDimensionResp, 0, len(t.Items))
	for _, it := range t.Items {
		dims = append(dims, dto.TemplateDimensionResp{
			ID: it.ID, TemplateID: it.TemplateID, Name: it.Name,
			Description: it.Description, Weight: it.Weight, OrderIndex: it.OrderIndex,
		})
	}
	return dto.TemplateResponse{
		ID: t.ID, Name: t.Name, Description: t.Description, Visibility: t.Visibility,
		OwnerID: t.OwnerID, CourseID: t.CourseID,
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Dimensions: dims,
	}
}

func (h *TemplatesHandler) List(w http.ResponseWriter, r *http.Request) {
	templates, err := h.svc.List(r.Context(), nil, nil, nil)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.TemplateResponse, 0, len(templates))
	for i := range templates {
		items = append(items, toTemplateResponse(&templates[i]))
	}
	JSON(w, http.StatusOK, items)
}

func (h *TemplatesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTemplateRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	t := &model.EvalTemplate{Name: req.Name, Description: req.Description, Visibility: "private", CourseID: req.CourseID}
	if req.Visibility != "" {
		t.Visibility = req.Visibility
	}
	if claims := middleware.GetClaims(r.Context()); claims != nil {
		t.OwnerID = &claims.Sub
	}
	if err := h.svc.Create(r.Context(), t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(req.Dimensions) > 0 {
		items := make([]model.TemplateDimension, len(req.Dimensions))
		for i, it := range req.Dimensions {
			items[i] = model.TemplateDimension{
				TemplateID: t.ID, Name: it.Name, Description: it.Description,
				Weight: it.Weight, OrderIndex: it.OrderIndex,
			}
		}
		if err := h.svc.SetItems(r.Context(), t.ID, items); err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		t.Items = items
	}
	JSON(w, http.StatusCreated, toTemplateResponse(t))
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
	ctx := r.Context()

	task, err := h.taskSvc.GetByID(ctx, req.TaskID)
	if err != nil || task == nil {
		Error(w, http.StatusNotFound, "Task not found")
		return
	}
	dims, err := h.taskSvc.GetDimensions(ctx, req.TaskID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	name := req.Name
	if name == "" {
		name = task.Name + " 模板"
	}
	t := &model.EvalTemplate{Name: name, Description: task.Description, Visibility: "private"}
	if claims := middleware.GetClaims(ctx); claims != nil {
		t.OwnerID = &claims.Sub
	}
	if err := h.svc.Create(ctx, t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]model.TemplateDimension, len(dims))
	for i, d := range dims {
		items[i] = model.TemplateDimension{
			TemplateID: t.ID, Name: d.Name, Description: d.Description,
			Weight: d.Weight, OrderIndex: d.OrderIndex,
		}
	}
	if len(items) > 0 {
		if err := h.svc.SetItems(ctx, t.ID, items); err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		t.Items = items
	}
	JSON(w, http.StatusCreated, toTemplateResponse(t))
}
