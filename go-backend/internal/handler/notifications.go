package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
)

type NotificationsHandler struct{ svc *service.NotificationService }

func NewNotificationsHandler(svc *service.NotificationService) *NotificationsHandler {
	return &NotificationsHandler{svc: svc}
}

func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	params := repository.ListParams{Page: 1, PageSize: QueryInt(r, "limit", 50)}
	notifs, _, err := h.svc.List(r.Context(), claims.Sub, false, params)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	unread, _ := h.svc.UnreadCount(r.Context(), claims.Sub)
	items := make([]dto.NotificationResponse, 0, len(notifs))
	for _, n := range notifs {
		items = append(items, dto.NotificationResponse{
			ID: n.ID, UserID: n.UserID, Type: n.Type, Title: n.Title,
			Content: n.Content, IsRead: n.IsRead, Link: n.Link,
			CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	JSON(w, http.StatusOK, dto.NotificationListResponse{Items: items, UnreadCount: unread})
}

func (h *NotificationsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}
	_ = h.svc.MarkRead(r.Context(), id)
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Marked as read"})
}

func (h *NotificationsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	_ = h.svc.MarkAllRead(r.Context(), claims.Sub)
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "All marked as read"})
}

func (h *NotificationsHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	prefs, err := h.svc.GetPreferences(r.Context(), claims.Sub)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.PreferencesResponse(prefs))
}

func (h *NotificationsHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdatePreferencesRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	claims := middleware.GetClaims(r.Context())
	if err := h.svc.UpdatePreference(r.Context(), claims.Sub, req.EventType, req.Enabled); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Return the full updated preferences map for convenience.
	prefs, err := h.svc.GetPreferences(r.Context(), claims.Sub)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.PreferencesResponse(prefs))
}
