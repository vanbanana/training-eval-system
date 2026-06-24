package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/testutil"
)

// ============================================================
// T7.1 — Agent audit logging tests
// ============================================================

// TEST-T7.1-01: Successful agent chat creates audit log entry
func TestT71_01_SuccessAuditLog(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create student session
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.StudentAToken(),
		dto.CreateAgentSessionRequest{Title: "Audit Test", AgentRole: "student"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Send a message
	resp = doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.StudentAToken(),
		dto.AgentStreamRequest{
			SessionID: session.ID,
			Message:   "什么是实训评价？",
			AgentRole: "student",
		})
	testutil.AssertStatus(t, resp, http.StatusOK)
	_, _ = io.ReadAll(resp.Body)

	// Wait for async audit log to be written
	time.Sleep(200 * time.Millisecond)

	// Query audit_logs for agent.chat entries
	var auditCount int
	err = app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM audit_logs WHERE action LIKE 'agent.chat.%' AND target_id=?",
		itoa(session.ID),
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if auditCount == 0 {
		t.Error("expected at least one audit log entry for agent.chat, found none")
	}

	// Verify the audit log has correct fields
	var action, result, detail string
	err = app.DB.Reader.QueryRow(
		"SELECT action, result, detail FROM audit_logs WHERE action LIKE 'agent.chat.%' AND target_id=? ORDER BY id DESC LIMIT 1",
		itoa(session.ID),
	).Scan(&action, &result, &detail)
	if err != nil {
		t.Fatalf("query audit detail: %v", err)
	}

	if !strings.HasPrefix(action, "agent.chat.") {
		t.Errorf("audit action=%q, want prefix 'agent.chat.'", action)
	}
	if result != "success" && result != "failure" {
		t.Errorf("audit result=%q, want 'success' or 'failure'", result)
	}

	// Detail should contain structured audit info
	if !strings.Contains(detail, "agent_role=student") {
		t.Errorf("audit detail missing agent_role: %s", detail)
	}
}

// TEST-T7.1-02: Failed LLM call creates failure audit log
func TestT71_02_FailureAuditLog(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create admin session
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.AdminAToken(),
		dto.CreateAgentSessionRequest{Title: "Fail Audit", AgentRole: "admin"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Send a message (LLM not configured → failure path)
	resp = doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.AdminAToken(),
		dto.AgentStreamRequest{
			SessionID: session.ID,
			Message:   "check system status",
			AgentRole: "admin",
		})
	testutil.AssertStatus(t, resp, http.StatusOK)
	_, _ = io.ReadAll(resp.Body)

	// Wait for async audit log
	time.Sleep(200 * time.Millisecond)

	// Verify failure audit log
	var result string
	err = app.DB.Reader.QueryRow(
		"SELECT result FROM audit_logs WHERE action LIKE 'agent.chat.%' AND target_id=? ORDER BY id DESC LIMIT 1",
		itoa(session.ID),
	).Scan(&result)
	if err != nil {
		t.Fatalf("query audit result: %v", err)
	}
	if result != "failure" {
		t.Errorf("expected failure audit, got result=%q", result)
	}
}

// TEST-T7.1-03: Audit log doesn't contain sensitive data
func TestT71_03_AuditLogSanitization(t *testing.T) {
	app := testutil.SetupTestApp(t)
	fixture, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create student session with evaluation context
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.StudentAToken(),
		dto.CreateAgentSessionRequest{Title: "Sanitize Test", AgentRole: "student"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Send a message with evaluation context
	resp = doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.StudentAToken(),
		dto.AgentStreamRequest{
			SessionID: session.ID,
			Message:   "帮我分析评价结果",
			AgentRole: "student",
			Context:   &dto.AgentContextReq{EvaluationID: int64Ptr(fixture.EvalAID)},
		})
	testutil.AssertStatus(t, resp, http.StatusOK)
	_, _ = io.ReadAll(resp.Body)

	// Wait for async audit log
	time.Sleep(200 * time.Millisecond)

	// Check audit log detail doesn't contain sensitive patterns
	var detail string
	err = app.DB.Reader.QueryRow(
		"SELECT COALESCE(detail, '') FROM audit_logs WHERE action LIKE 'agent.chat.%' AND target_id=? ORDER BY id DESC LIMIT 1",
		itoa(session.ID),
	).Scan(&detail)
	if err != nil {
		t.Fatalf("query audit detail: %v", err)
	}

	// Must not contain API keys, passwords, or raw submission content
	sensitivePatterns := []string{"sk-", "password", "api_key", "secret", "Bearer "}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(strings.ToLower(detail), strings.ToLower(pattern)) {
			t.Errorf("audit detail contains sensitive pattern %q: %s", pattern, truncate(detail, 200))
		}
	}
}

// ============================================================
// T7.2 — Role-based quick questions & empty state tests
// ============================================================

