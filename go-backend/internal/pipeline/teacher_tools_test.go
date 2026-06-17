// Package pipeline tests for teacher tools (T3.3).
// Uses inline mocks to avoid circular import with testutil.
package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// ============================================================
// Inline mock repos for teacher tool tests
// ============================================================

// mockTaskRepoForTeacher implements repository.TaskRepo for teacher tool tests.
type mockTaskRepoForTeacher struct {
	repository.TaskRepo
	tasks      map[int64]*model.TrainingTask
	dimensions map[int64][]model.Dimension
}

func (m *mockTaskRepoForTeacher) GetByID(_ context.Context, id int64) (*model.TrainingTask, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, errNotFound
	}
	return t, nil
}

func (m *mockTaskRepoForTeacher) GetDimensions(_ context.Context, taskID int64) ([]model.Dimension, error) {
	dims, ok := m.dimensions[taskID]
	if !ok {
		return nil, nil
	}
	return dims, nil
}

// mockUploadRepoForTeacher implements repository.UploadRepo for teacher tool tests.
type mockUploadRepoForTeacher struct {
	repository.UploadRepo
	uploads map[int64][]model.Upload // taskID → uploads
}

func (m *mockUploadRepoForTeacher) List(_ context.Context, params repository.UploadListParams) ([]model.Upload, int64, error) {
	if params.TaskID != nil {
		uploads := m.uploads[*params.TaskID]
		return uploads, int64(len(uploads)), nil
	}
	return nil, 0, nil
}

// mockClassRepoForTeacher implements repository.ClassRepo for teacher tool tests.
type mockClassRepoForTeacher struct {
	repository.ClassRepo
	classes map[int64]*model.Class
	members map[int64][]model.ClassMembership
}

func (m *mockClassRepoForTeacher) GetByID(_ context.Context, id int64) (*model.Class, error) {
	c, ok := m.classes[id]
	if !ok {
		return nil, errNotFound
	}
	return c, nil
}

func (m *mockClassRepoForTeacher) GetMembers(_ context.Context, classID int64) ([]model.ClassMembership, error) {
	members := m.members[classID]
	return members, nil
}

// errNotFound is a generic "not found" error for mocks.
var errNotFound = context.DeadlineExceeded // just need any error; we check ok/false not the type

// ============================================================
// T3.3 — Teacher Tools: Task/Class/Submission Statistics
// ============================================================

func makeScore(f float64) *float64 { return &f }

// TestT33_01_TeacherGetTaskSummary verifies the teacher_get_task_summary tool
// returns correct submission/evaluation counts and score statistics.
func TestT33_01_TeacherGetTaskSummary(t *testing.T) {
	taskID := int64(200)
	task := &model.TrainingTask{
		ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published",
	}

	uploads := []model.Upload{
		{ID: 1, TaskID: taskID, StudentID: 13},
		{ID: 2, TaskID: taskID, StudentID: 14},
		{ID: 3, TaskID: taskID, StudentID: 15},
	}

	evals := []model.Evaluation{
		{ID: 1, TaskID: taskID, Status: "scored", TotalScore: makeScore(85)},
		{ID: 2, TaskID: taskID, Status: "confirmed", TotalScore: makeScore(92)},
	}

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks: map[int64]*model.TrainingTask{taskID: task},
		},
		uploadRepo: &mockUploadRepoForTeacher{
			uploads: map[int64][]model.Upload{taskID: uploads},
		},
		evalRepo: &mockEvalRepo{
			evals: evals,
		},
	}

	ttctx := &TeacherToolContext{TeacherID: 11, TaskID: &taskID}
	args := map[string]any{"task_id": float64(taskID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_get_task_summary", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map data")
	}

	if data["submitted"].(int) != 3 {
		t.Errorf("submitted=%v, want 3", data["submitted"])
	}
	if data["scored"].(int) != 1 {
		t.Errorf("scored=%v, want 1", data["scored"])
	}
	if data["confirmed"].(int) != 1 {
		t.Errorf("confirmed=%v, want 1", data["confirmed"])
	}

	avg, ok := data["average_score"].(float64)
	if !ok || avg == 0 {
		t.Errorf("average_score=%v, want non-zero", data["average_score"])
	}
	// Average of 85 and 92 = 88.5
	if avg != 88.5 {
		t.Errorf("average_score=%v, want 88.5", avg)
	}
}

