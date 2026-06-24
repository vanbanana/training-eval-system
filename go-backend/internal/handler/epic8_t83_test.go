package handler_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/testutil"
)

// ============================================================
// T8.3 — Token cost, usage tracking, and admin reporting tests
// ============================================================

// TEST-T8.3-01: Token usage record is persisted and queryable via summary
func TestT83_01_TokenUsageRecorded(t *testing.T) {
	app := testutil.SetupTestApp(t)

	// Insert a usage record directly into the token_usage table
	now := time.Now().Format(time.RFC3339)
	_, err := app.DB.Writer.Exec(
		`INSERT INTO token_usage (user_id, user_role, agent_role, session_id, model, provider,
			prompt_tokens, completion_tokens, total_tokens, tool_call_count, success,
			latency_ms, cost_status, estimated_cost, error_code, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		13, "student", "student", 1,
		"test-model", "test-provider",
		100, 50, 150, 2, 1,
		500, "unknown", 0, "", now,
	)
	if err != nil {
		t.Fatalf("insert token usage: %v", err)
	}

	// Query the summary via the HTTP API (admin only)
	resp := doRequest(t, app.Server, "GET", "/api/usage/summary", testutil.AdminAToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	resp.Body.Close()

	// Verify summary section
	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatal("expected summary to be a map")
	}
	totalReq := toFloat(summary["total_requests"])
	if totalReq < 1 {
		t.Errorf("expected total_requests >= 1, got %v", totalReq)
	}
	totalTokens := toFloat(summary["total_tokens"])
	if totalTokens < 150 {
		t.Errorf("expected total_tokens >= 150, got %v", totalTokens)
	}
	promptTokens := toFloat(summary["total_prompt_tokens"])
	if promptTokens < 100 {
		t.Errorf("expected total_prompt_tokens >= 100, got %v", promptTokens)
	}
	completionTokens := toFloat(summary["total_completion_tokens"])
	if completionTokens < 50 {
		t.Errorf("expected total_completion_tokens >= 50, got %v", completionTokens)
	}

	t.Logf("T8.3-01 usage summary: total_requests=%.0f total_tokens=%.0f", totalReq, totalTokens)
}

// TEST-T8.3-02: Cost status is "unknown" when model pricing is not configured
func TestT83_02_CostUnknown(t *testing.T) {
	app := testutil.SetupTestApp(t)

	// Insert usage records without cost data (as the system would naturally do)
	now := time.Now().Format(time.RFC3339)
	for i := 0; i < 3; i++ {
		_, err := app.DB.Writer.Exec(
			`INSERT INTO token_usage (user_id, user_role, agent_role, session_id, model, provider,
				prompt_tokens, completion_tokens, total_tokens, tool_call_count, success,
				latency_ms, cost_status, estimated_cost, error_code, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			13, "student", "student", int64(i+1),
			"unknown-model", "",
			200, 100, 300, 0, 1,
			200, "unknown", 0, "", now,
		)
		if err != nil {
			t.Fatalf("insert token usage %d: %v", i, err)
		}
	}

	// Query the summary
	resp := doRequest(t, app.Server, "GET", "/api/usage/summary", testutil.AdminAToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()

	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatal("expected summary to be a map")
	}

	costStatus, _ := summary["cost_status"].(string)
	if costStatus != "unknown" {
		t.Errorf("expected cost_status = 'unknown', got %q", costStatus)
	}

	estimatedCost := toFloat(summary["total_estimated_cost"])
	if estimatedCost != 0 {
		t.Errorf("expected total_estimated_cost = 0 (no pricing), got %v", estimatedCost)
	}

	t.Logf("T8.3-02 cost_status=%q estimated_cost=%.2f (correctly unknown)", costStatus, estimatedCost)
}

