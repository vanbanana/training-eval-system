package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/testutil"
)

// TEST-T9.3-01: Staging smoke — student, teacher, admin all complete shortest conversation path.
func TestT93_01_StagingSmoke(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	roles := []struct {
		token string
		role  string
	}{
		{testutil.StudentAToken(), "student"},
		{testutil.TeacherAToken(), "teacher"},
		{testutil.AdminAToken(), "admin"},
	}

	for _, r := range roles {
		t.Run(r.role, func(t *testing.T) {
			// Create session.
			resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", r.token,
				dto.CreateAgentSessionRequest{
					Title:     "Smoke Test - " + r.role,
					AgentRole: r.role,
				})
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("%s: create session expected 201, got %d", r.role, resp.StatusCode)
			}
			var session dto.AgentSessionResponse
			json.NewDecoder(resp.Body).Decode(&session)
			resp.Body.Close()

			// List sessions.
			resp = doRequest(t, app.Server, "GET", "/api/agent/sessions", r.token, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: list sessions expected 200, got %d", r.role, resp.StatusCode)
			}
			var sessions []dto.AgentSessionResponse
			json.NewDecoder(resp.Body).Decode(&sessions)
			resp.Body.Close()

			if len(sessions) == 0 {
				t.Fatalf("%s: expected at least 1 session", r.role)
			}

			// Get messages (should be empty for new session).
			resp = doRequest(t, app.Server, "GET",
				"/api/agent/sessions/"+intToStr(session.ID)+"/messages", r.token, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: get messages expected 200, got %d", r.role, resp.StatusCode)
			}
			resp.Body.Close()
		})
	}
}

// TEST-T9.3-02: Rollback drill — disabling AGENT_V2 hides agent routes, old chat still works.
func TestT93_02_RollbackDrill(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Verify agent works with flags enabled (default in test setup).
	resp := doRequest(t, app.Server, "GET", "/api/agent/sessions", testutil.StudentAToken(), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pre-rollback: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Build a separate router with AgentV2 disabled (simulating rollback).
	rollbackRouter := buildFlagRouter(t, middleware.FeatureFlags{
		AgentV2Enabled:        false,
		StudentAgentV2Enabled: true,
		TeacherAgentEnabled:   true,
		AdminAgentEnabled:     true,
	})

	// Verify agent routes return 503.
	for _, tok := range []string{testutil.StudentAToken(), testutil.TeacherAToken(), testutil.AdminAToken()} {
		resp := doRequest(t, rollbackRouter, "GET", "/api/agent/sessions", tok, nil)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("rollback: expected 503 for agent, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Verify old chat routes still work on the main app (not the rollback router).
	resp = doRequest(t, app.Server, "GET", "/api/chat/sessions", testutil.StudentAToken(), nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("rollback: legacy chat expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify capabilities endpoint shows disabled state.
	resp, err = http.Get(rollbackRouter.URL + "/api/capabilities")
	if err != nil {
		t.Fatalf("GET capabilities: %v", err)
	}
	var caps map[string]any
	json.NewDecoder(resp.Body).Decode(&caps)
	resp.Body.Close()

	agentV2 := caps["agent_v2"].(map[string]any)
	if agentV2["enabled"] != false {
		t.Error("rollback: expected agent_v2.enabled = false")
	}
}

// TEST-T9.3-03: Log redaction — sensitive data must not appear in application logs.
func TestT93_03_LogsRedaction(t *testing.T) {
	// Capture log output into a buffer.
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	defer slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) // reset after test

	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Perform various API calls that might log sensitive data.
	// Login with credentials.
	loginBody := map[string]string{"username": "student_a", "password": "password123"}
	resp := doRequest(t, app.Server, "POST", "/api/auth/login", "", loginBody)
	resp.Body.Close()

	// Access agent endpoints with auth token.
	resp = doRequest(t, app.Server, "GET", "/api/agent/sessions", testutil.StudentAToken(), nil)
	resp.Body.Close()

	// Access LLM configs (admin only, might contain keys).
	resp = doRequest(t, app.Server, "GET", "/api/llm/configs", testutil.AdminAToken(), nil)
	resp.Body.Close()

	// Check log output for sensitive patterns.
	logOutput := logBuf.String()
	sensitivePatterns := []string{
		"password123",
		"sk-",        // API key prefix
		"Bearer ey",  // JWT token in logs
		"api_key",    // raw key field name with value
		"secret_key", // secret key value
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(logOutput, pattern) {
			t.Errorf("log redaction failure: sensitive pattern %q found in log output", pattern)
		}
	}
}

// intToStr converts int64 to string for URL path construction.
func intToStr(n int64) string {
	return fmt.Sprintf("%d", n)
}
