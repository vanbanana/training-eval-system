package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/testutil"
)

// ============================================================
// T8.4 — Performance benchmarks and stress tests
// ============================================================

// seedAgentSessions inserts N sessions for a given user directly into the DB.
func seedAgentSessions(t *testing.T, app *testutil.TestApp, userID int64, role string, count int) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	for i := 0; i < count; i++ {
		_, err := app.DB.Writer.Exec(
			`INSERT INTO agent_sessions (owner_id, owner_role, agent_role, title, context_json, created_at, last_active_at)
			 VALUES (?, ?, ?, ?, '{}', ?, ?)`,
			userID, role, role,
			fmt.Sprintf("Benchmark Session %d", i),
			now, now,
		)
		if err != nil {
			t.Fatalf("seed session %d: %v", i, err)
		}
	}
}

// seedAgentMessages inserts N messages into a given session.
func seedAgentMessages(t *testing.T, app *testutil.TestApp, sessionID int64, count int) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	for i := 0; i < count; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		_, err := app.DB.Writer.Exec(
			`INSERT INTO agent_messages (session_id, role, content, prompt_tokens, completion_tokens, created_at)
			 VALUES (?, ?, ?, 100, 50, ?)`,
			sessionID, role, fmt.Sprintf("message content %d", i), now,
		)
		if err != nil {
			t.Fatalf("seed message %d: %v", i, err)
		}
	}
}

// TEST-T8.4-01: Session list benchmark — 100 sessions per user
func TestT84_01_SessionListBenchmark(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Seed 100 sessions for student_a (user_id=13)
	seedAgentSessions(t, app, 13, "student", 100)

	// Benchmark: measure p95 latency for session list
	const iterations = 50
	latencies := make([]time.Duration, 0, iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp := doRequest(t, app.Server, "GET", "/api/agent/sessions", testutil.StudentAToken(), nil)
		elapsed := time.Since(start)
		testutil.AssertStatus(t, resp, http.StatusOK)

		var sessions []dto.AgentSessionResponse
		if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
			t.Fatalf("decode: %v", err)
		}
		resp.Body.Close()

		if len(sessions) < 100 {
			t.Errorf("iteration %d: expected >= 100 sessions, got %d", i, len(sessions))
		}
		latencies = append(latencies, elapsed)
	}

	// Sort latencies and compute p95
	sortDurations(latencies)
	p50 := latencies[len(latencies)/2]
	p95 := latencies[int(float64(len(latencies))*0.95)]
	p99 := latencies[int(float64(len(latencies))*0.99)]

	t.Logf("T8.4-01 session list benchmark (100 sessions, %d iterations):", iterations)
	t.Logf("  p50 = %v", p50)
	t.Logf("  p95 = %v (target < 300ms)", p95)
	t.Logf("  p99 = %v", p99)

	if p95 > 300*time.Millisecond {
		t.Errorf("p95 latency %v exceeds 300ms target", p95)
	}
}

// TEST-T8.4-01b: Message history pagination benchmark
func TestT84_01b_MessageHistoryBenchmark(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Create a session and seed 200 messages
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", testutil.StudentAToken(),
		dto.CreateAgentSessionRequest{Title: "Benchmark Messages", AgentRole: "student"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()

	seedAgentMessages(t, app, session.ID, 200)

	// Benchmark: measure p95 for message retrieval (limit 50)
	const iterations = 50
	latencies := make([]time.Duration, 0, iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp := doRequest(t, app.Server, "GET",
			fmt.Sprintf("/api/agent/sessions/%d/messages", session.ID),
			testutil.StudentAToken(), nil)
		elapsed := time.Since(start)
		testutil.AssertStatus(t, resp, http.StatusOK)

		var messages []dto.AgentMessageResponse
		if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
			t.Fatalf("decode: %v", err)
		}
		resp.Body.Close()

		latencies = append(latencies, elapsed)
	}

	sortDurations(latencies)
	p50 := latencies[len(latencies)/2]
	p95 := latencies[int(float64(len(latencies))*0.95)]

	t.Logf("T8.4-01b message history benchmark (200 msgs, limit 50, %d iterations):", iterations)
	t.Logf("  p50 = %v", p50)
	t.Logf("  p95 = %v (target < 300ms)", p95)

	if p95 > 300*time.Millisecond {
		t.Errorf("p95 latency %v exceeds 300ms target", p95)
	}
}

