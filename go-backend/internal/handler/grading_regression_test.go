package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/smartedu/training-eval-system/testutil"
)

// ============================================================
// T0.2: Grading workflow regression tests
// These verify the current behavior of grading endpoints.
// Do NOT change business logic — only fix if behavior regresses.
// ============================================================

// seedGradingFixture is a helper that builds the grading workflow fixture
// and returns it along with the test app.
func seedGradingFixture(t *testing.T) (*testutil.TestApp, *testutil.GradingFixture) {
	t.Helper()
	app := testutil.SetupTestApp(t)
	f, err := testutil.BuildGradingWorkflowFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildGradingWorkflowFixture: %v", err)
	}
	return app, f
}

func teacherAToken() string {
	return testutil.GenerateTestToken(2, "teacher_a", "teacher")
}

func teacherBToken() string {
	return testutil.GenerateTestToken(3, "teacher_b", "teacher")
}

// TEST-T0.2-01: Teacher fetches task A submissions
func TestGrading_GetSubmissions_TaskA(t *testing.T) {
	app, f := seedGradingFixture(t)
	resp := doRequest(t, app.Server, "GET", fmt.Sprintf("/api/grading/tasks/%d/submissions", f.TaskAID), teacherAToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)

	var subs []map[string]any
	testutil.DecodeJSON(t, resp, &subs)
	if len(subs) != 2 {
		t.Fatalf("expected 2 submissions for task A (student A + B), got %d", len(subs))
	}
}

// TEST-T0.2-02: Teacher fetches task A summary
func TestGrading_GetSummary_TaskA(t *testing.T) {
	app, f := seedGradingFixture(t)
	resp := doRequest(t, app.Server, "GET", fmt.Sprintf("/api/grading/tasks/%d/summary", f.TaskAID), teacherAToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)

	var summary map[string]any
	testutil.DecodeJSON(t, resp, &summary)

	// Should have summary fields
	if _, ok := summary["total_uploads"]; !ok {
		t.Fatal("expected total_uploads in summary")
	}
	if _, ok := summary["scored_count"]; !ok {
		t.Fatal("expected scored_count in summary")
	}
	if _, ok := summary["confirmed_count"]; !ok {
		t.Fatal("expected confirmed_count in summary")
	}
}

// TEST-T0.2-03: Confirm a scored evaluation
func TestGrading_Confirm_EvalA(t *testing.T) {
	app, f := seedGradingFixture(t)
	resp := doRequest(t, app.Server, "POST", fmt.Sprintf("/api/grading/evaluations/%d/confirm", f.EvalAID), teacherAToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// TEST-T0.2-04: Reject a scored evaluation with reason
func TestGrading_Reject_EvalA(t *testing.T) {
	app, f := seedGradingFixture(t)
	resp := doRequest(t, app.Server, "POST", fmt.Sprintf("/api/grading/evaluations/%d/reject", f.EvalAID), teacherAToken(), map[string]string{
		"reason": "报告内容不完整，缺少实验步骤和结论部分，需要补充",
	})
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// TEST-T0.2-05: Bulk confirm evaluations
func TestGrading_BulkConfirm(t *testing.T) {
	app, f := seedGradingFixture(t)
	resp := doRequest(t, app.Server, "POST", "/api/evaluations/bulk-action", teacherAToken(), map[string]any{
		"action":         "confirm",
		"evaluation_ids": []int64{f.EvalAID},
	})
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Affected int `json:"affected"`
		Failed   int `json:"failed"`
	}
	testutil.DecodeJSON(t, resp, &result)
	if result.Affected != 1 {
		t.Fatalf("expected 1 affected, got %d", result.Affected)
	}
	if result.Failed != 0 {
		t.Fatalf("expected 0 failed, got %d", result.Failed)
	}
}

// TEST-T0.2-06: Read parse result
func TestParse_GetResult_UploadA(t *testing.T) {
	app, f := seedGradingFixture(t)
	resp := doRequest(t, app.Server, "GET", fmt.Sprintf("/api/parse/%d/result", f.UploadAID), teacherAToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)

	var result struct {
		RawText string `json:"raw_text"`
	}
	testutil.DecodeJSON(t, resp, &result)
	if result.RawText == "" {
		t.Fatal("expected non-empty raw_text")
	}
}

// ============================================================
// Cross-teacher permission tests (baseline for Epic 1)
// ============================================================

// Teacher A should not be able to see Task B's submissions
func TestGrading_CrossTeacher_Forbidden(t *testing.T) {
	app, f := seedGradingFixture(t)
	// Teacher A tries to access Task B (owned by Teacher B)
	resp := doRequest(t, app.Server, "GET", fmt.Sprintf("/api/grading/tasks/%d/submissions", f.TaskBID), teacherAToken(), nil)
	defer resp.Body.Close()
	// Current behavior: may return 200 (no permission check yet). Record for regression.
	// In Epic 1 this should become 403.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 200 or 403, got %d", resp.StatusCode)
	}
}

func TestGrading_GetSummary_NumericTypes(t *testing.T) {
	app, f := seedGradingFixture(t)
	resp := doRequest(t, app.Server, "GET", fmt.Sprintf("/api/grading/tasks/%d/summary", f.TaskAID), teacherAToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)

	var raw map[string]any
	testutil.DecodeJSON(t, resp, &raw)

	// Verify numeric fields are actual numbers, not strings (common JSON bug)
	total, ok := raw["total_uploads"].(float64)
	if !ok {
		t.Fatalf("total_uploads is not a number, got %T: %v", raw["total_uploads"], raw["total_uploads"])
	}
	if total < 1 {
		t.Fatalf("expected at least 1 upload, got %v", total)
	}

	// Verify at least one scored upload exists
	scored, ok := raw["scored_count"].(float64)
	if !ok {
		t.Fatalf("scored_count is not a number, got %T: %v", raw["scored_count"], raw["scored_count"])
	}
	if scored < 1 {
		t.Fatalf("expected at least 1 scored evaluation, got %v", scored)
	}
}
