package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/store"
)

// HealthHandler handles the health check endpoint.
type HealthHandler struct {
	db *store.DB
}

func NewHealthHandler(db *store.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Health handles GET /healthz with database connectivity check.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	// Check database connectivity
	if err := h.db.Reader.PingContext(r.Context()); err != nil {
		JSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  "database unreachable",
		})
		return
	}
	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
