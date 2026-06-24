// Package handler provides UsageHandler for admin token usage reporting endpoints.
package handler

import (
	"net/http"
	"time"

	"github.com/smartedu/training-eval-system/internal/service"
)

// UsageHandler handles /api/usage/* endpoints for admin usage reporting.
type UsageHandler struct {
	usageSvc *service.UsageService
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(svc *service.UsageService) *UsageHandler {
	return &UsageHandler{usageSvc: svc}
}

// GetSummary returns aggregated usage statistics.
// GET /api/usage/summary?from=2006-01-02T15:04:05Z&to=2006-01-02T15:04:05Z
func (h *UsageHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	from, to := parseTimeRange(r)

	report, err := h.usageSvc.GetFullReport(r.Context(), from, to)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to retrieve usage data")
		return
	}

	JSON(w, http.StatusOK, report)
}

// parseTimeRange extracts from/to query parameters with sensible defaults.
func parseTimeRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now()
	from := now.AddDate(0, 0, -1) // default: last 24 hours
	to := now.Add(1 * time.Second)

	if f := r.URL.Query().Get("from"); f != "" {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			from = t
		} else if t, err := time.Parse("2006-01-02", f); err == nil {
			from = t
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = parsed
		} else if parsed, err := time.Parse("2006-01-02", t); err == nil {
			to = parsed.AddDate(0, 0, 1)
		}
	}

	// Support period shortcuts
	switch r.URL.Query().Get("period") {
	case "today":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to = now.Add(1 * time.Second)
	case "week":
		from = now.AddDate(0, 0, -7)
		to = now.Add(1 * time.Second)
	case "month":
		from = now.AddDate(0, -1, 0)
		to = now.Add(1 * time.Second)
	}

	return from, to
}
