package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/service"
)

type ParseHandler struct{ svc *service.UploadService }

func NewParseHandler(svc *service.UploadService) *ParseHandler {
	return &ParseHandler{svc: svc}
}

func (h *ParseHandler) GetResult(w http.ResponseWriter, r *http.Request) {
	_, err := PathInt64(r, "uploadId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid upload ID")
		return
	}
	// Placeholder: return 404 until parse results are populated
	Error(w, http.StatusNotFound, "Parse result not found")
}
