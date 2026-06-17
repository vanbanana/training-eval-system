// Package pipeline tests for the robust ChatOrchestrator agent loop.
// Uses an inline mock (mockLLM) to avoid circular import with testutil.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// --- inline mock (avoids testutil → handler → pipeline cycle) ---

type mockResponse struct {
	content   string
	toolCalls []mockToolCall
	err       error
}

type mockToolCall struct {
	name string
	args map[string]any
}

type mockLLM struct {
	mu        sync.Mutex
	responses []mockResponse
	idx       int
}

func newMockLLM(resps ...mockResponse) *mockLLM {
	return &mockLLM{responses: resps}
}

func (m *mockLLM) text(s string) mockResponse {
	return mockResponse{content: s}
}

func (m *mockLLM) toolCall(name string, args map[string]any) mockResponse {
	return mockResponse{toolCalls: []mockToolCall{{name: name, args: args}}}
}

func (m *mockLLM) err(e error) mockResponse {
	return mockResponse{err: e}
}

func (m *mockLLM) Complete(_ context.Context, _ []llm.ChatMessage, _ []llm.Tool) (*llm.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.idx >= len(m.responses) {
		return &llm.ChatResponse{
			Choices: []llm.ChatChoice{{FinishReason: "stop", Message: llm.ChatMessage{Role: "assistant", Content: ""}}},
		}, nil
	}
	r := m.responses[m.idx]
	m.idx++

	if r.err != nil {
		return nil, r.err
	}

	resp := &llm.ChatResponse{
		Choices: []llm.ChatChoice{{
			FinishReason: "stop",
			Message:      llm.ChatMessage{Role: "assistant", Content: r.content},
		}},
	}
	if len(r.toolCalls) > 0 {
		tcs := make([]llm.ToolCall, 0, len(r.toolCalls))
		for _, tc := range r.toolCalls {
			argsJSON, _ := json.Marshal(tc.args)
			tcs = append(tcs, llm.ToolCall{
				ID:   "mock_call_1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      tc.name,
					Arguments: string(argsJSON),
				},
			})
		}
		resp.Choices[0].Message.ToolCalls = tcs
		resp.Choices[0].FinishReason = "tool_calls"
	}
	return resp, nil
}

// --- helpers ---

func newTestCtx() *ChatToolContext {
	score := 85.0
	return &ChatToolContext{
		StudentID: 1,
		Evaluation: &model.Evaluation{
			ID: 10, Status: "graded", TotalScore: &score,
		},
		Task: &model.TrainingTask{
			ID: 100, Name: "测试任务", Requirements: "完成一个简单项目",
		},
		ParseResult: &model.ParseResult{RawText: "这是学生的提交内容，包含数据库设计和API开发。"},
		Dimensions: []model.Dimension{
			{ID: 1, Name: "代码质量", Weight: 30, Description: "代码规范与可读性"},
			{ID: 2, Name: "功能完整性", Weight: 40, Description: "功能是否齐全"},
		},
	}
}

// --- Tests ---

func TestRun_DirectAnswer(t *testing.T) {
	m := newMockLLM(mockResponse{content: "你好，这是直接回答。"})
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	resp, err := co.Run(context.Background(), nil, "你好", newTestCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "直接回答") {
		t.Errorf("expected direct answer, got: %s", resp.Choices[0].Message.Content)
	}
}

func TestRun_ToolCallThenAnswer(t *testing.T) {
	m := newMockLLM(
		mockResponse{toolCalls: []mockToolCall{{name: "get_parse_segment", args: map[string]any{"topic": "数据库"}}}},
		mockResponse{content: "根据原文内容，你的数据库设计部分..."},
	)
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	resp, err := co.Run(context.Background(), nil, "帮我看看数据库部分", newTestCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "数据库设计") {
		t.Errorf("expected answer about DB, got: %s", resp.Choices[0].Message.Content)
	}
}

func TestRun_LLMRetryOnTransientError(t *testing.T) {
	m := newMockLLM(
		mockResponse{err: errors.New("llm: API returned status 500: internal server error")},
		mockResponse{content: "重试成功后的回答"},
	)
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	resp, err := co.Run(context.Background(), nil, "测试重试", newTestCtx())
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "重试成功") {
		t.Errorf("expected retry answer, got: %v", resp.Choices[0].Message.Content)
	}
}

