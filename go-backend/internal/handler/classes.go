package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/service"
)

type ClassesHandler struct {
	svc     *service.ClassService
	userSvc *service.UserService
}

func NewClassesHandler(svc *service.ClassService, userSvc *service.UserService) *ClassesHandler {
	return &ClassesHandler{svc: svc, userSvc: userSvc}
}

func (h *ClassesHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var teacherID *int64
	if claims != nil && claims.Role == "teacher" {
		teacherID = &claims.Sub
	}
	classes, err := h.svc.List(r.Context(), nil, teacherID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, classes)
}

func (h *ClassesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateClassRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	claims := middleware.GetClaims(r.Context())
	c := &model.Class{Name: req.Name, CourseID: req.CourseID, TeacherID: claims.Sub}
	if err := h.svc.Create(r.Context(), c); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusCreated, c)
}

func (h *ClassesHandler) GetStudents(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid class ID")
		return
	}
	members, err := h.svc.GetMembers(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Resolve student info
	studentIDs := make(map[int64]struct{}, len(members))
	for _, m := range members {
		studentIDs[m.StudentID] = struct{}{}
	}
	usersMap := h.userSvc.GetUsersMap(r.Context(), studentIDs)

	items := make([]dto.StudentInClassResponse, 0, len(members))
	for _, m := range members {
		item := dto.StudentInClassResponse{
			ID:       m.StudentID,
			JoinedAt: m.JoinedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if u, ok := usersMap[m.StudentID]; ok {
			item.Username = u.Username
			item.DisplayName = u.DisplayName
		}
		items = append(items, item)
	}
	JSON(w, http.StatusOK, items)
}

func (h *ClassesHandler) ToggleArchive(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid class ID")
		return
	}
	cls, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "Class not found")
		return
	}
	cls.IsArchived = !cls.IsArchived
	JSON(w, http.StatusOK, cls)
}

func (h *ClassesHandler) BulkAddStudents(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid class ID")
		return
	}
	var req dto.BulkAddStudentsRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	for _, sid := range req.StudentIDs {
		_ = h.svc.AddMember(r.Context(), id, sid)
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Students added"})
}
