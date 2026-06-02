package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
)

type ImportsHandler struct{}

func NewImportsHandler() *ImportsHandler { return &ImportsHandler{} }

func (h *ImportsHandler) ImportUsers(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, dto.ImportResultResponse{Status: "done"})
}

func (h *ImportsHandler) ImportStudents(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, dto.ImportResultResponse{Status: "done"})
}

func (h *ImportsHandler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=user_template.xlsx")
	w.WriteHeader(http.StatusOK)
	// Placeholder: return empty xlsx-like content
	w.Write([]byte{})
}
