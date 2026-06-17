package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/service"
)

type ImportsHandler struct {
	svc     *service.ImportService
	userSvc *service.UserService
}

func NewImportsHandler(svc *service.ImportService, userSvc *service.UserService) *ImportsHandler {
	return &ImportsHandler{svc: svc, userSvc: userSvc}
}

func (h *ImportsHandler) ImportUsers(w http.ResponseWriter, r *http.Request) {}
func (h *ImportsHandler) ImportStudents(w http.ResponseWriter, r *http.Request) {}
func (h *ImportsHandler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {}

const maxImportFileSize = 10 << 20