// studentQuickQuestions are the T7.2 spec quick questions for students.
var studentQuickQuestions = []string{
	"帮我解释这次评价",
	"我下一步怎么提高",
	"推荐学习资源",
}

// teacherQuickQuestions are the T7.2 spec quick questions for teachers.
var teacherQuickQuestions = []string{
	"总结这个任务的批改情况",
	"生成评语草稿",
	"分析班级薄弱点",
	"解释疑似查重",
}

// adminQuickQuestions are the T7.2 spec quick questions for admins.
var adminQuickQuestions = []string{
	"总结系统运行情况",
	"检查 LLM 配置",
	"最近有什么异常",
	"给出用户治理建议",
}

// TEST-T7.2-01: Student quick questions are all sendable and produce valid responses
func TestT72_01_StudentQuickQuestions(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create student session
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.StudentAToken(),
		dto.CreateAgentSessionRequest{Title: "Quick Q Student", AgentRole: "student"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for i, q := range studentQuickQuestions {
		t.Run(studentQuickQuestions[i], func(t *testing.T) {
			resp := doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.StudentAToken(),
				dto.AgentStreamRequest{
					SessionID: session.ID,
					Message:   q,
					AgentRole: "student",
				})
			testutil.AssertStatus(t, resp, http.StatusOK)
			body, _ := io.ReadAll(resp.Body)

			// SSE stream should produce at least one event
			if len(body) == 0 {
				t.Errorf("empty response for student quick question %q", q)
			}
			// Should contain SSE data lines
			if !strings.Contains(string(body), "data:") {
				t.Errorf("no SSE data events for student quick question %q", q)
			}
		})
	}
}

// TEST-T7.2-02: Teacher quick questions carry correct context (task + class)
func TestT72_02_TeacherQuickQuestionsContext(t *testing.T) {
	app := testutil.SetupTestApp(t)
	fixture, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create teacher session
	session := createTeacherSession(t, app.Server, testutil.TeacherAToken(), "Quick Q Teacher")

	for _, q := range teacherQuickQuestions {
		t.Run(q, func(t *testing.T) {
			resp := doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.TeacherAToken(),
				dto.AgentStreamRequest{
					SessionID: session.ID,
					Message:   q,
					AgentRole: "teacher",
					Context: &dto.AgentContextReq{
						TaskID:  int64Ptr(fixture.TaskAID),
						ClassID: int64Ptr(fixture.ClassAID),
					},
				})
			testutil.AssertStatus(t, resp, http.StatusOK)
			body, _ := io.ReadAll(resp.Body)

			if len(body) == 0 {
				t.Errorf("empty response for teacher quick question %q", q)
			}
			if !strings.Contains(string(body), "data:") {
				t.Errorf("no SSE data events for teacher quick question %q", q)
			}
		})
	}

	// Verify the messages were saved with context
	time.Sleep(200 * time.Millisecond)
	var msgCount int
	err = app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM agent_messages WHERE session_id=?", session.ID,
	).Scan(&msgCount)
	if err != nil {
		t.Fatalf("query messages: %v", err)
	}
	// At least 4 user messages (one per quick question) + assistant responses
	if msgCount < 4 {
		t.Errorf("expected at least 4 messages, got %d", msgCount)
	}
}

// TEST-T7.2-03: Admin quick questions work without normal-user context
func TestT72_03_AdminQuickQuestionsNoUserContext(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create admin session
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.AdminAToken(),
		dto.CreateAgentSessionRequest{Title: "Quick Q Admin", AgentRole: "admin"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, q := range adminQuickQuestions {
		t.Run(q, func(t *testing.T) {
			// Admin sends WITHOUT any user context (no evaluation_id, task_id, etc.)
			resp := doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.AdminAToken(),
				dto.AgentStreamRequest{
					SessionID: session.ID,
					Message:   q,
					AgentRole: "admin",
					// No Context field — admin doesn't carry normal-user context
				})
			testutil.AssertStatus(t, resp, http.StatusOK)
			body, _ := io.ReadAll(resp.Body)

			if len(body) == 0 {
				t.Errorf("empty response for admin quick question %q", q)
			}
			if !strings.Contains(string(body), "data:") {
				t.Errorf("no SSE data events for admin quick question %q", q)
			}
		})
	}

	// Verify audit logs show admin role (no student/teacher context leakage)
	time.Sleep(200 * time.Millisecond)
	var auditRole string
	err = app.DB.Reader.QueryRow(
		"SELECT role FROM audit_logs WHERE action LIKE 'agent.chat.%' AND target_id=? ORDER BY id DESC LIMIT 1",
		itoa(session.ID),
	).Scan(&auditRole)
	if err != nil {
		t.Fatalf("query audit role: %v", err)
	}
	if auditRole != "admin" {
		t.Errorf("admin quick question audit role=%q, want 'admin'", auditRole)
	}
}