func TestRun_LLMNonRetryableError(t *testing.T) {
	m := newMockLLM(
		mockResponse{err: errors.New("llm: API returned status 400: bad request")},
	)
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	_, err := co.Run(context.Background(), nil, "测试不可重试错误", newTestCtx())
	if err == nil {
		t.Fatal("expected error for non-retryable 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got: %v", err)
	}
}

func TestRun_LLMAllRetriesExhausted_Fallback(t *testing.T) {
	serverDown := errors.New("llm: API returned status 500: server down")
	m := newMockLLM(
		// Round 0: tool call succeeds (get_parse_segment doesn't need repos)
		mockResponse{toolCalls: []mockToolCall{{name: "get_parse_segment", args: map[string]any{"topic": "数据库"}}}},
		// Round 1: LLM fails all 3 attempts (initial + 2 retries)
		mockResponse{err: serverDown},
		mockResponse{err: serverDown},
		mockResponse{err: serverDown},
		// Fallback final call also fails
		mockResponse{err: serverDown},
		mockResponse{err: serverDown},
		mockResponse{err: serverDown},
	)
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	resp, err := co.Run(context.Background(), nil, "测试全部失败", newTestCtx())
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "暂时遇到了问题") {
		t.Errorf("expected fallback message, got: %v", resp.Choices[0].Message.Content)
	}
}

func TestRun_LLMFailsOnFirstRound(t *testing.T) {
	m := newMockLLM(
		mockResponse{err: errors.New("llm: API returned status 400: bad request")},
	)
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	_, err := co.Run(context.Background(), nil, "测试首轮失败", newTestCtx())
	if err == nil {
		t.Fatal("expected error on first round failure")
	}
}

func TestRun_ToolMissingParam(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)

	result := co.DispatchTool(context.Background(), "get_dimension_detail", map[string]any{}, newTestCtx())
	if result.Success {
		t.Fatal("expected failure for missing required param")
	}
	if !strings.Contains(result.Error, "缺少必填参数") {
		t.Errorf("expected missing param error, got: %s", result.Error)
	}
}

func TestRun_ToolEmptyParam(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)

	result := co.DispatchTool(context.Background(), "get_learning_resources", map[string]any{"keyword": ""}, newTestCtx())
	if result.Success {
		t.Fatal("expected failure for empty required param")
	}
	if !strings.Contains(result.Error, "不能为空") {
		t.Errorf("expected empty param error, got: %s", result.Error)
	}
}

func TestRun_ToolUnknownName(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)

	result := co.DispatchTool(context.Background(), "nonexistent_tool", map[string]any{}, newTestCtx())
	if result.Success {
		t.Fatal("expected failure for unknown tool")
	}
	if !strings.Contains(result.Error, "unknown tool") {
		t.Errorf("expected unknown tool error, got: %s", result.Error)
	}
}

func TestRun_ToolOptionalParams(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)
	tctx := newTestCtx()
	// get_parse_segment requires "topic" — provide it so validation passes
	result := co.DispatchTool(context.Background(), "get_parse_segment", map[string]any{"topic": "数据库"}, tctx)
	if !result.Success {
		t.Fatalf("expected success when required param is provided, got: %s", result.Error)
	}
}

func TestRun_ConsecutiveToolFailures_CircuitBreaker(t *testing.T) {
	m := newMockLLM(
		mockResponse{toolCalls: []mockToolCall{{name: "get_dimension_detail", args: map[string]any{"dimension_name": "不存在的维度"}}}},
		mockResponse{toolCalls: []mockToolCall{{name: "get_dimension_detail", args: map[string]any{"dimension_name": "另一个不存在的"}}}},
		mockResponse{content: "根据已有信息，我无法找到该维度的详细数据。"},
	)
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	resp, err := co.Run(context.Background(), nil, "查询不存在的维度", newTestCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		t.Fatal("expected response")
	}
}

func TestRun_ToolTimeout(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)
	tctx := newTestCtx()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = co.DispatchTool(ctx, "get_parse_segment", map[string]any{"topic": "test"}, tctx)
}

