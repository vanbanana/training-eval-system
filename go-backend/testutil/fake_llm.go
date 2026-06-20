// Package testutil provides test helpers including FakeLLM for unit testing.
package testutil

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/smartedu/training-eval-system/internal/llm"
)

// FakeLLM is a mock LLM client that returns preset responses for testing.
// Supports:
// - Preset response sequences
// - Function Calling simulation (tool_calls)
// - Error injection
// - Delay simulation
// - Call recording for test assertions
type FakeLLM struct {
	mu        sync.Mutex
	responses []fakeResponse
	index     int
	// If set to true, all calls return error
	failAll bool
	failErr error // the error to return when failAll is true
	// Simulated delay in milliseconds (0 = no delay)
	delayMs int
	// Optional call recorder for assertions
	recorder *FakeCallRecorder
}

type fakeResponse struct {
	content   string
	toolCalls []fakeToolCall
	err       error
}

type fakeToolCall struct {
	name      string
	arguments map[string]any
}

// FakeCall records a single call made to the FakeLLM for test assertions.
type FakeCall struct {
	Messages []llm.ChatMessage
	Tools    []llm.Tool
}

// FakeCallRecorder captures all calls made to FakeLLM.
type FakeCallRecorder struct {
	mu    sync.Mutex
	Calls []FakeCall
}

// NewFakeLLM creates a FakeLLM with optional preset responses.
// Each response is consumed in order; remaining calls return empty content.
func NewFakeLLM() *FakeLLM {
	return &FakeLLM{}
}

// WithResponses adds preset text responses that will be returned in order.
func (f *FakeLLM) WithResponses(contents ...string) *FakeLLM {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range contents {
		f.responses = append(f.responses, fakeResponse{content: c})
	}
	return f
}

// WithJSONResponse adds a preset JSON object response.
func (f *FakeLLM) WithJSONResponse(v any) *FakeLLM {
	data, _ := json.Marshal(v)
	return f.WithResponses(string(data))
}

// WithToolCallResponse adds a response that simulates a Function Calling tool_call response.
// The response will contain tool_calls instead of content.
func (f *FakeLLM) WithToolCallResponse(toolName string, args map[string]any) *FakeLLM {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses = append(f.responses, fakeResponse{
		toolCalls: []fakeToolCall{{name: toolName, arguments: args}},
	})
	return f
}

// WithError adds an error that will be returned on the next call.
func (f *FakeLLM) WithError(err error) *FakeLLM {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses = append(f.responses, fakeResponse{err: err})
	return f
}

// FailAll configures FakeLLM to fail all subsequent calls with the given error.
func (f *FakeLLM) FailAll(err error) *FakeLLM {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failAll = true
	f.failErr = err
	// Ensure at least one response exists for Complete to reference safely
	f.responses = append(f.responses, fakeResponse{err: err})
	return f
}

// WithDelay sets the simulated delay in milliseconds.
func (f *FakeLLM) WithDelay(ms int) *FakeLLM {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delayMs = ms
	return f
}

// WithCallRecorder attaches a call recorder that captures all LLM calls for test assertions.
func (f *FakeLLM) WithCallRecorder(r *FakeCallRecorder) *FakeLLM {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recorder = r
	return f
}

// Complete returns the next preset response. Implements the LLM calling interface.
// If WithCallRecorder was used, records the call.
// If WithDelay was used, simulates the delay before returning.
func (f *FakeLLM) Complete(ctx context.Context, messages []llm.ChatMessage, tools []llm.Tool) (*llm.ChatResponse, error) {
	// Phase 1: snapshot state under lock, handle delay outside lock
	f.mu.Lock()
	delay := f.delayMs
	recorder := f.recorder

	if f.failAll {
		// Capture the error to return immediately
		lastErr := f.failErr
		// Record the call
		if recorder != nil {
			recorder.mu.Lock()
			recorder.Calls = append(recorder.Calls, FakeCall{
				Messages: copyMessages(messages),
				Tools:    copyTools(tools),
			})
			recorder.mu.Unlock()
		}
		f.mu.Unlock()

		// Simulate delay
		if delay > 0 {
			select {
			case <-time.After(time.Duration(delay) * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		return nil, lastErr
	}

	if f.index >= len(f.responses) {
		f.mu.Unlock()

		if delay > 0 {
			select {
			case <-time.After(time.Duration(delay) * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Record the call if a recorder is attached
		if recorder != nil {
			recorder.mu.Lock()
			recorder.Calls = append(recorder.Calls, FakeCall{
				Messages: copyMessages(messages),
				Tools:    copyTools(tools),
			})
			recorder.mu.Unlock()
		}

		return &llm.ChatResponse{
			Choices: []llm.ChatChoice{
				{Message: llm.ChatMessage{Role: "assistant", Content: ""}, FinishReason: "stop"},
			},
		}, nil
	}

	idx := f.index
	resp := f.responses[idx]
	f.index++
	f.mu.Unlock()

	// Simulate delay outside lock
	if delay > 0 {
		select {
		case <-time.After(time.Duration(delay) * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Record the call if a recorder is attached
	if recorder != nil {
		recorder.mu.Lock()
		recorder.Calls = append(recorder.Calls, FakeCall{
			Messages: copyMessages(messages),
			Tools:    copyTools(tools),
		})
		recorder.mu.Unlock()
	}

	if resp.err != nil {
		return nil, resp.err
	}

	chatResp := &llm.ChatResponse{
		Choices: []llm.ChatChoice{
			{
				Index:        0,
				FinishReason: "stop",
				Message:      llm.ChatMessage{Role: "assistant", Content: resp.content},
			},
		},
		Usage: &llm.UsageInfo{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
	}

	if len(resp.toolCalls) > 0 {
		tcs := make([]llm.ToolCall, 0, len(resp.toolCalls))
		for _, tc := range resp.toolCalls {
			argsJSON, _ := json.Marshal(tc.arguments)
			tcs = append(tcs, llm.ToolCall{
				ID:   "fake_call_1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      tc.name,
					Arguments: string(argsJSON),
				},
			})
		}
		chatResp.Choices[0].Message.ToolCalls = tcs
		chatResp.Choices[0].FinishReason = "tool_calls"
		chatResp.Choices[0].Message.Content = resp.content
	}

	return chatResp, nil
}

// Reset resets the FakeLLM state.
func (f *FakeLLM) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses = nil
	f.index = 0
	f.failAll = false
	f.failErr = nil
	f.delayMs = 0
	f.recorder = nil
}

// Remaining returns the number of unconsumed responses.
func (f *FakeLLM) Remaining() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.responses) - f.index
}

// Model returns "fake-llm" as the model identifier.
func (f *FakeLLM) Model() string {
	return "fake-llm"
}

// Compile-time assertion that *FakeLLM implements llm.LLMClient.
var _ llm.LLMClient = (*FakeLLM)(nil)

// copyMessages deep-copies a message slice to avoid aliasing issues in recordings.
func copyMessages(src []llm.ChatMessage) []llm.ChatMessage {
	dst := make([]llm.ChatMessage, len(src))
	copy(dst, src)
	return dst
}

// copyTools deep-copies a tool slice for recording.
func copyTools(src []llm.Tool) []llm.Tool {
	dst := make([]llm.Tool, len(src))
	copy(dst, src)
	return dst
}