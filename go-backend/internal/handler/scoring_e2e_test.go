package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
	"github.com/smartedu/training-eval-system/testutil"
)

// TestE2E_Scoring_FullPipeline verifies the complete AI scoring pipeline:
// auto-score (with FakeLLM returning submit_scores) → DB state → confirm → report.
func TestE2E_Scoring_FullPipeline(t *testing.T) {
	fakeLLM := testutil.NewFakeLLM()

	// The scoring prompt builds a tool schema; FakeLLM returns a valid submit_scores call.
	fakeLLM.WithToolCallResponse("submit_scores", map[string]any{
		"scores": []map[string]any{
			{"dimension_id": 200, "score": 88.0, "rationale": "代码结构清晰，逻辑正确"},
			{"dimension_id": 201, "score": 75.0, "rationale": "文档基本完整，可读性较好"},
			{"dimension_id": 202, "score": 92.0, "rationale": "功能完全实现，测试覆盖率高"},
		},
	})

	app := testutil.SetupTestAppWithLLM(t, fakeLLM)
	ctx := context.Background()

	// Seed dataset: task, course, class, uploaded+parsed file, NO evaluation yet.
	f := seedScoringFixture(t, app.DB)

	teacherToken := testutil.GenerateTestToken(f.TeacherAID, "teacher_a", "teacher")

	t.Run("auto-score creates evaluation and triggers scoring", func(t *testing.T) {
		resp := doRequest(t, app.Server, "POST",
			fmt.Sprintf("/api/grading/tasks/%d/auto-score", f.TaskAID),
			teacherToken, map[string]string{"mode": "unscored"})
		testutil.AssertStatus(t, resp, http.StatusOK)

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode auto-score response: %v", err)
		}
		t.Logf("auto-score result: %+v", result)

		if result["queued"] != float64(1) && result["requested"] != float64(1) {
			// Allow either "queued" or "requested" count ≥ 1
			queued, _ := result["queued"].(float64)
			requested, _ := result["requested"].(float64)
			if queued+requested < 1 {
				t.Errorf("expected at least 1 queued/requested, got queued=%v requested=%v", result["queued"], result["requested"])
			}
		}
	})

	t.Run("evaluation is scored after AI pipeline completes", func(t *testing.T) {
		// Poll for up to 5 seconds for the async scoring to finish
		var status string
		var evalID int64
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			err := app.DB.Reader.QueryRowContext(ctx,
				`SELECT id, status FROM evaluations WHERE task_id=? AND student_id=?`,
				f.TaskAID, f.StudentAID).Scan(&evalID, &status)
			if err == nil && status == "scored" {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if status != "scored" {
			t.Fatalf("expected status=scored after polling (5s), got status=%q", status)
		}
		if evalID == 0 {
			t.Fatal("expected non-zero evaluation ID")
		}
		f.EvalAID = evalID
		t.Logf("evaluation %d scored successfully", evalID)
	})

	t.Run("dimension scores are saved", func(t *testing.T) {
		var scoreCount int
		err := app.DB.Reader.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM dimension_scores WHERE evaluation_id=?", f.EvalAID).Scan(&scoreCount)
		if err != nil {
			t.Fatalf("query dimension scores: %v", err)
		}
		if scoreCount == 0 {
			t.Fatal("expected at least 1 dimension score after auto-score")
		}
		t.Logf("dimension scores count: %d", scoreCount)

		// Verify specific scores
		for dimID, expectedScore := range map[int64]float64{200: 88.0, 201: 75.0, 202: 92.0} {
			var aiScore float64
			err := app.DB.Reader.QueryRowContext(ctx,
				"SELECT ai_score FROM dimension_scores WHERE evaluation_id=? AND dimension_id=?",
				f.EvalAID, dimID).Scan(&aiScore)
			if err != nil {
				t.Errorf("query dim %d score: %v", dimID, err)
				continue
			}
			if aiScore != expectedScore {
				t.Errorf("dim %d: expected ai_score=%f, got %f", dimID, expectedScore, aiScore)
			}
		}
	})

	t.Run("teacher confirms evaluation", func(t *testing.T) {
		resp := doRequest(t, app.Server, "POST",
			fmt.Sprintf("/api/grading/evaluations/%d/confirm", f.EvalAID),
			teacherToken, nil)
		testutil.AssertStatus(t, resp, http.StatusOK)

		var status string
		app.DB.Reader.QueryRowContext(ctx,
			"SELECT status FROM evaluations WHERE id=?", f.EvalAID).Scan(&status)
		if status != "confirmed" {
			t.Errorf("expected status=confirmed, got %s", status)
		}
	})

	t.Run("student can download personal report", func(t *testing.T) {
		studentToken := testutil.GenerateTestToken(f.StudentAID, "student_a", "student")
		resp := doRequest(t, app.Server, "GET",
			fmt.Sprintf("/api/reports/personal/%d", f.EvalAID),
			studentToken, nil)
		testutil.AssertStatus(t, resp, http.StatusOK)
	})
}

// scoringFixture holds IDs for the scoring E2E test.
type scoringFixture struct {
	AdminID    int64
	TeacherAID int64
	StudentAID int64
	CourseAID  int64
	ClassAID   int64
	TaskAID    int64
	UploadAID  int64
	EvalAID    int64 // populated during test
}

