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
