// Package pipeline — Exported mock helpers for cross-package testing (T8.2).
package pipeline

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/smartedu/training-eval-system/internal/llm"
)

// MockLLMResponse defines a pre-programmed response for the mock LLM client.
type MockLLMResponse struct {
	Content   string
	ToolCalls []MockToolCallDef
	Err       error
}

// MockToolCallDef defines a mock tool call within a response.
type MockToolCallDef struct {
	Name string
	Args map[string]any
}

// MockLLMClient is an exported mock LLM client for testing the orchestrator
// from other packages (e.g., handler_test).
type MockLLMClient struct {
	mu        sync.Mutex
	responses []MockLLMResponse
	idx       int
	calls     int
}

// NewMockLLMClient creates a mock LLM client with pre-programmed responses.
func NewMockLLMClient(resps ...MockLLMResponse) *MockLLMClient {
	return &MockLLMClient{responses: resps}
}

// Complete implements the LLMCompleter interface.
// When tools is nil, the mock always returns a text response (no tool calls),
// simulating realistic LLM behavior when tools are not offered.
func (m *MockLLMClient) Complete(_ context.Context, _ []llm.ChatMessage, tools []llm.Tool) (*llm.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls++

	if m.idx >= len(m.responses) {
		return &llm.ChatResponse{
			Choices: []llm.ChatChoice{{FinishReason: "stop", Message: llm.ChatMessage{Role: "assistant", Content: ""}}},
		}, nil
	}
	r := m.responses[m.idx]
	m.idx++

	if r.Err != nil {
		return nil, r.Err
	}

	// When tools is nil, return only text content (no tool calls) — realistic LLM behavior.
	// If content is empty, provide a fallback message (simulates LLM giving a summary when tools are not offered).
	if tools == nil {
		content := r.Content
		if content == "" {
			content = "基于已有的工具返回结果，以下是降级回答。"
		}
		return &llm.ChatResponse{
			Choices: []llm.ChatChoice{{
				FinishReason: "stop",
				Message:      llm.ChatMessage{Role: "assistant", Content: content},
			}},
		}, nil
	}

	resp := &llm.ChatResponse{
		Choices: []llm.ChatChoice{{
			FinishReason: "stop",
			Message:      llm.ChatMessage{Role: "assistant", Content: r.Content},
		}},
	}
	if len(r.ToolCalls) > 0 {
		tcs := make([]llm.ToolCall, 0, len(r.ToolCalls))
		for _, tc := range r.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Args)
			tcs = append(tcs, llm.ToolCall{
				ID:       "call_mock_" + tc.Name,
				Type:     "function",
				Function: llm.FunctionCall{Name: tc.Name, Arguments: string(argsJSON)},
			})
		}
		resp.Choices[0].Message.ToolCalls = tcs
		resp.Choices[0].FinishReason = "tool_calls"
	}
	return resp, nil
}

// CallCount returns the number of times Complete was called.
func (m *MockLLMClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}
