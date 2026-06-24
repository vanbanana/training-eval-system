package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/smartedu/training-eval-system/testutil"
)

// ============================================================
// T0.2 — Grading workflow regression tests (Epic 0)
// These tests use BuildGradingWorkflowFixture to seed data and
// verify the current behavior of grading endpoints.
// ============================================================

func TestGradingRegression_Fixture(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()

	f, err := testutil.BuildGradingWorkflowFixture(ctx, app.DB)
	if err != nil {
		t.Fatalf("BuildGradingWorkflowFixture: %v", err)
	}

	// ── Fixture integrity checks ──

	t.Run("TEST-T0.1-01 fixture builds", func(t *testing.T) {
		if f.TeacherAID == 0 || f.TeacherBID == 0 || f.StudentAID == 0 {
			t.Fatal("fixture did not populate IDs")
		}
	})

	t.Run("TEST-T0.1-02 course-class hierarchy", func(t *testing.T) {
		var courseID int64
		app.DB.Reader.QueryRowContext(ctx, "SELECT course_id FROM classes WHERE id=?", f.ClassA1ID).Scan(&courseID)
		if courseID != f.CourseAID {
			t.Fatalf("class A1 expected course %d, got %d", f.CourseAID, courseID)
		}
	})

	t.Run("TEST-T0.1-03 task-course consistent", func(t *testing.T) {
		var courseID int64
		app.DB.Reader.QueryRowContext(ctx, "SELECT course_id FROM training_tasks WHERE id=?", f.TaskAID).Scan(&courseID)
		if courseID != f.CourseAID {
			t.Fatalf("task A expected course %d, got %d", f.CourseAID, courseID)
		}
		var count int
		app.DB.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM task_classes WHERE task_id=?", f.TaskAID).Scan(&count)
		if count != 2 {
			t.Fatalf("task A expected 2 classes, got %d", count)
		}
	})

	t.Run("TEST-T0.1-04 cross-teacher isolation", func(t *testing.T) {
		var teacherID int64
		app.DB.Reader.QueryRowContext(ctx, "SELECT teacher_id FROM training_tasks WHERE id=?", f.TaskBID).Scan(&teacherID)
		if teacherID == f.TeacherAID {
			t.Fatal("teacher A should not own task B")
		}
	})

	// ── Grading endpoint tests ──

	teacherAToken := testutil.GenerateTestToken(f.TeacherAID, "teacher_a", "teacher")
	_ = testutil.GenerateTestToken(f.TeacherBID, "teacher_b", "teacher")

	t.Run("TEST-T0.2-01 teacher A gets task A submissions", func(t *testing.T) {
		resp := doRequest(t, app.Server, "GET", "/api/grading/tasks/1/submissions", teacherAToken, nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var submissions []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&submissions); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(submissions) < 2 {
			t.Fatalf("expected at least 2 submissions, got %d", len(submissions))
		}
	})

	t.Run("TEST-T0.2-02 teacher A gets task A summary", func(t *testing.T) {
		resp := doRequest(t, app.Server, "GET", "/api/grading/tasks/1/summary", teacherAToken, nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("TEST-T0.2-03 confirm scored evaluation", func(t *testing.T) {
		resp := doRequest(t, app.Server, "POST", "/api/grading/evaluations/1/confirm", teacherAToken, nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var status string
		app.DB.Reader.QueryRowContext(ctx, "SELECT status FROM evaluations WHERE id=1").Scan(&status)
		if status != "confirmed" {
			t.Fatalf("expected confirmed, got %s", status)
		}
	})

	t.Run("TEST-T0.2-04 reject scored", func(t *testing.T) {
		app.DB.Writer.ExecContext(ctx, `UPDATE evaluations SET status='scored' WHERE id=1`)
		resp := doRequest(t, app.Server, "POST", "/api/grading/evaluations/1/reject", teacherAToken, nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var status string
		app.DB.Reader.QueryRowContext(ctx, "SELECT status FROM evaluations WHERE id=1").Scan(&status)
		if status != "rejected" {
			t.Fatalf("expected rejected, got %s", status)
		}
	})

	t.Run("TEST-T0.2-05 bulk confirm", func(t *testing.T) {
		app.DB.Writer.ExecContext(ctx, `UPDATE evaluations SET status='scored' WHERE id=1`)
		resp := doRequest(t, app.Server, "POST", "/api/evaluations/bulk-action", teacherAToken,
			map[string]any{"action": "confirm", "evaluation_ids": []int64{1}})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		aff := result["affected"].(float64)
		fail := result["failed"].(float64)
		if aff != 1 || fail != 0 {
			t.Fatalf("expected affected=1 failed=0, got affected=%v failed=%v", aff, fail)
		}
	})

	t.Run("TEST-T0.2-06 parse result", func(t *testing.T) {
		resp := doRequest(t, app.Server, "GET", "/api/parse/1/result", teacherAToken, nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}
