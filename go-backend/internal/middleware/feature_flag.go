package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
)

// FeatureFlags holds the runtime state of agent feature gates (T9.2).
type FeatureFlags struct {
	AgentV2Enabled         bool
	StudentAgentV2Enabled  bool
	TeacherAgentEnabled    bool
	AdminAgentEnabled      bool
	AgentToolEventsEnabled bool
}

// AgentFeatureGate is middleware that checks feature flags for agent routes.
// It returns 503 "service_disabled" when the agent system or the caller's
// role-specific flag is disabled. Returns 401 if authentication is missing.
func AgentFeatureGate(flags FeatureFlags) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Master switch.
			if !flags.AgentV2Enabled {
				disabledJSON(w, "Agent system is currently disabled")
				return
			}

			// Fail closed: require authentication for role-specific checks
			claims := GetClaims(r.Context())
			if claims == nil {
				disabledJSON(w, "Authentication required")
				return
			}

			// Role-specific check (fail closed on unknown roles)
			switch claims.Role {
			case "student":
				if !flags.StudentAgentV2Enabled {
					disabledJSON(w, "Student agent is currently disabled")
					return
				}
			case "teacher":
				if !flags.TeacherAgentEnabled {
					disabledJSON(w, "Teacher agent is currently disabled")
					return
				}
			case "admin":
				if !flags.AdminAgentEnabled {
					disabledJSON(w, "Admin agent is currently disabled")
					return
				}
			default:
				disabledJSON(w, "Unknown role: access denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func disabledJSON(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(dto.AgentErrorResponse{
		Code:    "SERVICE_DISABLED",
		Message: message,
	})
}
