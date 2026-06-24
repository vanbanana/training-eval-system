package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/handler"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/testutil"
)

// buildFlagRouter creates a minimal router with the given feature flags and an agent handler
// that returns 200 OK for all routes. Used to test feature flag gating in isolation.
func buildFlagRouter(t *testing.T, flags middleware.FeatureFlags) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	r.Use(middleware.TraceMiddleware)
	r.Use(middleware.Recoverer)

	// Public capabilities endpoint.
	capHandler := handler.NewCapabilitiesHandler(flags)
	r.Get("/api/capabilities", capHandler.GetCapabilities)

	// Protected agent routes with feature gate.
	r.Route("/api", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(testutil.TestJWTSecret))
			r.Route("/agent", func(r chi.Router) {
				r.Use(middleware.AgentFeatureGate(flags))
				r.Get("/sessions", func(w http.ResponseWriter, r *http.Request) {
					handler.JSON(w, http.StatusOK, []any{})
				})
				r.Post("/sessions", func(w http.ResponseWriter, r *http.Request) {
					handler.JSON(w, http.StatusCreated, map[string]any{"id": 1})
				})
				r.Post("/stream", func(w http.ResponseWriter, r *http.Request) {
					handler.JSON(w, http.StatusOK, map[string]any{"ok": true})
				})
			})
		})
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// TEST-T9.2-01: Teacher flag off — teacher agent returns 503.
func TestT92_01_TeacherFlagOff(t *testing.T) {
	flags := middleware.FeatureFlags{
		AgentV2Enabled:        true,
		StudentAgentV2Enabled: true,
		TeacherAgentEnabled:   false, // disabled
		AdminAgentEnabled:     true,
	}
	srv := buildFlagRouter(t, flags)

	resp := doRequest(t, srv, "GET", "/api/agent/sessions", testutil.TeacherAToken(), nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
	var errResp dto.AgentErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	resp.Body.Close()
	if errResp.Code != "SERVICE_DISABLED" {
		t.Errorf("expected code SERVICE_DISABLED, got %q", errResp.Code)
	}
}

// TEST-T9.2-02: Teacher flag on — teacher agent is available.
func TestT92_02_TeacherFlagOn(t *testing.T) {
	flags := middleware.FeatureFlags{
		AgentV2Enabled:        true,
		StudentAgentV2Enabled: true,
		TeacherAgentEnabled:   true,
		AdminAgentEnabled:     true,
	}
	srv := buildFlagRouter(t, flags)

	resp := doRequest(t, srv, "GET", "/api/agent/sessions", testutil.TeacherAToken(), nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TEST-T9.2-03: Admin flag off — admin agent returns 503.
func TestT92_03_AdminFlagOff(t *testing.T) {
	flags := middleware.FeatureFlags{
		AgentV2Enabled:        true,
		StudentAgentV2Enabled: true,
		TeacherAgentEnabled:   true,
		AdminAgentEnabled:     false, // disabled
	}
	srv := buildFlagRouter(t, flags)

	resp := doRequest(t, srv, "GET", "/api/agent/sessions", testutil.AdminAToken(), nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TEST-T9.2-04: Student v2 off — student agent returns 503.
func TestT92_04_StudentV2Off(t *testing.T) {
	flags := middleware.FeatureFlags{
		AgentV2Enabled:        true,
		StudentAgentV2Enabled: false, // disabled
		TeacherAgentEnabled:   true,
		AdminAgentEnabled:     true,
	}
	srv := buildFlagRouter(t, flags)

	resp := doRequest(t, srv, "GET", "/api/agent/sessions", testutil.StudentAToken(), nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TEST-T9.2-05: Master switch off — all roles get 503.
func TestT92_05_MasterSwitchOff(t *testing.T) {
	flags := middleware.FeatureFlags{
		AgentV2Enabled:        false, // master off
		StudentAgentV2Enabled: true,
		TeacherAgentEnabled:   true,
		AdminAgentEnabled:     true,
	}
	srv := buildFlagRouter(t, flags)

	tokens := []string{testutil.StudentAToken(), testutil.TeacherAToken(), testutil.AdminAToken()}
	for _, tok := range tokens {
		resp := doRequest(t, srv, "GET", "/api/agent/sessions", tok, nil)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("token %.10s...: expected 503, got %d", tok, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TEST-T9.2-06: Capabilities endpoint returns correct flag state.
func TestT92_06_CapabilitiesEndpoint(t *testing.T) {
	flags := middleware.FeatureFlags{
		AgentV2Enabled:         true,
		StudentAgentV2Enabled:  true,
		TeacherAgentEnabled:    false,
		AdminAgentEnabled:      true,
		AgentToolEventsEnabled: false,
	}
	srv := buildFlagRouter(t, flags)

	// Capabilities is public — no auth needed.
	resp, err := http.Get(srv.URL + "/api/capabilities")
	if err != nil {
		t.Fatalf("GET capabilities: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var caps map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
		t.Fatalf("decode: %v", err)
	}

	agentV2, ok := caps["agent_v2"].(map[string]any)
	if !ok {
		t.Fatal("missing agent_v2 in capabilities response")
	}

	if agentV2["enabled"] != true {
		t.Error("expected agent_v2.enabled = true")
	}
	if agentV2["student"] != true {
		t.Error("expected agent_v2.student = true")
	}
	if agentV2["teacher"] != false {
		t.Error("expected agent_v2.teacher = false (flag disabled)")
	}
	if agentV2["admin"] != true {
		t.Error("expected agent_v2.admin = true")
	}
	if caps["agent_tool_events"] != false {
		t.Error("expected agent_tool_events = false")
	}
}

// TEST-T9.2-07: Capabilities master off — all roles show disabled.
func TestT92_07_CapabilitiesMasterOff(t *testing.T) {
	flags := middleware.FeatureFlags{
		AgentV2Enabled:        false,
		StudentAgentV2Enabled: true,
		TeacherAgentEnabled:   true,
		AdminAgentEnabled:     true,
	}
	srv := buildFlagRouter(t, flags)

	resp, err := http.Get(srv.URL + "/api/capabilities")
	if err != nil {
		t.Fatalf("GET capabilities: %v", err)
	}
	defer resp.Body.Close()

	var caps map[string]any
	json.NewDecoder(resp.Body).Decode(&caps)

	agentV2 := caps["agent_v2"].(map[string]any)
	if agentV2["enabled"] != false {
		t.Error("expected agent_v2.enabled = false (master off)")
	}
	// When master is off, all role flags should show false.
	for _, role := range []string{"student", "teacher", "admin"} {
		if agentV2[role] != false {
			t.Errorf("expected agent_v2.%s = false when master off", role)
		}
	}
}

// TEST-T9.2-08: Full integration — student flag off via SetupTestApp's full router.
func TestT92_08_StudentFallback(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// The test app has all flags enabled, so student agent should work.
	resp := doRequest(t, app.Server, "GET", "/api/agent/sessions", testutil.StudentAToken(), nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with flags enabled, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify capabilities endpoint is accessible.
	capResp := doRequest(t, app.Server, "GET", "/api/capabilities", "", nil)
	if capResp.StatusCode != http.StatusOK {
		t.Errorf("capabilities: expected 200, got %d", capResp.StatusCode)
	}
	var caps map[string]any
	json.NewDecoder(capResp.Body).Decode(&caps)
	capResp.Body.Close()

	agentV2 := caps["agent_v2"].(map[string]any)
	if agentV2["enabled"] != true {
		t.Error("expected agent_v2.enabled = true in test app")
	}
}