// seedScoringFixture creates a task + uploaded/parsed file without an evaluation.
func seedScoringFixture(t *testing.T, db *store.DB) *scoringFixture {
	t.Helper()
	ctx := context.Background()
	w := db.Writer
	now := time.Now()
	deadline := now.Add(7 * 24 * time.Hour)
	f := &scoringFixture{}

	// Users
	w.ExecContext(ctx, "INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (10,'admin','管理员','x','admin',1)")
	f.AdminID = 10
	w.ExecContext(ctx, "INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (11,'teacher_a','教师A','x','teacher',1)")
	f.TeacherAID = 11
	w.ExecContext(ctx, "INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (13,'student_a','学生A','x','student',1)")
	f.StudentAID = 13

	// Course + class
	w.ExecContext(ctx, "INSERT INTO courses (id,name,code,is_archived) VALUES (200,'评分测试课程','SCORE-TEST',0)")
	f.CourseAID = 200
	w.ExecContext(ctx, "INSERT INTO classes (id,name,course_id,teacher_id,student_count,is_archived) VALUES (200,'评分测试班',200,11,0,0)")
	f.ClassAID = 200
	w.ExecContext(ctx, "INSERT INTO class_memberships (class_id,student_id) VALUES (200,13)")

	// Task with dimensions: 代码质量(40), 文档完整(30), 功能完整(30)
	w.ExecContext(ctx,
		`INSERT INTO training_tasks (id,name,description,requirements,teacher_id,course_id,status,deadline)
		 VALUES (200,'评分测试任务','E2E Scoring Test','Implement Go API',11,200,'published',?)`, deadline)
	f.TaskAID = 200
	w.ExecContext(ctx, "INSERT INTO task_classes (task_id,class_id) VALUES (200,200)")

	w.ExecContext(ctx, "INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (200,200,'代码质量',40,0)")
	w.ExecContext(ctx, "INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (201,200,'文档完整',30,1)")
	w.ExecContext(ctx, "INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (202,200,'功能完整',30,2)")

	// Upload: parsed but NO evaluation yet
	w.ExecContext(ctx,
		`INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status,created_at)
		 VALUES (200,200,13,'report.pdf','pdf',1024,'/tmp/score_test.pdf','parsed',?)`, now)
	f.UploadAID = 200

	// Parse result
	w.ExecContext(ctx,
		"INSERT INTO parse_results (upload_id,raw_text,parsed_at) VALUES (200,'实验目的：掌握Go语言。实验步骤：1.安装 2.编码 3.测试。实验结论：成功。',?)", now)

	return f
}

// Ensure unused imports are suppressed
var _ = model.Evaluation{}

// TestE2E_Agent_ToolCalls verifies the teacher agent tool call path runs through
// FakeLLM correctly, producing expected tool call → tool result → final response flow.
func TestE2E_Agent_ToolCalls(t *testing.T) {
	fakeLLM := testutil.NewFakeLLM()
	// First Complete call: return a teacher_get_task_summary tool call
	fakeLLM.WithToolCallResponse("teacher_get_task_summary", map[string]any{
		"task_id": 200,
	})
	// Second Complete call (after tool result is fed back): return final text
	fakeLLM.WithResponses("根据数据，该任务共有 1 名学生提交了作业。")

	app := testutil.SetupTestAppWithLLM(t, fakeLLM)
	f := seedScoringFixture(t, app.DB)
	token := testutil.GenerateTestToken(f.TeacherAID, "teacher_a", "teacher")

	// Create teacher session
	resp := doRequest(t, app.Server, "POST", "/api/agent/sessions", token,
		map[string]any{"title": "Agent Tool Test", "agent_role": "teacher"})
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var session struct {
		ID        int64  `json:"id"`
		AgentRole string `json:"agent_role"`
	}
	json.NewDecoder(resp.Body).Decode(&session)

	// Send message with task context — this triggers the teacher tool flow
	resp = doRequest(t, app.Server, "POST", "/api/agent/stream", token,
		map[string]any{
			"session_id": session.ID,
			"message":    "总结这个任务的提交情况",
			"agent_role": "teacher",
			"context":    map[string]any{"task_id": f.TaskAID},
		})
	testutil.AssertStatus(t, resp, http.StatusOK)

	body := readAll(resp.Body)
	events := parseSSEEvents(t, string(body))

	var hasToolCall, hasText, hasDone bool
	for _, evt := range events {
		typ, _ := evt["type"].(string)
		switch typ {
		case "tool_start":
			hasToolCall = true
		case "text":
			if content, ok := evt["content"].(string); ok && content != "" {
				hasText = true
			}
		case "done":
			hasDone = true
		}
	}

	if !hasToolCall {
		t.Error("expected tool_start event in agent stream — teacher tool was not triggered")
	}
	if !hasText {
		t.Error("expected text event in agent stream")
	}
	if !hasDone {
		t.Error("expected done event in agent stream")
	}
	if !hasToolCall || !hasText || !hasDone {
		t.Logf("SSE events received (%d):", len(events))
		for i, evt := range events {
			t.Logf("  event[%d]: type=%v content=%v", i, evt["type"], truncateStr(evt["content"], 80))
		}
	}
}

func readAll(r io.ReadCloser) string {
	b, _ := io.ReadAll(r)
	r.Close()
	return string(b)
}

func truncateStr(s any, max int) string {
	if s == nil {
		return "<nil>"
	}
	str, ok := s.(string)
	if !ok {
		return fmt.Sprintf("%v", s)
	}
	if len([]rune(str)) > max {
		return string([]rune(str)[:max]) + "..."
	}
	return str
}