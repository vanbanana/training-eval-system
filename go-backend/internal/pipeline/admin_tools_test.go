// Package pipeline tests for admin tools (T4.1–T4.4).
// Uses inline mocks to avoid circular import with testutil.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// toGenericJSON converts a ToolResult.Data to a generic map via JSON round-trip.
func toGenericJSON(data any) map[string]any {
	b, _ := json.Marshal(data)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// ============================================================
// Inline mock repos for admin tool tests
// ============================================================

// mockUserRepo implements repository.UserRepo for admin tool tests.
type mockUserRepo struct {
	repository.UserRepo
	users []model.User
}

func (m *mockUserRepo) GetByID(_ context.Context, id int64) (*model.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, errNotFound
}

func (m *mockUserRepo) GetByUsername(_ context.Context, username string) (*model.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return &u, nil
		}
	}
	return nil, errNotFound
}

func (m *mockUserRepo) List(_ context.Context, _ repository.ListParams) ([]model.User, int64, error) {
	return m.users, int64(len(m.users)), nil
}

// mockLLMConfigRepo implements repository.LLMConfigRepo for admin tool tests.
type mockLLMConfigRepo struct {
	repository.LLMConfigRepo
	config *model.LLMConfig
}

func (m *mockLLMConfigRepo) GetActive(_ context.Context) (*model.LLMConfig, error) {
	if m.config != nil && m.config.IsActive {
		return m.config, nil
	}
	return nil, errNotFound
}

// mockAuditRepo implements repository.AuditRepo for admin tool tests.
type mockAuditRepo struct {
	repository.AuditRepo
	logs []model.AuditLog
}

func (m *mockAuditRepo) List(_ context.Context, _ repository.ListParams, userID *int64, action *string) ([]model.AuditLog, int64, error) {
	var result []model.AuditLog
	for _, l := range m.logs {
		if userID != nil && (l.UserID == nil || *l.UserID != *userID) {
			continue
		}
		if action != nil && l.Action != *action {
			continue
		}
		result = append(result, l)
	}
	return result, int64(len(result)), nil
}

// ============================================================
// T4.1 — Admin Context Permission & Security Tests
// ============================================================

func TestT41_01_AdminToolSchemas(t *testing.T) {
	schemas := AdminToolSchemas()
	if len(schemas) != 12 {
		t.Fatalf("expected 12 admin tools, got %d", len(schemas))
	}

	expectedNames := map[string]bool{
		"admin_get_system_overview":             false,
		"admin_get_usage_metrics":               false,
		"admin_check_llm_status":                false,
		"admin_get_recent_failures":             false,
		"admin_get_user_summary":                false,
		"admin_find_inactive_users":             false,
		"admin_get_course_class_summary":        false,
		"admin_generate_governance_suggestions": false,
		"admin_search_audit_logs":               false,
		"admin_summarize_audit_anomalies":       false,
		"admin_explain_user_activity":           false,
		"admin_get_ai_usage_summary":            false,
	}

	for _, s := range schemas {
		name := s.Function.Name
		if _, ok := expectedNames[name]; !ok {
			t.Errorf("unexpected tool name: %s", name)
		}
		expectedNames[name] = true
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestT41_02_AdminToolUnknownName(t *testing.T) {
	co := &ChatOrchestrator{}
	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_unknown", map[string]any{}, actx)
	if result.Success {
		t.Error("expected failure for unknown tool")
	}
	if !containsStr(result.Error, "unknown") {
		t.Errorf("error should mention 'unknown', got: %s", result.Error)
	}
}

func TestT41_03_AdminPromptNoSecrets(t *testing.T) {
	// Verify admin tools never return API key or encrypted blob
	co := &ChatOrchestrator{
		evalRepo:   &mockEvalRepo{evals: []model.Evaluation{}},
		uploadRepo: &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{}},
		taskRepo:   &mockTaskRepoForTeacher{tasks: map[int64]*model.TrainingTask{}},
		llmConfigRepo: &mockLLMConfigRepo{
			config: &model.LLMConfig{
				ID:              1,
				Provider:        "deepseek",
				BaseURL:         "https://api.deepseek.com",
				APIKeyEncrypted: "encrypted_blob_should_not_appear",
				ChatModel:       "deepseek-chat",
				IsActive:        true,
			},
		},
	}

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_check_llm_status", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if data["status"].(string) != "active" {
		t.Errorf("expected status='active', got %v", data["status"])
	}
	// Verify no API key in any field
	for k, v := range data {
		if s, ok := v.(string); ok {
			if containsStr(s, "encrypted_blob") || containsStr(s, "sk-") {
				t.Errorf("field %q contains sensitive data: %s", k, s)
			}
		}
	}
}

