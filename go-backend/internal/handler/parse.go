package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/service"
)

type ParseHandler struct{ svc *service.UploadService }

func NewParseHandler(svc *service.UploadService) *ParseHandler {
	return &ParseHandler{svc: svc}
}

func (h *ParseHandler) GetResult(w http.ResponseWriter, r *http.Request) {
	uploadID, err := PathInt64(r, "uploadId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid upload ID")
		return
	}

	pr, err := h.svc.GetParseResult(r.Context(), uploadID)
	if err != nil || pr == nil {
		Error(w, http.StatusNotFound, "Parse result not found")
		return
	}

	JSON(w, http.StatusOK, dto.ParseResultResponse{
		ID:                pr.ID,
		UploadID:          pr.UploadID,
		StructuredContent: pr.StructuredContent,
		RawText:           pr.RawText,
		SimHash:           pr.SimHash,
		ErrorMessage:      pr.ErrorMessage,
		ParsedAt:          pr.ParsedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}
