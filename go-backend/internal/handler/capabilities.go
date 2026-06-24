package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/middleware"
)

// CapabilitiesHandler serves the public capabilities endpoint,
// allowing the frontend to discover which agent features are enabled
// without requiring authentication (T9.2).
type CapabilitiesHandler struct {
	Flags middleware.FeatureFlags
}

// NewCapabilitiesHandler creates a CapabilitiesHandler with the given feature flags.
func NewCapabilitiesHandler(flags middleware.FeatureFlags) *CapabilitiesHandler {
	return &CapabilitiesHandler{Flags: flags}
}

// GetCapabilities returns the current feature flag state as JSON.
// This endpoint is public — the frontend uses it to show/hide UI entries.
func (h *CapabilitiesHandler) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]any{
		"agent_v2": map[string]any{
			"enabled": h.Flags.AgentV2Enabled,
			"student": h.Flags.StudentAgentV2Enabled && h.Flags.AgentV2Enabled,
			"teacher": h.Flags.TeacherAgentEnabled && h.Flags.AgentV2Enabled,
			"admin":   h.Flags.AdminAgentEnabled && h.Flags.AgentV2Enabled,
		},
		"agent_tool_events": h.Flags.AgentToolEventsEnabled,
	})
}