// ============================================================
// T4.2 — System Overview & Health Check Tests
// ============================================================

func TestT42_01_SystemOverview(t *testing.T) {
	users := []model.User{
		{ID: 1, Username: "admin1", Role: "admin", IsActive: true},
		{ID: 2, Username: "teacher1", Role: "teacher", IsActive: true},
		{ID: 3, Username: "student1", Role: "student", IsActive: true},
		{ID: 4, Username: "student2", Role: "student", IsActive: true},
	}

	co := &ChatOrchestrator{
		userRepo:   &mockUserRepo{users: users},
		evalRepo:   &mockEvalRepo{evals: []model.Evaluation{}},
		uploadRepo: &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{}},
		taskRepo:   &mockTaskRepoForTeacher{tasks: map[int64]*model.TrainingTask{}},
	}

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_get_system_overview", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if data["user_count"].(int) != 4 {
		t.Errorf("expected user_count=4, got %v", data["user_count"])
	}
	if data["admin_count"].(int) != 1 {
		t.Errorf("expected admin_count=1, got %v", data["admin_count"])
	}
	if data["teacher_count"].(int) != 1 {
		t.Errorf("expected teacher_count=1, got %v", data["teacher_count"])
	}
	if data["student_count"].(int) != 2 {
		t.Errorf("expected student_count=2, got %v", data["student_count"])
	}
}

func TestT42_02_LLMStatusNoConfig(t *testing.T) {
	co := &ChatOrchestrator{
		llmConfigRepo: &mockLLMConfigRepo{config: nil},
	}

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_check_llm_status", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if data["status"].(string) != "missing" {
		t.Errorf("expected status='missing', got %v", data["status"])
	}
}

func TestT42_03_LLMStatusNoRepo(t *testing.T) {
	co := &ChatOrchestrator{} // no llmConfigRepo

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_check_llm_status", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if data["status"].(string) != "unknown" {
		t.Errorf("expected status='unknown', got %v", data["status"])
	}
}

func TestT42_04_RecentFailuresRedaction(t *testing.T) {
	uid := int64(5)
	logs := []model.AuditLog{
		{ID: 1, OccurredAt: time.Now(), UserID: &uid, Username: "user1", Action: "failure", Result: "failure", Detail: "login failed with password=secret123"},
	}

	co := &ChatOrchestrator{
		auditRepo: &mockAuditRepo{logs: logs},
	}

	actx := &AdminToolContext{AdminID: 1}
	args := map[string]any{"limit": float64(20)}
	result := co.DispatchAdminTool(context.Background(), "admin_get_recent_failures", args, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := toGenericJSON(result.Data)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	// Detail should be truncated and should not contain full password
	item0 := items[0].(map[string]any)
	detail := item0["detail"].(string)
	if containsStr(detail, "password=secret123") {
		t.Error("detail should be redacted, not contain password")
	}
}

func TestT42_05_UsageMetrics(t *testing.T) {
	score := 85.0
	evals := []model.Evaluation{
		{ID: 1, Status: "confirmed", TotalScore: &score},
		{ID: 2, Status: "scored", TotalScore: &score},
	}

	co := &ChatOrchestrator{
		uploadRepo: &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{
			1: {{ID: 100}, {ID: 101}},
		}},
		evalRepo: &mockEvalRepo{evals: evals},
	}

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_get_usage_metrics", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if data["eval_count"].(int64) != 2 {
		t.Errorf("expected eval_count=2, got %v", data["eval_count"])
	}
	if data["confirmed_count"].(int) != 1 {
		t.Errorf("expected confirmed_count=1, got %v", data["confirmed_count"])
	}
}

// ============================================================
// T4.3 — User/Course/Class Governance Tests
// ============================================================

