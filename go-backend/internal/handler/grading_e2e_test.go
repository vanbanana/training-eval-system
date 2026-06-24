package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/smartedu/training-eval-system/testutil"
)

// TestGradingE2E_GoldenPath validates the full grading flow (T7.1).
func TestGradingE2E_GoldenPath(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()

	f, err := testutil.BuildGradingWorkflowFixture(ctx, app.DB)
	if err != nil {
		t.Fatalf("BuildGradingWorkflowFixture: %v", err)
	}

	teacherToken := testutil.GenerateTestToken(f.TeacherAID, "teacher_a", "teacher")

	t.Run("get submissions", func(t *testing.T) {
		resp := doRequest(t, app.Server, "GET", "/api/grading/tasks/1/submissions", teacherToken, nil)
		defer resp.Body.Close()
		testutil.AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("get report-view for upload A", func(t *testing.T) {
		resp := doRequest(t, app.Server, "GET", "/api/grading/uploads/1/report-view", teacherToken, nil)
		defer resp.Body.Close()
		testutil.AssertStatus(t, resp, http.StatusOK)
		var rv map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&rv); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if rv["is_readable"] != true {
			t.Fatalf("upload A should be readable, got %v", rv["is_readable"])
		}
	})

	t.Run("override dimension score", func(t *testing.T) {
		resp := doRequest(t, app.Server, "PATCH", "/api/evaluations/1/dimensions/1", teacherToken,
			map[string]any{"teacher_score": 85.0})
		defer resp.Body.Close()
		testutil.AssertStatus(t, resp, http.StatusOK)
	})

	t.Run("confirm evaluation", func(t *testing.T) {
		resp := doRequest(t, app.Server, "POST", "/api/grading/evaluations/1/confirm", teacherToken, nil)
		defer resp.Body.Close()
		testutil.AssertStatus(t, resp, http.StatusOK)

		var status string
		app.DB.Reader.QueryRowContext(ctx, "SELECT status FROM evaluations WHERE id=1").Scan(&status)
		if status != "confirmed" {
			t.Fatalf("expected confirmed, got %s", status)
		}
	})
}