// TestT33_02_TeacherGetTaskSummaryForbidden verifies that teacher A cannot
// access teacher B's task via the teacher_get_task_summary tool.
func TestT33_02_TeacherGetTaskSummaryForbidden(t *testing.T) {
	taskID := int64(201) // belongs to teacher B (id=12)
	task := &model.TrainingTask{
		ID: taskID, Name: "Other Task", TeacherID: 12, Status: "published",
	}

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks: map[int64]*model.TrainingTask{taskID: task},
		},
		uploadRepo: &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{}},
		evalRepo:   &mockEvalRepo{},
	}

	// Teacher A (id=11) tries to access Teacher B's task
	ttctx := &TeacherToolContext{TeacherID: 11, TaskID: &taskID}
	args := map[string]any{"task_id": float64(taskID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_get_task_summary", args, ttctx)
	if result.Success {
		t.Fatal("expected failure for cross-teacher access")
	}
	if result.Error == "" {
		t.Error("expected error message for cross-teacher access")
	}
	if !containsStr(result.Error, "forbidden") {
		t.Errorf("error should contain 'forbidden', got: %s", result.Error)
	}
}

// TestT33_03_TeacherListPendingSubmissions verifies that only evaluations
// with status "scored" (pending teacher confirmation) are returned.
func TestT33_03_TeacherListPendingSubmissions(t *testing.T) {
	taskID := int64(200)
	task := &model.TrainingTask{
		ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published",
	}

	evals := []model.Evaluation{
		{ID: 1, TaskID: taskID, StudentID: 13, Status: "scored", TotalScore: makeScore(80)},
		{ID: 2, TaskID: taskID, StudentID: 14, Status: "scored", TotalScore: makeScore(75)},
		{ID: 3, TaskID: taskID, StudentID: 15, Status: "confirmed", TotalScore: makeScore(90)},
		{ID: 4, TaskID: taskID, StudentID: 16, Status: "rejected"},
	}

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks: map[int64]*model.TrainingTask{taskID: task},
		},
		uploadRepo: &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{}},
		evalRepo:   &mockEvalRepo{evals: evals},
	}

	ttctx := &TeacherToolContext{TeacherID: 11, TaskID: &taskID}
	args := map[string]any{"task_id": float64(taskID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_list_pending_submissions", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map data")
	}

	items, ok := data["items"].([]struct {
		EvalID    int64   `json:"evaluation_id"`
		StudentID int64   `json:"student_id"`
		Score     float64 `json:"total_score"`
		Status    string  `json:"status"`
	})

	// The items are returned as pendingItem structs, check total
	total := data["total"].(int)
	if total != 2 {
		t.Errorf("total=%d, want 2 (only 'scored' evaluations)", total)
	}
	_ = items
	_ = ok
}

// TestT33_04_TeacherGetClassPerformanceEmpty verifies the tool handles
// an empty class (no students or no evaluations) without panicking.
func TestT33_04_TeacherGetClassPerformanceEmpty(t *testing.T) {
	classID := int64(200)
	classObj := &model.Class{
		ID: classID, Name: "Empty Class", TeacherID: 11,
	}

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{tasks: map[int64]*model.TrainingTask{}},
		uploadRepo: &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{}},
		evalRepo:   &mockEvalRepo{},
		classRepo: &mockClassRepoForTeacher{
			classes: map[int64]*model.Class{classID: classObj},
			members: map[int64][]model.ClassMembership{},
		},
	}

	ttctx := &TeacherToolContext{TeacherID: 11, ClassID: &classID}
	args := map[string]any{"class_id": float64(classID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_get_class_performance", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map data")
	}

	if data["student_count"].(int) != 0 {
		t.Errorf("student_count=%v, want 0", data["student_count"])
	}
	if msg, ok := data["message"].(string); !ok || msg == "" {
		t.Error("expected 'no data' message for empty class")
	}
}