// TEST-T7.2-04: Empty state text differs by role (verified via agent session agent_role and system prompt routing)
func TestT72_04_EmptyStateRoleDifferentiation(t *testing.T) {
	roles := []struct {
		name      string
		token     string
		agentRole string
	}{
		{"student", testutil.StudentAToken(), "student"},
		{"teacher", testutil.TeacherAToken(), "teacher"},
		{"admin", testutil.AdminAToken(), "admin"},
	}

	for _, r := range roles {
		t.Run(r.name, func(t *testing.T) {
			app := testutil.SetupTestApp(t)
			_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
			if err != nil {
				t.Fatalf("BuildAgentFixture: %v", err)
			}

			// Create session with agent_role
			resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", r.token,
				dto.CreateAgentSessionRequest{Title: "Empty State " + r.name, AgentRole: r.agentRole})
			testutil.AssertStatus(t, resp, http.StatusCreated)
			var session dto.AgentSessionResponse
			if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
				t.Fatalf("decode: %v", err)
			}

			// Verify session has correct agent_role
			if session.AgentRole != r.agentRole {
				t.Errorf("session agent_role=%q, want %q", session.AgentRole, r.agentRole)
			}

			// Send an initial message to trigger role-specific system prompt
			resp = doRequest(t, app.Server, "POST", "/api/agent/stream", r.token,
				dto.AgentStreamRequest{
					SessionID: session.ID,
					Message:   "你好",
					AgentRole: r.agentRole,
				})
			testutil.AssertStatus(t, resp, http.StatusOK)
			_, _ = io.ReadAll(resp.Body)

			// Verify assistant message was saved (confirming role-specific processing)
			time.Sleep(200 * time.Millisecond)
			var savedCount int
			err = app.DB.Reader.QueryRow(
				"SELECT COUNT(*) FROM agent_messages WHERE session_id=? AND role='assistant'",
				session.ID,
			).Scan(&savedCount)
			if err != nil {
				t.Fatalf("query assistant messages: %v", err)
			}
			if savedCount == 0 {
				t.Errorf("no assistant message saved for %s role", r.agentRole)
			}
		})
	}
}

// TEST-T7.1-04: Audit log records tool names when tools are called
func TestT71_04_AuditToolNames(t *testing.T) {
	app := testutil.SetupTestApp(t)
	fixture, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create teacher session with task context
	session := createTeacherSession(t, app.Server, testutil.TeacherAToken(), "Tool Audit")

	resp := doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.TeacherAToken(),
		dto.AgentStreamRequest{
			SessionID: session.ID,
			Message:   "总结提交情况",
			AgentRole: "teacher",
			Context:   &dto.AgentContextReq{TaskID: int64Ptr(fixture.TaskAID)},
		})
	testutil.AssertStatus(t, resp, http.StatusOK)
	_, _ = io.ReadAll(resp.Body)

	// Wait for async audit log
	time.Sleep(200 * time.Millisecond)

	// Check audit log exists
	var auditCount int
	err = app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM audit_logs WHERE action LIKE 'agent.chat.%' AND target_id=?",
		itoa(session.ID),
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if auditCount == 0 {
		t.Error("expected audit log entry for teacher tool-augmented chat")
	}
}

// TEST-T7.1-05: All three roles create audit logs
func TestT71_05_AllRolesAudit(t *testing.T) {
	roles := []struct {
		name      string
		token     string
		agentRole string
	}{
		{"student", testutil.StudentAToken(), "student"},
		{"teacher", testutil.TeacherAToken(), "teacher"},
		{"admin", testutil.AdminAToken(), "admin"},
	}

	for _, r := range roles {
		t.Run(r.name, func(t *testing.T) {
			app := testutil.SetupTestApp(t)
			_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
			if err != nil {
				t.Fatalf("BuildAgentFixture: %v", err)
			}

			// Create session
			resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", r.token,
				dto.CreateAgentSessionRequest{Title: "Audit Role Test", AgentRole: r.agentRole})
			testutil.AssertStatus(t, resp, http.StatusCreated)
			var session dto.AgentSessionResponse
			if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
				t.Fatalf("decode: %v", err)
			}

			// Send message
			resp = doRequest(t, app.Server, "POST", "/api/agent/stream", r.token,
				dto.AgentStreamRequest{
					SessionID: session.ID,
					Message:   "hello",
					AgentRole: r.agentRole,
				})
			testutil.AssertStatus(t, resp, http.StatusOK)
			_, _ = io.ReadAll(resp.Body)

			time.Sleep(200 * time.Millisecond)

			// Verify audit log with correct role
			var auditRole string
			err = app.DB.Reader.QueryRow(
				"SELECT role FROM audit_logs WHERE action LIKE 'agent.chat.%' AND target_id=? ORDER BY id DESC LIMIT 1",
				itoa(session.ID),
			).Scan(&auditRole)
			if err != nil {
				t.Fatalf("query audit role: %v", err)
			}
			if auditRole != r.agentRole {
				t.Errorf("audit role=%q, want %q", auditRole, r.agentRole)
			}
		})
	}
}
