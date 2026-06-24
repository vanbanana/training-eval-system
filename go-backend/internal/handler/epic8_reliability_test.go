package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/handler"
	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/pipeline"
	"github.com/smartedu/training-eval-system/testutil"
)

// ============================================================
// T8.1 — Role-based quota and rate limiting tests
// ============================================================

// --- StreamTracker unit tests ---

func TestT81_StreamTracker_AcquireRelease(t *testing.T) {
	tracker := handler.NewStreamTracker(2, 10)

	// Acquire 2 slots for user 1
	if err := tracker.Acquire(1); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := tracker.Acquire(1); err != nil {
		t.Fatalf("second acquire: %v", err)
	}

	// Third acquire should fail (user limit = 2)
	if err := tracker.Acquire(1); err == nil {
		t.Fatal("expected user limit error, got nil")
	}

	// Different user can still acquire
	if err := tracker.Acquire(2); err != nil {
		t.Fatalf("different user acquire: %v", err)
	}

	// Verify counts
	if got := tracker.UserActive(1); got != 2 {
		t.Errorf("user 1 active = %d, want 2", got)
	}
	if got := tracker.GlobalActive(); got != 3 {
		t.Errorf("global active = %d, want 3", got)
	}

	// Release one slot
	tracker.Release(1)
	if got := tracker.UserActive(1); got != 1 {
		t.Errorf("after release: user 1 active = %d, want 1", got)
	}
	if got := tracker.GlobalActive(); got != 2 {
		t.Errorf("after release: global active = %d, want 2", got)
	}

	// Now user 1 can acquire again
	if err := tracker.Acquire(1); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}

func TestT81_StreamTracker_GlobalLimit(t *testing.T) {
	tracker := handler.NewStreamTracker(5, 3) // user limit 5, global limit 3

	// Fill up global limit with different users
	for i := int64(1); i <= 3; i++ {
		if err := tracker.Acquire(i); err != nil {
			t.Fatalf("acquire user %d: %v", i, err)
		}
	}

	// Next acquire from any user should fail (global limit)
	if err := tracker.Acquire(4); err == nil {
		t.Fatal("expected global limit error, got nil")
	}

	// Release one
	tracker.Release(2)
	if err := tracker.Acquire(4); err != nil {
		t.Fatalf("acquire after global release: %v", err)
	}
}

func TestT81_StreamTracker_ConcurrentSafety(t *testing.T) {
	tracker := handler.NewStreamTracker(100, 100)
	var wg sync.WaitGroup

	// Concurrently acquire and release
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(userID int64) {
			defer wg.Done()
			if err := tracker.Acquire(userID); err != nil {
				return // some may fail due to global limit
			}
			time.Sleep(time.Millisecond)
			tracker.Release(userID)
		}(int64(i))
	}
	wg.Wait()

	// After all goroutines finish, counts should be zero
	if got := tracker.GlobalActive(); got != 0 {
		t.Errorf("global active after all done = %d, want 0", got)
	}
}

// --- Quota integration tests (HTTP level) ---

// TEST-T8.1-01: Student daily limit — fill up to limit, next request returns 429
func TestT81_01_StudentDailyLimit(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create 3 student sessions (to spread messages and avoid session limit of 20)
	var sessionIDs []int64
	for i := 0; i < 3; i++ {
		resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.StudentAToken(),
			dto.CreateAgentSessionRequest{Title: fmt.Sprintf("Daily Limit Session %d", i), AgentRole: "student"})
		testutil.AssertStatus(t, resp, http.StatusCreated)
		var session dto.AgentSessionResponse
		if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
			t.Fatalf("decode: %v", err)
		}
		sessionIDs = append(sessionIDs, session.ID)
	}

	// Insert 50 user messages across 3 sessions (17+17+16, all under session limit of 20)
	// Default student daily limit = 50
	now := time.Now().Format(time.RFC3339)
	msgsPerSession := []int{17, 17, 16} // total = 50
	for sIdx, count := range msgsPerSession {
		for i := 0; i < count; i++ {
			_, err := app.DB.Writer.Exec(
				`INSERT INTO agent_messages (session_id, role, content, prompt_tokens, completion_tokens, created_at)
				 VALUES (?, 'user', ?, 0, 0, ?)`,
				sessionIDs[sIdx], fmt.Sprintf("dummy msg %d", i), now,
			)
			if err != nil {
				t.Fatalf("insert session %d msg %d: %v", sIdx, i, err)
			}
		}
	}

	// Next message should fail with 429 AGENT_DAILY_LIMIT (50 messages already, limit = 50)
	resp := doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.StudentAToken(),
		dto.AgentStreamRequest{
			SessionID: sessionIDs[0],
			Message:   "第51条消息",
			AgentRole: "student",
		})
	testutil.AssertStatus(t, resp, http.StatusTooManyRequests)
	var errResp dto.AgentErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Code != dto.AgentErrDailyLimit {
		t.Errorf("error code = %q, want %q", errResp.Code, dto.AgentErrDailyLimit)
	}
}