func TestT43_01_UserSummary(t *testing.T) {
	users := []model.User{
		{ID: 1, Role: "admin", IsActive: true},
		{ID: 2, Role: "teacher", IsActive: true},
		{ID: 3, Role: "student", IsActive: true},
		{ID: 4, Role: "student", IsActive: false},
	}

	co := &ChatOrchestrator{
		userRepo: &mockUserRepo{users: users},
	}

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_get_user_summary", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if data["total"].(int64) != 4 {
		t.Errorf("expected total=4, got %v", data["total"])
	}
	if data["active_count"].(int) != 3 {
		t.Errorf("expected active_count=3, got %v", data["active_count"])
	}
	if data["inactive_count"].(int) != 1 {
		t.Errorf("expected inactive_count=1, got %v", data["inactive_count"])
	}
	if !containsStr(data["note"].(string), "不会自动") {
		t.Error("note should clarify no auto-action")
	}
}

func TestT43_02_PasswordHashRedaction(t *testing.T) {
	users := []model.User{
		{ID: 1, Username: "admin1", PasswordHash: "$2a$10$hashshouldnotappear", Role: "admin", IsActive: true},
	}

	co := &ChatOrchestrator{
		userRepo: &mockUserRepo{users: users},
	}

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_get_user_summary", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	// Serialize result to string and check no hash appears
	data := result.Data.(map[string]any)
	for _, v := range data {
		if s, ok := v.(string); ok {
			if containsStr(s, "$2a$10$") {
				t.Error("result should not contain password hash")
			}
		}
	}
}

func TestT43_03_InactiveUsersLimit(t *testing.T) {
	var users []model.User
	oldTime := time.Now().AddDate(0, 0, -60)
	for i := int64(1); i <= 50; i++ {
		users = append(users, model.User{
			ID: i, Username: fmt.Sprintf("user%d", i), Role: "student",
			IsActive: true, LastLoginAt: &oldTime,
		})
	}

	co := &ChatOrchestrator{
		userRepo: &mockUserRepo{users: users},
	}

	actx := &AdminToolContext{AdminID: 1}
	args := map[string]any{"days": float64(30), "limit": float64(10)}
	result := co.DispatchAdminTool(context.Background(), "admin_find_inactive_users", args, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := toGenericJSON(result.Data)
	items := data["items"].([]any)
	if len(items) > 10 {
		t.Errorf("expected at most 10 items, got %d", len(items))
	}
	if data["truncated"].(bool) != true {
		t.Error("expected truncated=true when items exceed limit")
	}
}

func TestT43_04_GovernanceSuggestions(t *testing.T) {
	users := []model.User{
		{ID: 1, Role: "admin", IsActive: true},
		{ID: 2, Role: "student", IsActive: false},
	}
	score := 75.0
	evals := []model.Evaluation{
		{ID: 1, Status: "scored", TotalScore: &score},
	}

	co := &ChatOrchestrator{
		userRepo:      &mockUserRepo{users: users},
		evalRepo:      &mockEvalRepo{evals: evals},
		uploadRepo:    &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{}},
		taskRepo:      &mockTaskRepoForTeacher{tasks: map[int64]*model.TrainingTask{}},
		llmConfigRepo: &mockLLMConfigRepo{config: nil}, // no active config
	}

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_generate_governance_suggestions", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if data["is_draft"].(bool) != true {
		t.Error("expected is_draft=true")
	}
	suggestions := data["suggestions"].(string)
	if !containsStr(suggestions, "建议") {
		t.Error("suggestions should contain '建议'")
	}
}

// ============================================================
// T4.4 — Audit Log Explanation Tests
// ============================================================

