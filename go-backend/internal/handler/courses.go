package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
)

type CoursesHandler struct {
	svc      *service.CourseService
	classSvc *service.ClassService
}

func NewCoursesHandler(svc *service.CourseService, classSvc *service.ClassService) *CoursesHandler {
	return &CoursesHandler{svc: svc, classSvc: classSvc}
}

func (h *CoursesHandler) List(w http.ResponseWriter, r *http.Request) {
	params := repository.ListParams{Page: 1, PageSize: 100, Search: QueryStr(r, "search", "")}
	courses, _, err := h.svc.List(r.Context(), params)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, courses)
}

func (h *CoursesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCourseRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	c := &model.Course{Name: req.Name, Code: req.Code}
	if err := h.svc.Create(r.Context(), c); err != nil {
		Error(w, http.StatusConflict, err.Error())
		return
	}
	JSON(w, http.StatusCreated, c)
}

func (h *CoursesHandler) ToggleArchive(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid course ID")
		return
	}
	course, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "Course not found")
		return
	}
	course.IsArchived = !course.IsArchived
	_ = h.svc.Update(r.Context(), course)
	JSON(w, http.StatusOK, course)
}

func (h *CoursesHandler) GetClasses(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid course ID")
		return
	}
	classes, err := h.classSvc.List(r.Context(), &id, nil)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, classes)
}
