package handler_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/smartedu/training-eval-system/testutil"
)

// Regression tests for the teacher dimension-override path
// (PATCH /api/evaluations/{id}/dimensions/{dimId}).
//
// New AI-first model: AI scores 100%, a teacher may override individual
// dimensions and the teacher score fully replaces the AI score. Overriding a
// dimension must NOT silently change the evaluation status (a confirmed
// evaluation stays confirmed) and a rejected evaluation is immutable.
func TestGradingOverride_DimensionScore(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()

	f, err := testutil.BuildGradingWorkflowFixture(ctx, app.DB)
	if err != nil {
		t.Fatalf("BuildGradingWorkflowFixture: %v", err)
	}

	teacherAToken := testutil.GenerateTestToken(f.TeacherAID, "teacher_a", "teacher")
	patch := func(dimID int64, subj float64) *http.Response {
		return doRequest(t, app.Server, "PATCH",
			"/api/evaluations/1/dimensions/"+strconv.FormatInt(dimID, 10), teacherAToken,
			map[string]any{"subj_score": subj, "comment": "调整"})
	}
	statusOf := func() string {
		var s string
		app.DB.Reader.QueryRowContext(ctx, "SELECT status FROM evaluations WHERE id=1").Scan(&s)
		return s
	}
	totalOf := func() float64 {
		var v float64
		app.DB.Reader.QueryRowContext(ctx, "SELECT total_score FROM evaluations WHERE id=1").Scan(&v)
		return v
	}

	t.Run("confirmed evaluation stays confirmed after override", func(t *testing.T) {
		app.DB.Writer.ExecContext(ctx, `UPDATE evaluations SET status='confirmed' WHERE id=1`)

		resp := patch(1, 100) // dim1 weight 40
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if got := statusOf(); got != "confirmed" {
			t.Fatalf("status must stay confirmed, got %s", got)
		}
		// 100*0.4 + 75*0.3 + 90*0.3 = 89.5
		if got := totalOf(); got != 89.5 {
			t.Fatalf("expected total 89.5, got %v", got)
		}
	})

	t.Run("rejected evaluation cannot be overridden", func(t *testing.T) {
		app.DB.Writer.ExecContext(ctx, `UPDATE evaluations SET status='rejected' WHERE id=1`)

		resp := patch(2, 50)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409 for rejected eval, got %d", resp.StatusCode)
		}
		if got := statusOf(); got != "rejected" {
			t.Fatalf("rejected eval must stay rejected, got %s", got)
		}
	})

	t.Run("scored evaluation stays scored after override", func(t *testing.T) {
		app.DB.Writer.ExecContext(ctx, `UPDATE evaluations SET status='scored' WHERE id=1`)

		resp := patch(2, 60) // dim2 weight 30; dim1 teacher 100 persists
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if got := statusOf(); got != "scored" {
			t.Fatalf("status must stay scored, got %s", got)
		}
		// 100*0.4 + 60*0.3 + 90*0.3 = 85.0
		if got := totalOf(); got != 85.0 {
			t.Fatalf("expected total 85.0, got %v", got)
		}
	})
}