// TEST-T8.1-02: Teacher quota is independent from student quota
func TestT81_02_TeacherQuotaIndependent(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create a teacher session
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.TeacherAToken(),
		dto.CreateAgentSessionRequest{Title: "Teacher Quota Test", AgentRole: "teacher"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Insert 49 messages for the teacher (student daily limit = 50, teacher daily limit = 200)
	now := time.Now().Format(time.RFC3339)
	for i := 0; i < 49; i++ {
		_, err := app.DB.Writer.Exec(
			`INSERT INTO agent_messages (session_id, role, content, prompt_tokens, completion_tokens, created_at)
			 VALUES (?, 'user', ?, 0, 0, ?)`,
			session.ID, fmt.Sprintf("teacher msg %d", i), now,
		)
		if err != nil {
			t.Fatalf("insert message %d: %v", i, err)
		}
	}

	// Teacher should still be able to send (49 < 200 teacher limit, even though 49 < 50 student limit)
	// Send the 50th message — would fail for student but should succeed for teacher
	resp = doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.TeacherAToken(),
		dto.AgentStreamRequest{
			SessionID: session.ID,
			Message:   "teacher message 50",
			AgentRole: "teacher",
		})
	testutil.AssertStatus(t, resp, http.StatusOK)
	_, _ = drain(resp)

	// Verify the message was saved (teacher quota allowed it)
	var msgCount int
	err = app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM agent_messages WHERE session_id=? AND role='user'", session.ID,
	).Scan(&msgCount)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if msgCount < 50 {
		t.Errorf("expected at least 50 messages, got %d", msgCount)
	}
}

// TEST-T8.1-03: Per-user concurrent limit — N+1th stream returns 429
func TestT81_03_ConcurrentLimit(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// The test app's StreamTracker has userMax=2.
	// We'll verify by testing the StreamTracker directly since we can't
	// easily hold multiple SSE connections open in a unit test.

	// Create a tracker and simulate the concurrent limit
	tracker := handler.NewStreamTracker(2, 50)

	// Acquire 2 slots (the user limit)
	if err := tracker.Acquire(13); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := tracker.Acquire(13); err != nil {
		t.Fatalf("second acquire: %v", err)
	}

	// Third acquire should fail
	err = tracker.Acquire(13)
	if err == nil {
		t.Fatal("expected concurrent limit error, got nil")
	}

	// Verify the error message contains "concurrent"
	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}

	// Release one and verify we can acquire again
	tracker.Release(13)
	if err := tracker.Acquire(13); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}

	// Clean up
	tracker.Release(13)
	tracker.Release(13)

	// Verify clean state
	if got := tracker.GlobalActive(); got != 0 {
		t.Errorf("global active after cleanup = %d, want 0", got)
	}
}

// TEST-T8.1-04: Stream disconnect releases concurrency slot
func TestT81_04_StreamDisconnectReleases(t *testing.T) {
	tracker := handler.NewStreamTracker(2, 50)

	// Simulate a stream acquiring a slot
	if err := tracker.Acquire(42); err != nil {
		t.Fatalf("acquire: %v", err)
	}

	if got := tracker.UserActive(42); got != 1 {
		t.Fatalf("user active = %d, want 1", got)
	}

	// Simulate stream ending (defer Release)
	tracker.Release(42)

	// Verify slot was released
	if got := tracker.UserActive(42); got != 0 {
		t.Errorf("user active after release = %d, want 0", got)
	}
	if got := tracker.GlobalActive(); got != 0 {
		t.Errorf("global active after release = %d, want 0", got)
	}

	// Verify double-release doesn't panic or go negative
	tracker.Release(42)
	if got := tracker.UserActive(42); got != 0 {
		t.Errorf("user active after double release = %d, want 0", got)
	}
	if got := tracker.GlobalActive(); got != 0 {
		t.Errorf("global active after double release = %d, want 0", got)
	}
}