func TestT44_01_AuditSearchBasic(t *testing.T) {
	uid := int64(5)
	logs := []model.AuditLog{
		{ID: 1, OccurredAt: time.Now(), UserID: &uid, Username: "user1", Action: "login", Result: "success", ClientIP: "192.168.1.100"},
		{ID: 2, OccurredAt: time.Now(), UserID: &uid, Username: "user1", Action: "upload", Result: "success", ClientIP: "192.168.1.100"},
	}

	co := &ChatOrchestrator{
		auditRepo: &mockAuditRepo{logs: logs},
	}

	actx := &AdminToolContext{AdminID: 1}
	args := map[string]any{"limit": float64(20)}
	result := co.DispatchAdminTool(context.Background(), "admin_search_audit_logs", args, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := toGenericJSON(result.Data)
	items := data["items"].([]any)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	// Check IP masking
	item0 := items[0].(map[string]any)
	clientIP := item0["client_ip"].(string)
	if clientIP != "192.168.*.*" {
		t.Errorf("expected masked IP '192.168.*.*', got %q", clientIP)
	}
}

func TestT44_02_AuditSearchNoRepo(t *testing.T) {
	co := &ChatOrchestrator{} // no auditRepo

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_search_audit_logs", map[string]any{}, actx)
	if result.Success {
		t.Fatal("expected failure when audit repo is nil")
	}
	if !containsStr(result.Error, "not available") {
		t.Errorf("error should mention 'not available', got: %s", result.Error)
	}
}

func TestT44_03_AnomalyWording(t *testing.T) {
	uid := int64(5)
	var logs []model.AuditLog
	for i := int64(1); i <= 10; i++ {
		logs = append(logs, model.AuditLog{
			ID: i, OccurredAt: time.Now(), UserID: &uid,
			Username: "user1", Action: "failure", Result: "failure",
		})
	}

	co := &ChatOrchestrator{
		auditRepo: &mockAuditRepo{logs: logs},
	}

	actx := &AdminToolContext{AdminID: 1}
	args := map[string]any{"days": float64(7)}
	result := co.DispatchAdminTool(context.Background(), "admin_summarize_audit_anomalies", args, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if data["is_draft"].(bool) != true {
		t.Error("expected is_draft=true")
	}
	summary := data["summary"].(string)
	// Should mention "建议" not definitive "malicious"
	if !containsStr(summary, "建议") {
		t.Error("summary should contain '建议' (suggestion)")
	}
	if containsStr(summary, "恶意") {
		t.Error("summary should NOT use definitive '恶意' (malicious) language")
	}
}

func TestT44_04_AnomalyMaxDays(t *testing.T) {
	co := &ChatOrchestrator{
		auditRepo: &mockAuditRepo{logs: []model.AuditLog{}},
	}

	actx := &AdminToolContext{AdminID: 1}
	// Pass 365 days — should be capped at 90
	args := map[string]any{"days": float64(365)}
	result := co.DispatchAdminTool(context.Background(), "admin_summarize_audit_anomalies", args, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	// The result is valid; internally capped to 90 days
}

func TestT44_05_ExplainUserActivity(t *testing.T) {
	uid := int64(5)
	logs := []model.AuditLog{
		{ID: 1, OccurredAt: time.Now(), UserID: &uid, Username: "user1", Action: "login", Result: "success", TargetType: "session"},
		{ID: 2, OccurredAt: time.Now(), UserID: &uid, Username: "user1", Action: "upload", Result: "success", TargetType: "file"},
	}
	users := []model.User{
		{ID: 5, Username: "user1", Role: "student", IsActive: true, PasswordHash: "$2a$10$hash"},
	}

	co := &ChatOrchestrator{
		auditRepo: &mockAuditRepo{logs: logs},
		userRepo:  &mockUserRepo{users: users},
	}

	actx := &AdminToolContext{AdminID: 1}
	args := map[string]any{"user_id": float64(5)}
	result := co.DispatchAdminTool(context.Background(), "admin_explain_user_activity", args, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := toGenericJSON(result.Data)
	if data["username"].(string) != "user1" {
		t.Errorf("expected username='user1', got %v", data["username"])
	}
	items := data["items"].([]any)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestT44_06_ExplainUserActivityMissingParam(t *testing.T) {
	co := &ChatOrchestrator{
		auditRepo: &mockAuditRepo{},
	}

	actx := &AdminToolContext{AdminID: 1}
	// Missing required user_id
	result := co.DispatchAdminTool(context.Background(), "admin_explain_user_activity", map[string]any{}, actx)
	if result.Success {
		t.Fatal("expected failure for missing user_id")
	}
	if !containsStr(result.Error, "missing required parameter") {
		t.Errorf("error should mention 'missing required parameter', got: %s", result.Error)
	}
}

func TestT44_07_NonAdminToolDispatch(t *testing.T) {
	// Verify that admin tool dispatch works with empty orchestrator for basic tools
	co := &ChatOrchestrator{
		evalRepo:   &mockEvalRepo{},
		uploadRepo: &mockUploadRepoForTeacher{},
		taskRepo:   &mockTaskRepoForTeacher{},
	}

	actx := &AdminToolContext{AdminID: 1}
	result := co.DispatchAdminTool(context.Background(), "admin_get_system_overview", map[string]any{}, actx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
}
