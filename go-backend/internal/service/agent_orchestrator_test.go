package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestT13_01_UnknownRole(t *testing.T) {
	orch := NewRoleAgentOrchestrator(nil)
	err := orch.Validate(AgentContext{
		UserID:    1,
		UserRole:  "student",
		AgentRole: "parent",
		SessionID: 1,
	})
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
	if !errors.Is(err, ErrUnsupportedAgentRole) {
		t.Errorf("expected ErrUnsupportedAgentRole, got %v", err)
	}
}

func TestT13_02_LLMNilFallback(t *testing.T) {
	orch := NewRoleAgentOrchestrator(nil)
	rec := httptest.NewRecorder()

	resp, err := orch.Stream(context.Background(), AgentContext{
		UserID:    1,
		UserRole:  "student",
		AgentRole: "student",
		SessionID: 1,
	}, "hello", nil, rec)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Content != "AI 助手暂未配置" {
		t.Errorf("expected fallback message, got %q", resp.Content)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"type":"text"`) {
		t.Error("SSE should contain type=text event")
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Error("SSE should contain type=done event")
	}
	if !strings.Contains(body, "AI 助手暂未配置") {
		t.Error("SSE text content should contain fallback message")
	}
}

func TestT13_03_RoleDispatch(t *testing.T) {
	// Test that each valid role produces the correct system prompt
	roles := []struct {
		role     string
		contains string
	}{
		{"student", "学生"},
		{"teacher", "教师"},
		{"admin", "管理员"},
	}

	for _, tc := range roles {
		t.Run(tc.role, func(t *testing.T) {
			prompt := buildSystemPrompt(AgentContext{AgentRole: tc.role})
			if !strings.Contains(prompt, tc.contains) {
				t.Errorf("role=%q prompt should contain %q, got %q", tc.role, tc.contains, prompt)
			}
		})
	}

	// Test context injection
	evalID := int64(42)
	taskID := int64(100)
	prompt := buildSystemPrompt(AgentContext{
		AgentRole:    "student",
		EvaluationID: &evalID,
		TaskID:       &taskID,
	})
	if !strings.Contains(prompt, "42") {
		t.Error("student prompt should contain evaluation_id 42")
	}
	if !strings.Contains(prompt, "100") {
		t.Error("student prompt should contain task_id 100")
	}
}

func TestT13_04_AllRolesValid(t *testing.T) {
	orch := NewRoleAgentOrchestrator(nil)
	for _, role := range []string{"student", "teacher", "admin"} {
		err := orch.Validate(AgentContext{AgentRole: role})
		if err != nil {
			t.Errorf("role %q should be valid, got error: %v", role, err)
		}
	}
}

func TestT13_05_LLMNilFallbackAllRoles(t *testing.T) {
	orch := NewRoleAgentOrchestrator(nil)
	for _, role := range []string{"student", "teacher", "admin"} {
		t.Run(role, func(t *testing.T) {
			rec := httptest.NewRecorder()
			resp, err := orch.Stream(context.Background(), AgentContext{
				UserID:    1,
				UserRole:  role,
				AgentRole: role,
				SessionID: 1,
			}, "test", nil, rec)
			if err != nil {
				t.Fatalf("role %q: unexpected error: %v", role, err)
			}
			if resp == nil {
				t.Fatalf("role %q: expected non-nil response", role)
			}

			body := rec.Body.String()
			if !strings.Contains(body, `"type":"done"`) {
				t.Errorf("role %q: SSE should contain type=done", role)
			}
		})
	}
}

func TestT13_06_ValidateRejectsEmptyRole(t *testing.T) {
	orch := NewRoleAgentOrchestrator(nil)
	err := orch.Validate(AgentContext{AgentRole: ""})
	if err == nil {
		t.Fatal("expected error for empty role")
	}
	if !errors.Is(err, ErrUnsupportedAgentRole) {
		t.Errorf("expected ErrUnsupportedAgentRole, got %v", err)
	}
}

// Ensure Stream sets correct SSE headers when llmClient is nil
func TestT13_07_StreamSSEFormat(t *testing.T) {
	orch := NewRoleAgentOrchestrator(nil)
	rec := httptest.NewRecorder()

	_, err := orch.Stream(context.Background(), AgentContext{
		UserID:    1,
		UserRole:  "student",
		AgentRole: "student",
		SessionID: 1,
	}, "hello", nil, rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check SSE data format: each line starts with "data: "
	body := rec.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			t.Errorf("SSE line should start with 'data: ', got %q", line)
		}
	}

	_ = http.StatusOK // keep import
}

// ============================================================
// T3.2 — Teacher Agent System Prompt tests
// ============================================================

// TestT32_01_PromptContainsTeacherBoundaries verifies the teacher prompt includes
// the required boundary keywords: draft-only, teacher confirmation, no fabrication.
func TestT32_01_PromptContainsTeacherBoundaries(t *testing.T) {
	prompt := buildSystemPrompt(AgentContext{AgentRole: "teacher"})

	keywords := []struct {
		keyword string
		desc    string
	}{
		{"草稿", "draft-only output"},
		{"需教师确认", "teacher confirmation required"},
		{"不编造", "no fabrication"},
		{"疑似", "suspected plagiarism wording"},
		{"需要复核", "needs review wording"},
		{"当前没有数据", "no-data explicit statement"},
		{"侮辱性", "respectful language"},
		{"统计范围", "scope notation for statistics"},
	}
	for _, kw := range keywords {
		if !strings.Contains(prompt, kw.keyword) {
			t.Errorf("teacher prompt missing %q (%s)", kw.keyword, kw.desc)
		}
	}
}

// TestT32_02_PromptWithContext verifies the teacher prompt with context includes
// context info but does NOT contain API keys or secrets.
func TestT32_02_PromptWithContext(t *testing.T) {
	taskID := int64(200)
	classID := int64(300)
	courseID := int64(400)
	prompt := buildSystemPrompt(AgentContext{
		AgentRole: "teacher",
		TaskID:    &taskID,
		ClassID:   &classID,
		CourseID:  &courseID,
	})

	// Context IDs should be present
	if !strings.Contains(prompt, "200") {
		t.Error("prompt should contain task_id 200")
	}
	if !strings.Contains(prompt, "300") {
		t.Error("prompt should contain class_id 300")
	}
	if !strings.Contains(prompt, "400") {
		t.Error("prompt should contain course_id 400")
	}

	// Sensitive content should NOT be present
	for _, secret := range []string{"api_key", "API_KEY", "密钥", "secret", "token"} {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(secret)) {
			t.Errorf("prompt should NOT contain sensitive term %q", secret)
		}
	}
}

// TestT32_03_NoContextPrompt verifies the teacher prompt without context
// does not contain any business data IDs.
func TestT32_03_NoContextPrompt(t *testing.T) {
	prompt := buildSystemPrompt(AgentContext{AgentRole: "teacher"})

	// Should contain the role description
	if !strings.Contains(prompt, "教师端实训评价助手") {
		t.Error("no-context prompt should still contain role description")
	}

	// Should NOT contain any context-specific IDs (no "ID=" pattern)
	if strings.Contains(prompt, "ID=") {
		t.Error("no-context prompt should not contain any business data IDs")
	}

	// Should NOT contain specific database references
	for _, forbidden := range []string{"关联任务", "关联班级", "关联课程", "关联评价"} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("no-context prompt should not contain %q", forbidden)
		}
	}
}