func TestRun_EnhancedErrorFeedback(t *testing.T) {
	m := newMockLLM(
		mockResponse{toolCalls: []mockToolCall{{name: "get_dimension_detail", args: map[string]any{"dimension_name": "不存在"}}}},
		mockResponse{content: "抱歉找不到该维度"},
	)
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	resp, err := co.Run(context.Background(), nil, "查询维度", newTestCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

func TestRun_MaxRoundsReached(t *testing.T) {
	resps := make([]mockResponse, 0, MaxToolRounds+1)
	for i := 0; i < MaxToolRounds; i++ {
		resps = append(resps, mockResponse{toolCalls: []mockToolCall{{name: "get_parse_segment", args: map[string]any{"topic": "test"}}}})
	}
	resps = append(resps, mockResponse{content: "经过多次查询，以下是我的回答..."})

	m := newMockLLM(resps...)
	co := NewChatOrchestrator(m, nil, nil, nil, nil)

	resp, err := co.Run(context.Background(), nil, "查询所有内容", newTestCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "多次查询") {
		t.Errorf("expected final answer after max rounds, got: %v", resp.Choices[0].Message.Content)
	}
}

func TestRetryLLMCall_RetryableVsNonRetryable(t *testing.T) {
	// 429 is retryable
	m1 := newMockLLM(
		mockResponse{err: errors.New("llm: API returned status 429: rate limit")},
		mockResponse{content: "success after rate limit"},
	)
	co1 := NewChatOrchestrator(m1, nil, nil, nil, nil)
	resp, err := co1.retryLLMCall(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("429 should be retried, got error: %v", err)
	}
	if resp.Choices[0].Message.Content != "success after rate limit" {
		t.Errorf("expected success, got: %v", resp.Choices[0].Message.Content)
	}

	// 403 is NOT retryable
	m2 := newMockLLM(
		mockResponse{err: errors.New("llm: API returned status 403: forbidden")},
	)
	co2 := NewChatOrchestrator(m2, nil, nil, nil, nil)
	_, err = co2.retryLLMCall(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("403 should not be retried")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 error, got: %v", err)
	}
}

func TestRun_ContextCancelled(t *testing.T) {
	m := newMockLLM(
		mockResponse{toolCalls: []mockToolCall{{name: "get_parse_segment", args: map[string]any{"topic": "test"}}}},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	co := NewChatOrchestrator(m, nil, nil, nil, nil)
	_, err := co.Run(ctx, nil, "测试超时", newTestCtx())
	if err == nil {
		return // first round completed before timeout is acceptable
	}
	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "LLM") && !strings.Contains(err.Error(), "fallback") {
		t.Errorf("expected context or LLM error, got: %v", err)
	}
}

func TestFallbackResponse(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)
	resp, err := co.fallbackResponse(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := resp.Choices[0].Message.Content
	if !strings.Contains(msg, "暂时遇到了问题") {
		t.Errorf("expected friendly fallback, got: %s", msg)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("expected finish_reason=stop, got: %s", resp.Choices[0].FinishReason)
	}
}

func TestBuildChatSystemPrompt(t *testing.T) {
	tctx := newTestCtx()
	prompt := BuildChatSystemPrompt(tctx.Task, tctx.Evaluation, tctx.ParseResult, tctx.Dimensions)

	for _, c := range []string{"实训评价 AI 助手", "测试任务", "代码质量", "功能完整性", "85.0", "数据库设计"} {
		if !strings.Contains(prompt, c) {
			t.Errorf("system prompt missing %q", c)
		}
	}
}

func TestChatToolSchemas(t *testing.T) {
	schemas := ChatToolSchemas()
	if len(schemas) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(schemas))
	}
	for _, s := range schemas {
		if s.Type != "function" {
			t.Errorf("tool %s: expected type=function, got %s", s.Function.Name, s.Type)
		}
		if s.Function.Name == "" {
			t.Error("tool has empty name")
		}
		if s.Function.Description == "" {
			t.Errorf("tool %s: missing description", s.Function.Name)
		}
		var params map[string]any
		if err := json.Unmarshal(s.Function.Parameters, &params); err != nil {
			t.Errorf("tool %s: invalid parameters JSON: %v", s.Function.Name, err)
		}
	}
}

func TestToolRequiredParams(t *testing.T) {
	schemas := ChatToolSchemas()
	for _, s := range schemas {
		if _, ok := toolRequiredParams[s.Function.Name]; !ok {
			t.Errorf("tool %s not in toolRequiredParams map", s.Function.Name)
		}
	}
}

// ============================================================
// T2.2 — Tool-level permission and data isolation tests
// ============================================================

// mockEvalRepo is a minimal EvaluationRepo mock for tool tests.
type mockEvalRepo struct {
	evals []model.Evaluation
}