// TestT33_05_TeacherGetDimensionDistribution verifies dimension distribution
// returns correct per-dimension statistics.
func TestT33_05_TeacherGetDimensionDistribution(t *testing.T) {
	taskID := int64(200)
	task := &model.TrainingTask{
		ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published",
	}

	dims := []model.Dimension{
		{ID: 1, TaskID: taskID, Name: "Code Quality", Weight: 50},
		{ID: 2, TaskID: taskID, Name: "Documentation", Weight: 50},
	}

	evals := []model.Evaluation{
		{
			ID: 1, TaskID: taskID, Status: "scored", TotalScore: makeScore(85),
			Scores: []model.DimensionScore{
				{DimensionID: 1, AIScore: makeScore(80)},
				{DimensionID: 2, AIScore: makeScore(90)},
			},
		},
		{
			ID: 2, TaskID: taskID, Status: "confirmed", TotalScore: makeScore(78),
			Scores: []model.DimensionScore{
				{DimensionID: 1, AIScore: makeScore(70), TeacherScore: makeScore(75)},
				{DimensionID: 2, AIScore: makeScore(82)},
			},
		},
	}

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks:      map[int64]*model.TrainingTask{taskID: task},
			dimensions: map[int64][]model.Dimension{taskID: dims},
		},
		uploadRepo: &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{}},
		evalRepo:   &mockEvalRepo{evals: evals},
	}

	ttctx := &TeacherToolContext{TeacherID: 11, TaskID: &taskID}
	args := map[string]any{"task_id": float64(taskID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_get_dimension_distribution", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map data")
	}

	// dimDist is unexported inside the function; use JSON round-trip to inspect values.
	rawDims, ok := data["dimensions"]
	if !ok {
		t.Fatal("expected 'dimensions' key in result data")
	}
	distJSON, err := json.Marshal(rawDims)
	if err != nil {
		t.Fatalf("failed to marshal dimensions: %v", err)
	}
	var distList []map[string]any
	if err := json.Unmarshal(distJSON, &distList); err != nil {
		t.Fatalf("failed to unmarshal dimensions: %v", err)
	}
	if len(distList) != 2 {
		t.Fatalf("expected 2 dimensions, got %d", len(distList))
	}
	// Dimension 1: Code Quality — teacher score 75 takes precedence over AI 80 for eval 2
	// Eval 1: AI=80, Eval 2: teacher=75 → avg = (80+75)/2 = 77.5
	avg0 := distList[0]["average"].(float64)
	if avg0 != 77.5 {
		t.Errorf("dim Code Quality avg=%v, want 77.5", avg0)
	}
	// Dimension 2: Documentation — AI 90 and AI 82 → avg = (90+82)/2 = 86
	avg1 := distList[1]["average"].(float64)
	if avg1 != 86 {
		t.Errorf("dim Documentation avg=%v, want 86", avg1)
	}
}