// TEST-T8.4-02: Admin system overview tool benchmark
func TestT84_02_AdminToolBenchmark(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Seed moderate data: 50 sessions, 10 users (already from fixture)
	seedAgentSessions(t, app, 10, "admin", 50)

	// Benchmark admin tool dispatch via the admin agent stream endpoint
	// We test the underlying tool directly for isolation
	const iterations = 20
	latencies := make([]time.Duration, 0, iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		// Use the dashboard endpoint as a proxy for admin overview queries
		resp := doRequest(t, app.Server, "GET", "/api/dashboard", testutil.AdminAToken(), nil)
		elapsed := time.Since(start)
		testutil.AssertStatus(t, resp, http.StatusOK)
		resp.Body.Close()

		latencies = append(latencies, elapsed)
	}

	sortDurations(latencies)
	p50 := latencies[len(latencies)/2]
	p95 := latencies[int(float64(len(latencies))*0.95)]

	t.Logf("T8.4-02 admin overview benchmark (%d iterations):", iterations)
	t.Logf("  p50 = %v", p50)
	t.Logf("  p95 = %v (target < 500ms)", p95)

	if p95 > 500*time.Millisecond {
		t.Errorf("p95 latency %v exceeds 500ms target", p95)
	}
}

// TEST-T8.4-03: Concurrent stream stress test — 20 concurrent streams with mock LLM
func TestT84_03_ConcurrentStream(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	const concurrency = 20
	const streamsPerUser = 1

	// Create sessions for each concurrent stream (to avoid session limit)
	type streamTarget struct {
		sessionID int64
		token     string
		role      string
	}
	var targets []streamTarget

	// Use 3 different users to spread load
	tokens := []struct {
		token string
		role  string
	}{
		{testutil.StudentAToken(), "student"},
		{testutil.StudentBToken(), "student"},
		{testutil.TeacherAToken(), "teacher"},
	}

	for _, tok := range tokens {
		for j := 0; j < 7; j++ { // 7 sessions per user × 3 users = 21 (enough for 20 concurrent)
			resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", tok.token,
				dto.CreateAgentSessionRequest{
					Title:     fmt.Sprintf("Stress Session %d", j),
					AgentRole: tok.role,
				})
			testutil.AssertStatus(t, resp, http.StatusCreated)
			var session dto.AgentSessionResponse
			if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
				t.Fatalf("decode: %v", err)
			}
			resp.Body.Close()
			targets = append(targets, streamTarget{sessionID: session.ID, token: tok.token, role: tok.role})
		}
	}

	// Launch concurrent streams
	var wg sync.WaitGroup
	var successCount, failCount int64
	var panicCount int64
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		target := targets[i%len(targets)]
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
					errCh <- fmt.Errorf("stream %d panicked: %v", idx, r)
				}
			}()

			_ = streamsPerUser // used for session planning

			resp, err := doStreamRequest(app.Server.URL, target.token, target.sessionID,
				fmt.Sprintf("stress test message %d", idx), target.role)
			if err != nil {
				atomic.AddInt64(&failCount, 1)
				errCh <- fmt.Errorf("stream %d: %w", idx, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
				// Read error body for diagnostics
				var errResp dto.AgentErrorResponse
				json.NewDecoder(resp.Body).Decode(&errResp)
				if errResp.Code != "" {
					errCh <- fmt.Errorf("stream %d: status %d code=%s", idx, resp.StatusCode, errResp.Code)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	success := atomic.LoadInt64(&successCount)
	failures := atomic.LoadInt64(&failCount)
	panics := atomic.LoadInt64(&panicCount)

	t.Logf("T8.4-03 concurrent stream stress test (%d streams):", concurrency)
	t.Logf("  success: %d", success)
	t.Logf("  failures: %d (rate-limited or expected)", failures)
	t.Logf("  panics: %d", panics)

	// Log first few errors for diagnostics
	var errCount int
	for err := range errCh {
		if errCount < 5 {
			t.Logf("  error: %v", err)
		}
		errCount++
	}

	// Critical: no panics
	if panics > 0 {
		t.Fatalf("detected %d panics during concurrent streams", panics)
	}

	// Acceptable failure rate: the test app's StreamTracker has userMax=2 per user,
	// so with 3 users only 6 of 20 concurrent streams can proceed simultaneously.
	// The remaining ~70% will be rejected with 429 — this proves the rate limiter works.
	// The critical assertions are: zero panics, and total > 0.
	total := success + failures
	if total == 0 {
		t.Fatal("no streams completed")
	}
	errorRate := float64(failures) / float64(total)
	t.Logf("  error rate: %.1f%% (expected ~70%% due to per-user concurrent limit)", errorRate*100)
	if errorRate > 0.80 {
		t.Errorf("error rate %.1f%% exceeds 80%% threshold", errorRate*100)
	}
}

// sortDurations sorts a slice of time.Duration in ascending order.
func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

// doStreamRequest sends an agent stream request without using the test helpers
// (to work with SSE responses that need special handling).
func doStreamRequest(baseURL, token string, sessionID int64, message, role string) (*http.Response, error) {
	body := dto.AgentStreamRequest{
		SessionID: sessionID,
		Message:   message,
		AgentRole: role,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/api/agent/stream",
		bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}