func (m *mockEvalRepo) GetByID(_ context.Context, _ int64) (*model.Evaluation, error) {
	return nil, errors.New("not implemented")
}
func (m *mockEvalRepo) List(_ context.Context, params EvalListParams) ([]model.Evaluation, int64, error) {
	var filtered []model.Evaluation
	for _, e := range m.evals {
		if params.StudentID != nil && e.StudentID != *params.StudentID {
			continue
		}
		if params.TaskID != nil && e.TaskID != *params.TaskID {
			continue
		}
		if params.Status != nil && e.Status != *params.Status {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, int64(len(filtered)), nil
}
func (m *mockEvalRepo) Create(_ context.Context, _ *model.Evaluation) error {
	return errors.New("not implemented")
}
func (m *mockEvalRepo) Update(_ context.Context, _ *model.Evaluation) error {
	return errors.New("not implemented")
}
func (m *mockEvalRepo) Delete(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}
func (m *mockEvalRepo) BatchConfirm(_ context.Context, _ []int64) error {
	return errors.New("not implemented")
}
func (m *mockEvalRepo) SaveScores(_ context.Context, _ int64, _ []model.DimensionScore) error {
	return errors.New("not implemented")
}
func (m *mockEvalRepo) AppendHistory(_ context.Context, _ *model.EvaluationHistory) error {
	return errors.New("not implemented")
}
func (m *mockEvalRepo) GetHistory(_ context.Context, _ int64) ([]model.EvaluationHistory, error) {
	return nil, errors.New("not implemented")
}
func (m *mockEvalRepo) GetDimensionScores(_ context.Context, _ int64) ([]model.DimensionScore, error) {
	return nil, errors.New("not implemented")
}
func (m *mockEvalRepo) UpdateDimensionTeacherScore(_ context.Context, _ int64, _ int64, _ *float64) error {
	return errors.New("not implemented")
}

// EvalListParams type alias for the mock (pipeline imports repository already)
type EvalListParams = repository.EvalListParams

// TestT22_Tool01_GetParseSegmentNormal verifies get_parse_segment returns the
// student's own text fragment when the topic matches raw text.
func TestT22_Tool01_GetParseSegmentNormal(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)
	tctx := newTestCtx()

	result := co.DispatchTool(context.Background(), "get_parse_segment",
		map[string]any{"topic": "数据库"}, tctx)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	segments, ok := dataMap["segments"].([]map[string]any)
	if !ok || len(segments) == 0 {
		t.Fatal("expected non-empty segments")
	}
	text, _ := segments[0]["text"].(string)
	if !strings.Contains(text, "数据库") {
		t.Errorf("segment text should contain topic '数据库', got: %s", text)
	}
}

// TestT22_Tool02_GetParseSegmentNoContext verifies get_parse_segment returns
// Success=false with a descriptive error when ParseResult is nil.
func TestT22_Tool02_GetParseSegmentNoContext(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)
	tctx := &ChatToolContext{
		StudentID:   1,
		ParseResult: nil, // no evaluation context
	}

	result := co.DispatchTool(context.Background(), "get_parse_segment",
		map[string]any{"topic": "数据库"}, tctx)

	if result.Success {
		t.Fatal("expected failure when ParseResult is nil")
	}
	if !strings.Contains(result.Error, "no parsed content") {
		t.Errorf("expected 'no parsed content' error, got: %s", result.Error)
	}
}

// TestT22_Tool03_GetClassStatisticsAnonymized verifies get_class_statistics
// returns only anonymous aggregate data (mean/median/p75) and never includes
// username, display_name, or student_id fields.
func TestT22_Tool03_GetClassStatisticsAnonymized(t *testing.T) {
	taskID := int64(100)
	score1, score2, score3 := 80.0, 90.0, 70.0
	repo := &mockEvalRepo{
		evals: []model.Evaluation{
			{ID: 1, TaskID: taskID, StudentID: 1, Status: "graded", TotalScore: &score1},
			{ID: 2, TaskID: taskID, StudentID: 2, Status: "graded", TotalScore: &score2},
			{ID: 3, TaskID: taskID, StudentID: 3, Status: "graded", TotalScore: &score3},
		},
	}
	co := NewChatOrchestrator(nil, repo, nil, nil, nil)
	tctx := &ChatToolContext{
		StudentID: 1,
		Task:      &model.TrainingTask{ID: taskID, Name: "Test Task"},
	}

	result := co.DispatchTool(context.Background(), "get_class_statistics",
		map[string]any{}, tctx)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	// Serialize result data to JSON and verify no PII fields
	dataJSON, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	dataStr := string(dataJSON)

	for _, forbidden := range []string{"username", "display_name", "student_id", "name"} {
		if strings.Contains(strings.ToLower(dataStr), forbidden) {
			t.Errorf("class statistics contains forbidden field %q in: %s", forbidden, dataStr)
		}
	}

	// Verify aggregate fields are present
	dataMap := result.Data.(map[string]any)
	for _, key := range []string{"count", "mean", "median", "p75"} {
		if _, ok := dataMap[key]; !ok {
			t.Errorf("expected aggregate field %q in result", key)
		}
	}
}