// TestT33_06_TeacherToolSchemas verifies that TeacherToolSchemas returns
// exactly 4 tools with correct names.
func TestT33_06_TeacherToolSchemas(t *testing.T) {
	schemas := TeacherToolSchemas()
	if len(schemas) != 8 {
		t.Fatalf("expected 8 teacher tools, got %d", len(schemas))
	}

	expectedNames := map[string]bool{
		"teacher_get_task_summary":            false,
		"teacher_list_pending_submissions":    false,
		"teacher_get_class_performance":       false,
		"teacher_get_dimension_distribution":  false,
		"teacher_get_evaluation_detail":       false,
		"teacher_generate_feedback_draft":     false,
		"teacher_suggest_score_review":        false,
		"teacher_compare_with_rubric":         false,
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

// TestT33_07_TeacherToolUnknownName verifies that DispatchTeacherTool
// returns an error for unknown tool names.
func TestT33_07_TeacherToolUnknownName(t *testing.T) {
	co := &ChatOrchestrator{}
	ttctx := &TeacherToolContext{TeacherID: 11}
	result := co.DispatchTeacherTool(context.Background(), "teacher_unknown_tool", map[string]any{}, ttctx)
	if result.Success {
		t.Error("expected failure for unknown tool")
	}
	if !containsStr(result.Error, "unknown") {
		t.Errorf("error should mention 'unknown', got: %s", result.Error)
	}
}

// TestT33_08_TeacherToolMissingRequiredParam verifies that missing required
// parameters are caught before dispatch.
func TestT33_08_TeacherToolMissingRequiredParam(t *testing.T) {
	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{tasks: map[int64]*model.TrainingTask{}},
	}
	ttctx := &TeacherToolContext{TeacherID: 11}

	// Missing task_id
	result := co.DispatchTeacherTool(context.Background(), "teacher_get_task_summary", map[string]any{}, ttctx)
	if result.Success {
		t.Error("expected failure for missing task_id")
	}
	if !containsStr(result.Error, "task_id") {
		t.Errorf("error should mention 'task_id', got: %s", result.Error)
	}
}

// TestT33_09_TeacherGetClassPerformanceForbidden verifies cross-teacher
// class access is blocked.
func TestT33_09_TeacherGetClassPerformanceForbidden(t *testing.T) {
	classID := int64(201)
	classObj := &model.Class{
		ID: classID, Name: "Other Class", TeacherID: 12,
	}

	co := &ChatOrchestrator{
		taskRepo:   &mockTaskRepoForTeacher{tasks: map[int64]*model.TrainingTask{}},
		uploadRepo: &mockUploadRepoForTeacher{uploads: map[int64][]model.Upload{}},
		evalRepo:   &mockEvalRepo{},
		classRepo: &mockClassRepoForTeacher{
			classes: map[int64]*model.Class{classID: classObj},
		},
	}

	// Teacher A (11) tries Teacher B's class (201)
	ttctx := &TeacherToolContext{TeacherID: 11, ClassID: &classID}
	args := map[string]any{"class_id": float64(classID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_get_class_performance", args, ttctx)
	if result.Success {
		t.Fatal("expected failure for cross-teacher class access")
	}
	if !containsStr(result.Error, "forbidden") {
		t.Errorf("error should contain 'forbidden', got: %s", result.Error)
	}
}

// ============================================================
// Helper
// ============================================================

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// suppress unused import
var _ = time.Now

// ============================================================
// T3.4 — Teacher Tools: Assisted Grading & Feedback Drafts
// ============================================================

// mockUploadRepoWithParse extends mockUploadRepoForTeacher with GetByID and GetParseResult.
type mockUploadRepoWithParse struct {
	repository.UploadRepo
	uploads      map[int64]*model.Upload       // uploadID → upload
	parseResults map[int64]*model.ParseResult   // uploadID → parse result
	taskUploads  map[int64][]model.Upload       // taskID → uploads (for List)
}

func (m *mockUploadRepoWithParse) GetByID(_ context.Context, id int64) (*model.Upload, error) {
	u, ok := m.uploads[id]
	if !ok {
		return nil, errNotFound
	}
	return u, nil
}

func (m *mockUploadRepoWithParse) GetParseResult(_ context.Context, uploadID int64) (*model.ParseResult, error) {
	pr, ok := m.parseResults[uploadID]
	if !ok {
		return nil, errNotFound
	}
	return pr, nil
}

func (m *mockUploadRepoWithParse) List(_ context.Context, params repository.UploadListParams) ([]model.Upload, int64, error) {
	if params.TaskID != nil {
		uploads := m.taskUploads[*params.TaskID]
		return uploads, int64(len(uploads)), nil
	}
	return nil, 0, nil
}

// makeEvalHelper builds a standard teacher test evaluation with dimensions and scores.
func makeEvalHelper(evalID, taskID, studentID, uploadID int64, status string, totalScore float64, dims []model.DimensionScore) model.Evaluation {
	return model.Evaluation{
		ID: evalID, TaskID: taskID, StudentID: studentID, UploadID: uploadID,
		Status: status, TotalScore: makeScore(totalScore), Scores: dims,
	}
}

// TestT34_01_EvaluationDetail verifies teacher_get_evaluation_detail returns
// scores, dimensions, status, and a truncated parse summary.
func TestT34_01_EvaluationDetail(t *testing.T) {
	taskID := int64(200)
	evalID := int64(1)
	uploadID := int64(100)
	task := &model.TrainingTask{ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published"}
	dims := []model.Dimension{
		{ID: 1, TaskID: taskID, Name: "Code Quality", Weight: 50},
		{ID: 2, TaskID: taskID, Name: "Documentation", Weight: 50},
	}
	eval := makeEvalHelper(evalID, taskID, 13, uploadID, "scored", 85, []model.DimensionScore{
		{DimensionID: 1, AIScore: makeScore(80), Rationale: "Good code structure"},
		{DimensionID: 2, AIScore: makeScore(90), Rationale: "Well documented"},
	})

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks:      map[int64]*model.TrainingTask{taskID: task},
			dimensions: map[int64][]model.Dimension{taskID: dims},
		},
		uploadRepo: &mockUploadRepoWithParse{
			uploads: map[int64]*model.Upload{
				uploadID: {ID: uploadID, TaskID: taskID, StudentID: 13, ParseStatus: "parsed"},
			},
			parseResults: map[int64]*model.ParseResult{
				uploadID: {UploadID: uploadID, RawText: "Student experiment report content here."},
			},
		},
		evalRepo: &mockEvalRepo{evals: []model.Evaluation{eval}},
	}

	ttctx := &TeacherToolContext{TeacherID: 11}
	args := map[string]any{"evaluation_id": float64(evalID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_get_evaluation_detail", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map data")
	}

	if data["status"].(string) != "scored" {
		t.Errorf("status=%v, want 'scored'", data["status"])
	}
	if ts, ok := data["total_score"].(*float64); ok {
		if *ts != 85 {
			t.Errorf("total_score=%v, want 85", *ts)
		}
	} else if ts, ok := data["total_score"].(float64); ok {
		if ts != 85 {
			t.Errorf("total_score=%v, want 85", ts)
		}
	} else {
		t.Errorf("total_score type unexpected: %T", data["total_score"])
	}
	if _, ok := data["scores"]; !ok {
		t.Error("expected 'scores' key in result")
	}
	if data["upload_summary"].(string) == "" {
		t.Error("expected non-empty upload_summary")
	}
	// No confirmed warning for scored status
	if _, ok := data["confirmed_warning"]; ok {
		t.Error("unexpected confirmed_warning for scored eval")
	}
}

// TestT34_02_EvaluationDetailForbidden verifies cross-teacher access is blocked.
func TestT34_02_EvaluationDetailForbidden(t *testing.T) {
	taskID := int64(201) // belongs to teacher B (12)
	evalID := int64(2)
	task := &model.TrainingTask{ID: taskID, Name: "Other Task", TeacherID: 12, Status: "published"}
	eval := makeEvalHelper(evalID, taskID, 14, 200, "scored", 78, nil)

	co := &ChatOrchestrator{
		taskRepo:   &mockTaskRepoForTeacher{tasks: map[int64]*model.TrainingTask{taskID: task}},
		uploadRepo: &mockUploadRepoWithParse{},
		evalRepo:   &mockEvalRepo{evals: []model.Evaluation{eval}},
	}

	// Teacher A (11) tries to access Teacher B's evaluation
	ttctx := &TeacherToolContext{TeacherID: 11}
	args := map[string]any{"evaluation_id": float64(evalID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_get_evaluation_detail", args, ttctx)
	if result.Success {
		t.Fatal("expected failure for cross-teacher access")
	}
	if !containsStr(result.Error, "forbidden") {
		t.Errorf("error should contain 'forbidden', got: %s", result.Error)
	}
}

// TestT34_03_FeedbackDraftNoDBWrite verifies teacher_generate_feedback_draft
// returns a draft without modifying the evaluation.
func TestT34_03_FeedbackDraftNoDBWrite(t *testing.T) {
	taskID := int64(200)
	evalID := int64(1)
	uploadID := int64(100)
	task := &model.TrainingTask{
		ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published",
		Requirements: "Write a Go program with tests",
	}
	dims := []model.Dimension{
		{ID: 1, TaskID: taskID, Name: "Code Quality", Weight: 60},
		{ID: 2, TaskID: taskID, Name: "Testing", Weight: 40},
	}
	eval := makeEvalHelper(evalID, taskID, 13, uploadID, "scored", 82, []model.DimensionScore{
		{DimensionID: 1, AIScore: makeScore(85), Rationale: "Clean code"},
		{DimensionID: 2, AIScore: makeScore(78), Rationale: "Some tests missing"},
	})

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks:      map[int64]*model.TrainingTask{taskID: task},
			dimensions: map[int64][]model.Dimension{taskID: dims},
		},
		uploadRepo: &mockUploadRepoWithParse{
			uploads: map[int64]*model.Upload{
				uploadID: {ID: uploadID, TaskID: taskID, StudentID: 13, ParseStatus: "parsed"},
			},
			parseResults: map[int64]*model.ParseResult{
				uploadID: {UploadID: uploadID, RawText: "Go program with unit tests for sorting algorithms."},
			},
		},
		evalRepo: &mockEvalRepo{evals: []model.Evaluation{eval}},
	}

	ttctx := &TeacherToolContext{TeacherID: 11}
	args := map[string]any{"evaluation_id": float64(evalID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_generate_feedback_draft", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map data")
	}

	draft, ok := data["draft"].(string)
	if !ok || draft == "" {
		t.Fatal("expected non-empty draft string")
	}
	if !containsStr(draft, "草稿") {
		t.Error("draft should contain '草稿' marker")
	}
	if !containsStr(draft, "教师确认") && !containsStr(draft, "教师审阅") {
		t.Error("draft should mention teacher confirmation")
	}
	if data["is_draft"] != true {
		t.Error("is_draft should be true")
	}

	// Verify eval was NOT modified — the original eval in mockEvalRepo should be unchanged
	storedEval := co.evalRepo.(*mockEvalRepo).evals[0]
	if storedEval.TeacherComment != "" {
		t.Error("eval TeacherComment should remain empty (no DB write)")
	}
}

// TestT34_04_ConfirmedEvaluation verifies that accessing a confirmed evaluation
// includes a warning about the confirmed status.
func TestT34_04_ConfirmedEvaluation(t *testing.T) {
	taskID := int64(200)
	evalID := int64(1)
	task := &model.TrainingTask{ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published"}
	dims := []model.Dimension{
		{ID: 1, TaskID: taskID, Name: "Code Quality", Weight: 100},
	}
	eval := makeEvalHelper(evalID, taskID, 13, 100, "confirmed", 90, []model.DimensionScore{
		{DimensionID: 1, AIScore: makeScore(90), Rationale: "Excellent"},
	})

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks:      map[int64]*model.TrainingTask{taskID: task},
			dimensions: map[int64][]model.Dimension{taskID: dims},
		},
		uploadRepo: &mockUploadRepoWithParse{
			uploads:      map[int64]*model.Upload{},
			parseResults: map[int64]*model.ParseResult{},
		},
		evalRepo: &mockEvalRepo{evals: []model.Evaluation{eval}},
	}

	ttctx := &TeacherToolContext{TeacherID: 11}
	args := map[string]any{"evaluation_id": float64(evalID)}

	// Test evaluation detail
	result := co.DispatchTeacherTool(context.Background(), "teacher_get_evaluation_detail", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	data := result.Data.(map[string]any)
	warning, ok := data["confirmed_warning"].(string)
	if !ok || warning == "" {
		t.Error("expected confirmed_warning for confirmed eval")
	}
	if !containsStr(warning, "已确认") {
		t.Errorf("confirmed_warning should mention '已确认', got: %s", warning)
	}

	// Test feedback draft also warns
	result2 := co.DispatchTeacherTool(context.Background(), "teacher_generate_feedback_draft", args, ttctx)
	if !result2.Success {
		t.Fatalf("feedback draft expected success, got error: %s", result2.Error)
	}
	data2 := result2.Data.(map[string]any)
	draft := data2["draft"].(string)
	if !containsStr(draft, "已确认") {
		t.Error("feedback draft should warn about confirmed status")
	}
}

// TestT34_05_ParseResultMissing verifies tools handle missing parse results gracefully.
func TestT34_05_ParseResultMissing(t *testing.T) {
	taskID := int64(200)
	evalID := int64(1)
	task := &model.TrainingTask{ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published"}
	dims := []model.Dimension{
		{ID: 1, TaskID: taskID, Name: "Code Quality", Weight: 100},
	}
	eval := makeEvalHelper(evalID, taskID, 13, 100, "scored", 75, []model.DimensionScore{
		{DimensionID: 1, AIScore: makeScore(75), Rationale: "Acceptable"},
	})

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks:      map[int64]*model.TrainingTask{taskID: task},
			dimensions: map[int64][]model.Dimension{taskID: dims},
		},
		// No upload or parse result available
		uploadRepo: &mockUploadRepoWithParse{
			uploads:      map[int64]*model.Upload{},
			parseResults: map[int64]*model.ParseResult{},
		},
		evalRepo: &mockEvalRepo{evals: []model.Evaluation{eval}},
	}

	ttctx := &TeacherToolContext{TeacherID: 11}
	args := map[string]any{"evaluation_id": float64(evalID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_get_evaluation_detail", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	note, ok := data["parse_result_note"].(string)
	if !ok || note == "" {
		t.Error("expected parse_result_note when parse result is missing")
	}
	if !containsStr(note, "不可用") {
		t.Errorf("parse_result_note should mention '不可用', got: %s", note)
	}

	// Feedback draft should also handle missing parse gracefully
	result2 := co.DispatchTeacherTool(context.Background(), "teacher_generate_feedback_draft", args, ttctx)
	if !result2.Success {
		t.Fatalf("expected success for feedback draft, got: %s", result2.Error)
	}
	draft := result2.Data.(map[string]any)["draft"].(string)
	if !containsStr(draft, "不可用") && !containsStr(draft, "解析结果") {
		t.Error("draft should note missing parse content")
	}
}

// TestT34_06_SuggestScoreReview verifies teacher_suggest_score_review identifies
// dimensions with potential scoring issues.
func TestT34_06_SuggestScoreReview(t *testing.T) {
	taskID := int64(200)
	evalID := int64(1)
	task := &model.TrainingTask{ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published"}
	dims := []model.Dimension{
		{ID: 1, TaskID: taskID, Name: "Code Quality", Weight: 50},
		{ID: 2, TaskID: taskID, Name: "Documentation", Weight: 50},
	}
	// AI gives very low score on dim 1 (25) and normal on dim 2 (85)
	eval := makeEvalHelper(evalID, taskID, 13, 100, "scored", 55, []model.DimensionScore{
		{DimensionID: 1, AIScore: makeScore(25), Rationale: "poor"},
		{DimensionID: 2, AIScore: makeScore(85), Rationale: "Good documentation with examples"},
	})

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks:      map[int64]*model.TrainingTask{taskID: task},
			dimensions: map[int64][]model.Dimension{taskID: dims},
		},
		uploadRepo: &mockUploadRepoWithParse{
			uploads:      map[int64]*model.Upload{},
			parseResults: map[int64]*model.ParseResult{},
		},
		evalRepo: &mockEvalRepo{evals: []model.Evaluation{eval}},
	}

	ttctx := &TeacherToolContext{TeacherID: 11}
	args := map[string]any{"evaluation_id": float64(evalID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_suggest_score_review", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	reviewCount := data["review_suggested"].(int)
	if reviewCount == 0 {
		t.Error("expected at least 1 review suggestion for the very low score (25)")
	}
	if !containsStr(data["note"].(string), "不自动改分") {
		t.Error("note should clarify that scores are not auto-modified")
	}
}

// TestT34_07_CompareWithRubric verifies teacher_compare_with_rubric returns
// criteria comparison with fulfillment levels.
func TestT34_07_CompareWithRubric(t *testing.T) {
	taskID := int64(200)
	evalID := int64(1)
	task := &model.TrainingTask{
		ID: taskID, Name: "Test Task", TeacherID: 11, Status: "published",
		EvaluationCriteria: "Code must compile, have tests, and documentation",
	}
	dims := []model.Dimension{
		{ID: 1, TaskID: taskID, Name: "Code Quality", Weight: 50, Description: "Code compiles and runs"},
		{ID: 2, TaskID: taskID, Name: "Documentation", Weight: 50, Description: "Has README and comments"},
	}
	eval := makeEvalHelper(evalID, taskID, 13, 100, "scored", 80, []model.DimensionScore{
		{DimensionID: 1, AIScore: makeScore(90), Rationale: "Clean code"},
		{DimensionID: 2, AIScore: makeScore(70), Rationale: "Partial docs"},
	})

	co := &ChatOrchestrator{
		taskRepo: &mockTaskRepoForTeacher{
			tasks:      map[int64]*model.TrainingTask{taskID: task},
			dimensions: map[int64][]model.Dimension{taskID: dims},
		},
		uploadRepo: &mockUploadRepoWithParse{
			uploads:      map[int64]*model.Upload{},
			parseResults: map[int64]*model.ParseResult{},
		},
		evalRepo: &mockEvalRepo{evals: []model.Evaluation{eval}},
	}

	ttctx := &TeacherToolContext{TeacherID: 11}
	args := map[string]any{"evaluation_id": float64(evalID)}

	result := co.DispatchTeacherTool(context.Background(), "teacher_compare_with_rubric", args, ttctx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data := result.Data.(map[string]any)
	if _, ok := data["criteria_comparison"]; !ok {
		t.Error("expected 'criteria_comparison' key")
	}
	if !containsStr(data["note"].(string), "不自动确认") {
		t.Error("note should clarify no auto-confirm")
	}
}