// TEST-T8.3-03: Admin usage summary returns all three aggregation sections
func TestT83_03_AdminUsageSummary(t *testing.T) {
	app := testutil.SetupTestApp(t)

	// Insert usage records for multiple roles
	now := time.Now().Format(time.RFC3339)
	records := []struct {
		userID int64
		role   string
		tokens int
	}{
		{13, "student", 500},
		{14, "student", 300},
		{11, "teacher", 800},
		{10, "admin", 200},
	}

	for i, r := range records {
		_, err := app.DB.Writer.Exec(
			`INSERT INTO token_usage (user_id, user_role, agent_role, session_id, model, provider,
				prompt_tokens, completion_tokens, total_tokens, tool_call_count, success,
				latency_ms, cost_status, estimated_cost, error_code, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.userID, r.role, r.role, int64(i+1),
			"test-model", "test",
			r.tokens/2, r.tokens/2, r.tokens, 0, 1,
			100, "unknown", 0, "", now,
		)
		if err != nil {
			t.Fatalf("insert record %d: %v", i, err)
		}
	}

	// Query the usage summary
	resp := doRequest(t, app.Server, "GET", "/api/usage/summary?period=week", testutil.AdminAToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()

	// Verify all three sections exist
	if _, ok := result["summary"]; !ok {
		t.Error("missing 'summary' section")
	}
	if _, ok := result["by_role"]; !ok {
		t.Error("missing 'by_role' section")
	}
	if _, ok := result["top_users"]; !ok {
		t.Error("missing 'top_users' section")
	}

	// Verify summary has correct total
	summary := result["summary"].(map[string]any)
	totalTokens := toFloat(summary["total_tokens"])
	if totalTokens < 1800 { // 500+300+800+200
		t.Errorf("expected total_tokens >= 1800, got %v", totalTokens)
	}

	// Verify by_role has multiple roles
	byRole, ok := result["by_role"].([]any)
	if !ok {
		t.Fatal("expected by_role to be an array")
	}
	if len(byRole) < 3 { // student, teacher, admin
		t.Errorf("expected at least 3 roles in by_role, got %d", len(byRole))
	}

	// Verify top_users has entries
	topUsers, ok := result["top_users"].([]any)
	if !ok {
		t.Fatal("expected top_users to be an array")
	}
	if len(topUsers) < 1 {
		t.Error("expected at least 1 top user")
	}

	t.Logf("T8.3-03 admin usage summary: total_tokens=%.0f roles=%d top_users=%d",
		totalTokens, len(byRole), len(topUsers))
}

// TEST-T8.3-04: Usage summary never leaks API keys or full prompts
func TestT83_04_UsageNoSecrets(t *testing.T) {
	app := testutil.SetupTestApp(t)

	// Insert usage records with potentially sensitive data in various fields
	now := time.Now().Format(time.RFC3339)
	_, err := app.DB.Writer.Exec(
		`INSERT INTO token_usage (user_id, user_role, agent_role, session_id, model, provider,
			prompt_tokens, completion_tokens, total_tokens, tool_call_count, success,
			latency_ms, cost_status, estimated_cost, error_code, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		13, "student", "student", 1,
		"my-secret-model", "openai",
		1000, 500, 1500, 0, 1,
		300, "unknown", 0, "some-error", now,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Query the summary
	resp := doRequest(t, app.Server, "GET", "/api/usage/summary", testutil.AdminAToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var rawBody json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rawBody); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()

	// Marshal the entire response to a string for secret scanning
	responseStr := string(rawBody)

	// Verify the response does NOT contain common sensitive patterns
	sensitivePatterns := []string{
		"api_key", "apikey", "api-key",
		"sk-", "sk_", "Bearer ",
		"password", "secret",
		"prompt_text", "user_message", "full_prompt",
	}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(strings.ToLower(responseStr), strings.ToLower(pattern)) {
			t.Errorf("response contains potentially sensitive pattern: %q", pattern)
		}
	}

	// Verify the schema only contains aggregate fields, not individual records
	var result map[string]any
	if err := json.Unmarshal(rawBody, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Summary should have aggregated fields only
	summary, _ := result["summary"].(map[string]any)
	forbiddenFields := []string{"prompt", "message", "content", "api_key", "token_value"}
	for _, field := range forbiddenFields {
		if _, exists := summary[field]; exists {
			t.Errorf("summary contains forbidden field: %q", field)
		}
	}

	t.Logf("T8.3-04 verified: no sensitive data in usage summary response (%d bytes)", len(responseStr))
}

// TEST-T8.3-05: Non-admin users cannot access usage summary
func TestT83_05_UsageAdminOnly(t *testing.T) {
	app := testutil.SetupTestApp(t)

	// Student should get 403
	resp := doRequest(t, app.Server, "GET", "/api/usage/summary", testutil.StudentAToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()

	// Teacher should get 403
	resp = doRequest(t, app.Server, "GET", "/api/usage/summary", testutil.TeacherAToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()
}

// toFloat converts a JSON number (float64) to float64 for assertions.
func toFloat(v any) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}