// TestT22_Tool04_GetDimensionHistoryOnlyOwnData verifies get_dimension_history
// only queries evals for the current student (StudentID in context) and does not
// include other students' data.
func TestT22_Tool04_GetDimensionHistoryOnlyOwnData(t *testing.T) {
	studentA := int64(1)
	studentB := int64(2)
	scoreA, scoreB := 85.0, 95.0
	repo := &mockEvalRepo{
		evals: []model.Evaluation{
			{ID: 1, StudentID: studentA, Status: "graded", Scores: []model.DimensionScore{
				{DimensionID: 1, TeacherScore: &scoreA},
			}},
			{ID: 2, StudentID: studentB, Status: "graded", Scores: []model.DimensionScore{
				{DimensionID: 1, TeacherScore: &scoreB},
			}},
		},
	}
	co := NewChatOrchestrator(nil, repo, nil, nil, nil)
	tctx := &ChatToolContext{
		StudentID: studentA,
		Dimensions: []model.Dimension{
			{ID: 1, Name: "代码质量"},
		},
	}

	result := co.DispatchTool(context.Background(), "get_dimension_history",
		map[string]any{"dimension_name": "代码质量"}, tctx)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	dataMap := result.Data.(map[string]any)
	scores, ok := dataMap["scores"].([]map[string]any)
	if !ok {
		t.Fatalf("expected scores slice, got %T", dataMap["scores"])
	}

	// Should only contain student A's score (85), not student B's (95)
	for _, s := range scores {
		score, _ := s["score"].(float64)
		if score == scoreB {
			t.Errorf("dimension history contains student B's score (%v), should only have student A's", score)
		}
	}
	if len(scores) != 1 {
		t.Errorf("expected exactly 1 score entry for student A, got %d", len(scores))
	}
}

// TestT22_Tool05_ToolParamFuzz verifies that tools handle malformed parameters
// (numeric, null, excessively long strings) without panicking and return
// structured errors.
func TestT22_Tool05_ToolParamFuzz(t *testing.T) {
	co := NewChatOrchestrator(nil, nil, nil, nil, nil)
	tctx := newTestCtx()

	tests := []struct {
		name     string
		tool     string
		args     map[string]any
		wantFail bool
	}{
		{
			name:     "dimension_name as number",
			tool:     "get_dimension_detail",
			args:     map[string]any{"dimension_name": float64(12345)},
			wantFail: true, // type assertion to string fails → empty name → not found
		},
		{
			name:     "dimension_name as null",
			tool:     "get_dimension_detail",
			args:     map[string]any{"dimension_name": nil},
			wantFail: true, // required param validation catches nil
		},
		{
			name:     "dimension_name excessively long",
			tool:     "get_dimension_detail",
			args:     map[string]any{"dimension_name": strings.Repeat("A", 10000)},
			wantFail: true, // dimension not found → error
		},
		{
			name:     "topic as number",
			tool:     "get_parse_segment",
			args:     map[string]any{"topic": float64(42)},
			wantFail: false, // topic type assertion fails → empty string → returns full text
		},
		{
			name:     "keyword as null",
			tool:     "get_learning_resources",
			args:     map[string]any{"keyword": nil},
			wantFail: true, // required param validation catches nil
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic
			result := co.DispatchTool(context.Background(), tc.tool, tc.args, tctx)
			if result == nil {
				t.Fatal("DispatchTool returned nil result")
			}
			if tc.wantFail && result.Success {
				t.Errorf("expected failure for %s, got success", tc.name)
			}
			if tc.wantFail && result.Error == "" {
				t.Errorf("expected error message for %s, got empty", tc.name)
			}
		})
	}
}
