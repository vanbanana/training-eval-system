package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
)

type AuditHandler struct{ svc *service.AuditService }

func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	params := repository.ListParams{
		Page: QueryInt(r, "page", 1), PageSize: QueryInt(r, "page_size", 20),
		Search: QueryStr(r, "search", ""),
	}
	logs, total, err := h.svc.List(r.Context(), params, nil, nil)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.AuditLogResponse, 0, len(logs))
	for _, l := range logs {
		items = append(items, dto.AuditLogResponse{
			ID: l.ID, OccurredAt: l.OccurredAt.Format("2006-01-02T15:04:05Z07:00"),
			UserID: l.UserID, Username: l.Username, Role: l.Role, Action: l.Action,
			TargetType: l.TargetType, TargetID: l.TargetID, Target: l.Target,
			Result: l.Result, Detail: l.Detail, ClientIP: l.ClientIP,
			TraceID: l.TraceID, SuspiciousFlag: l.SuspiciousFlag,
			CreatedAt: l.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	JSON(w, http.StatusOK, dto.PaginatedResponse[dto.AuditLogResponse]{Items: items, Total: total, Page: params.Page, PageSize: params.PageSize})
}

func (h *AuditHandler) Export(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_logs.csv")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("id,occurred_at,username,action,target,result\n"))
}