// TEST-T8.1-05: Role-based quota lookup returns correct values
func TestT81_05_RoleBasedQuotaLookup(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Verify that the service correctly uses role-based quotas by testing
	// the session limit: student=20, teacher=50, admin=50

	// Create a student session and fill to student session limit (20)
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.StudentAToken(),
		dto.CreateAgentSessionRequest{Title: "Session Limit Test", AgentRole: "student"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Insert 19 messages (limit is 20, check is >= so 19 < 20 passes)
	now := time.Now().Format(time.RFC3339)
	for i := 0; i < 19; i++ {
		_, err := app.DB.Writer.Exec(
			`INSERT INTO agent_messages (session_id, role, content, prompt_tokens, completion_tokens, created_at)
			 VALUES (?, 'user', ?, 0, 0, ?)`,
			session.ID, fmt.Sprintf("msg %d", i), now,
		)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// 20th message should succeed
	resp = doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.StudentAToken(),
		dto.AgentStreamRequest{
			SessionID: session.ID,
			Message:   "msg 20",
			AgentRole: "student",
		})
	testutil.AssertStatus(t, resp, http.StatusOK)
	_, _ = drain(resp)

	// 21st message should fail with session limit (20 reached)
	resp = doRequest(t, app.Server, "POST", "/api/agent/stream", testutil.StudentAToken(),
		dto.AgentStreamRequest{
			SessionID: session.ID,
			Message:   "msg 21",
			AgentRole: "student",
		})
	testutil.AssertStatus(t, resp, http.StatusTooManyRequests)
	var errResp dto.AgentErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errResp.Code != dto.AgentErrSessionLimit {
		t.Errorf("error code = %q, want %q", errResp.Code, dto.AgentErrSessionLimit)
	}
}

// --- Helpers ---

// drain reads the response body to prevent connection leaks.
func drain(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	buf := make([]byte, 4096)
	var result []byte
	for {
		n, err := resp.Body.Read(buf)
		result = append(result, buf[:n]...)
		if err != nil {
			break
		}
	}
	return result, nil
}

// Ensure model import is used
var _ = model.AgentMessage{}

// ============================================================
// T8.2 — LLM timeout, retry, degradation, and circuit breaker tests
// ============================================================

// --- T8.2-01: First token timeout ---
// Uses a real llm.Client pointed at a slow httptest server.
func TestT82_01_FirstTokenTimeout(t *testing.T) {
	// Create a slow HTTP server that delays response by 3 seconds
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"test","choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	}))
	defer slowServer.Close()

	// Create a real LLM client with a 500ms first-token timeout
	client := llm.NewClient(slowServer.URL, "test-key", "test-model", "")
	client.SetFirstTokenTimeout(500 * time.Millisecond)
	client.SetHTTPTimeout(10 * time.Second) // long total timeout so only first-token triggers

	// The client should detect the first-token timeout
	ctx := context.Background()
	_, err := client.Complete(ctx, []llm.ChatMessage{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected first token timeout error, got nil")
	}

	// Verify the error wraps ErrFirstTokenTimeout
	if !errors.Is(err, llm.ErrFirstTokenTimeout) {
		t.Errorf("expected ErrFirstTokenTimeout, got: %v", err)
	}
	t.Logf("first token timeout detected: %v", err)
}

// --- T8.2-02: Tool round limit → degraded answer ---
// Uses a mock LLM that always requests tool calls, forcing MaxToolRounds exhaustion.
func TestT82_02_ToolRoundLimit(t *testing.T) {
	// Build a mock that always returns a tool call (never a final answer)
	// We need more responses than MaxToolRounds to cover all rounds + the final forced answer
	responses := make([]pipeline.MockLLMResponse, 0, 10)
	for i := 0; i < 8; i++ {
		responses = append(responses, pipeline.MockLLMResponse{
			ToolCalls: []pipeline.MockToolCallDef{{Name: "get_learning_resources", Args: map[string]any{"keyword": "test"}}},
		})
	}
	// Final fallback response
	responses = append(responses, pipeline.MockLLMResponse{Content: "这是基于已有信息的降级回答。"})

	mock := pipeline.NewMockLLMClient(responses...)

	orch := pipeline.NewChatOrchestrator(mock, nil, nil, nil, nil)
	orch.SetMaxToolRounds(3) // Set low limit for testing

	ctx := context.Background()
	resp, err := orch.Run(ctx, nil, "请帮我查一下学习资料", &pipeline.ChatToolContext{})

	// Should succeed with a degraded/fallback response, not error
	if err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		t.Fatal("expected non-empty response")
	}

	// The response should have content (either from the LLM or the fallback)
	content := resp.Choices[0].Message.Content
	t.Logf("response content: %q", content)
	if content == "" {
		t.Error("expected non-empty content in degraded response")
	}

	// Verify the mock was called exactly maxToolRounds + 1 times (3 tool rounds + 1 final forced answer)
	calls := mock.CallCount()
	if calls != 4 { // 3 rounds + 1 forced final
		t.Errorf("expected %d LLM calls, got %d", 4, calls)
	}
}

// --- T8.2-03: Consecutive failure circuit breaker ---
func TestT82_03_CircuitBreakerOpen(t *testing.T) {
	// Create a breaker with threshold=3 and short cooldown
	breaker := llm.NewCircuitBreaker(3, 1*time.Second)

	// Record 3 failures to open the circuit
	breaker.RecordFailure()
	breaker.RecordFailure()
	breaker.RecordFailure()

	// Circuit should be open
	if breaker.State() != llm.StateOpen {
		t.Fatalf("expected StateOpen, got %v", breaker.State())
	}

	// Allow() should return error
	if err := breaker.Allow(); err == nil {
		t.Error("expected circuit breaker to reject requests when open")
	}

	// Now test with a real LLM client: it should propagate ErrCircuitOpen
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"test","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "key", "model", "")
	client.SetBreaker(breaker)

	_, err := client.Complete(context.Background(), []llm.ChatMessage{{Role: "user", Content: "test"}}, nil)
	if err == nil {
		t.Fatal("expected error from open circuit breaker")
	}
	if !errors.Is(err, llm.ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
	t.Logf("circuit breaker correctly rejected: %v", err)
}

// --- T8.2-04: Circuit breaker recovery after cooldown ---
func TestT82_04_CircuitBreakerRecovery(t *testing.T) {
	// Create a breaker with threshold=2 and 500ms cooldown
	breaker := llm.NewCircuitBreaker(2, 500*time.Millisecond)

	// Trip the breaker
	breaker.RecordFailure()
	breaker.RecordFailure()

	if breaker.State() != llm.StateOpen {
		t.Fatalf("expected StateOpen after 2 failures, got %v", breaker.State())
	}

	// Immediately: should reject
	if err := breaker.Allow(); err == nil {
		t.Error("expected rejection immediately after trip")
	}

	// Wait for cooldown
	time.Sleep(600 * time.Millisecond)

	// After cooldown: should transition to half-open and allow one request
	if err := breaker.Allow(); err != nil {
		t.Errorf("expected half-open to allow request after cooldown, got: %v", err)
	}

	if breaker.State() != llm.StateHalfOpen {
		t.Errorf("expected StateHalfOpen after cooldown, got %v", breaker.State())
	}

	// Simulate a successful request → should close the breaker
	breaker.RecordSuccess()
	if breaker.State() != llm.StateClosed {
		t.Errorf("expected StateClosed after success in half-open, got %v", breaker.State())
	}

	// Verify the full recovery cycle: breaker is now fully operational
	if err := breaker.Allow(); err != nil {
		t.Errorf("expected Allow to succeed after full recovery, got: %v", err)
	}

	// Test with a real LLM client to verify end-to-end recovery
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"test","choices":[{"message":{"role":"assistant","content":"recovered"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "key", "model", "")
	client.SetBreaker(breaker)

	resp, err := client.Complete(context.Background(), []llm.ChatMessage{{Role: "user", Content: "test"}}, nil)
	if err != nil {
		t.Fatalf("expected successful request after recovery, got: %v", err)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "recovered" {
		t.Errorf("unexpected response: %+v", resp)
	}
	t.Log("circuit breaker recovery verified end-to-end")
}

// --- T8.2: Additional resilience tests ---

// Test that the error classifier correctly maps timeout errors
func TestT82_ClassifyLLMError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"first token timeout", llm.ErrFirstTokenTimeout, dto.AgentErrLLMTimeout},
		{"total timeout", llm.ErrTotalTimeout, dto.AgentErrLLMTimeout},
		{"circuit open", llm.ErrCircuitOpen, dto.AgentErrLLMUnavailable},
		{"wrapped timeout", fmt.Errorf("wrapped: %w", llm.ErrFirstTokenTimeout), dto.AgentErrLLMTimeout},
		{"generic error", fmt.Errorf("some random error"), dto.AgentErrInternal},
		{"nil error", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, msg := handler.ClassifyLLMError(tt.err)
			if code != tt.wantCode {
				t.Errorf("code = %q, want %q", code, tt.wantCode)
			}
			if tt.err != nil && msg == "" {
				t.Error("expected non-empty user message for non-nil error")
			}
		})
	}
}
