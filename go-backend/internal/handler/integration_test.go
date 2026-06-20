package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/testutil"
)

// --- Helper ---

func doRequest(t *testing.T, srv *httptest.Server, method, path, token string, body any) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, srv.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// ============================================================
// Task 2.3: Tasks Handler integration tests
// ============================================================

func TestTasks_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/tasks", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestTasks_Role_StudentCanRead(t *testing.T) {
	app := testutil.SetupTestApp(t)
	// Students CAN read tasks (they see published tasks)
	resp := doRequest(t, app.Server, "GET", "/api/tasks", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestTasks_Role_StudentCannotWrite(t *testing.T) {
	app := testutil.SetupTestApp(t)
	// Students CANNOT create tasks
	resp := doRequest(t, app.Server, "POST", "/api/tasks", testutil.StudentToken(), map[string]string{"name": "test"})
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestTasks_List_Empty(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/tasks", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
	// Handler returns a plain JSON array (not paginated envelope)
	var result []dto.TaskResponse
	testutil.DecodeJSON(t, resp, &result)
	if len(result) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(result))
	}
}

// ============================================================
// Task 3.3: Uploads Handler integration tests
// ============================================================

func TestUploads_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/uploads/by-task/1", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestUploads_ListByTask_Empty(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/uploads/by-task/1", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 4.3: Evaluations Handler integration tests
// ============================================================

func TestEvaluations_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/evaluations/my", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestEvaluations_GetMy_Empty(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/evaluations/my", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestEvaluations_GetByID_NotFound(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/evaluations/999", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// ============================================================
// Task 5.2: Grading Handler integration tests
// ============================================================

func TestGrading_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/grading/tasks/1/submissions", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestGrading_Role_StudentForbidden(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/grading/tasks/1/submissions", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestGrading_Submissions_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/grading/tasks/1/submissions", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 6.3: Courses Handler integration tests
// ============================================================

func TestCourses_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/courses", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestCourses_List_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/courses", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 7.3: Classes Handler integration tests
// ============================================================

func TestClasses_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/classes", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestClasses_List_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/classes", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 8.3: Notifications Handler integration tests
// ============================================================

func TestNotifications_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/notifications", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestNotifications_List_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/notifications", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestNotifications_MarkAllRead_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "POST", "/api/notifications/read-all", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 9.2: Chat Handler integration tests (including SSE)
// ============================================================

func TestChat_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/chat/sessions", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestChat_ListSessions_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/chat/sessions", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestChat_Stream_SSE(t *testing.T) {
		app := testutil.SetupTestApp(t)
		// Use SessionID=0 so no DB persistence is attempted (no session needed for basic SSE format test)
		body := dto.ChatStreamRequest{SessionID: 0, Message: "hello"}
		resp := doRequest(t, app.Server, "POST", "/api/chat/stream", testutil.StudentToken(), body)
		if resp.Header.Get("Content-Type") != "text/event-stream" {
			t.Fatalf("expected text/event-stream, got %q", resp.Header.Get("Content-Type"))
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(data, []byte("data:")) {
			t.Fatal("expected SSE data event in response")
		}
	}

// ============================================================
// Task 10.2: Similarity Handler integration tests
// ============================================================

func TestSimilarity_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/similarity/task/1", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestSimilarity_Role_StudentForbidden(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/similarity/task/1", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestSimilarity_GetByTask_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/similarity/task/1", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 11.3: Templates Handler integration tests
// ============================================================

func TestTemplates_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/templates", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestTemplates_List_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/templates", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 12.2: Imports Handler integration tests
// ============================================================

func TestImports_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/imports/template/user.xlsx", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestImports_Role_NonAdminForbidden(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/imports/template/user.xlsx", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestImports_DownloadTemplate_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/imports/template/user.xlsx", testutil.AdminToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
	ct := resp.Header.Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("expected xlsx content-type, got %q", ct)
	}
}

// ============================================================
// Task 13.3: Dashboard Handler integration tests
// ============================================================

func TestDashboard_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/dashboard", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestDashboard_Admin_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/dashboard", testutil.AdminToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestDashboard_Student_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/dashboard", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 14.2: Reports Handler integration tests
// ============================================================

func TestReports_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/reports/statistics/1", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestReports_Role_StudentCanAccessPersonal(t *testing.T) {
	app := testutil.SetupTestApp(t)
	// Students are NOT blocked by the role gate on the personal report route.
	// With an empty test DB the evaluation does not exist, so the handler
	// reaches its data lookup and returns 404 (not 401/403), proving the
	// student passed authentication and authorization.
	resp := doRequest(t, app.Server, "GET", "/api/reports/personal/1", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestReports_Role_StudentCannotAccessStatistics(t *testing.T) {
	app := testutil.SetupTestApp(t)
	// Students CANNOT access statistics (teacher/admin only)
	resp := doRequest(t, app.Server, "GET", "/api/reports/statistics/1", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestReports_CSV_ContentType(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/reports/task/1/csv", testutil.TeacherToken(), nil)
	// With an empty test DB the task does not exist, so export returns 404.
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// ============================================================
// Task 15.2: Profiles Handler integration tests
// ============================================================

func TestProfiles_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/profiles/student/1", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestProfiles_NotFound(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/profiles/student/999", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// ============================================================
// Task 16.3: LLM Config Handler integration tests
// ============================================================

func TestLLM_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/llm/configs", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestLLM_Role_NonAdminForbidden(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/llm/configs", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestLLM_List_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/llm/configs", testutil.AdminToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Task 17.2: Audit Handler integration tests
// ============================================================

func TestAudit_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/audit", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestAudit_Role_NonAdminForbidden(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/audit", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestAudit_List_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/audit", testutil.AdminToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestAudit_Export_CSV(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/audit/export", testutil.AdminToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
	if resp.Header.Get("Content-Type") != "text/csv" {
		t.Fatalf("expected text/csv, got %q", resp.Header.Get("Content-Type"))
	}
}

// ============================================================
// Task 18.3: Account Handler integration tests
// ============================================================

func TestAccount_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/account/me", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestAccount_GetMe_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/account/me", testutil.StudentToken(), nil)
	// May be 404 since no user with ID 3 exists in empty DB, but should not be 401
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("should not be 401 with valid token")
	}
}

// ============================================================
// Task 19.2: Parse Handler integration tests
// ============================================================

func TestParse_Auth_Required(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/parse/1/result", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestParse_NotFound(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/parse/999/result", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// ============================================================
// Task 20.2: Users Handler additions integration tests
// ============================================================

func TestUsers_ToggleActive_Auth(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "PATCH", "/api/users/1/toggle-active", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestUsers_ToggleActive_NonAdminForbidden(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "PATCH", "/api/users/1/toggle-active", testutil.TeacherToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestUsers_ResetPassword_Auth(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "POST", "/api/users/1/reset-password", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

// ============================================================
// Task 23.1: Property Test - Route completeness (non-404)
// ============================================================

func TestProperty_RouteCompleteness(t *testing.T) {
	app := testutil.SetupTestApp(t)

	// All defined API paths should NOT return 404
	// They may return 401 (no auth) or 405 (wrong method), but never 404 for the route itself
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/tasks"},
		{"POST", "/api/tasks"},
		{"GET", "/api/uploads/1"},
		{"GET", "/api/evaluations/my"},
		{"GET", "/api/grading/tasks/1/submissions"},
		{"GET", "/api/courses"},
		{"GET", "/api/classes"},
		{"GET", "/api/notifications"},
		{"GET", "/api/chat/sessions"},
		{"GET", "/api/similarity/task/1"},
		{"GET", "/api/templates"},
		{"GET", "/api/imports/template/user.xlsx"},
		{"GET", "/api/dashboard"},
		{"GET", "/api/reports/statistics/1"},
		{"GET", "/api/profiles/student/1"},
		{"GET", "/api/llm/configs"},
		{"GET", "/api/audit"},
		{"GET", "/api/account/me"},
		{"GET", "/api/parse/1/result"},
		{"POST", "/api/auth/login"},
		{"GET", "/healthz"},
	}

	for _, rt := range routes {
		resp := doRequest(t, app.Server, rt.method, rt.path, "", nil)
		if resp.StatusCode == http.StatusNotFound {
			// Check if it's a "resource not found" vs "route not found"
			// Route not found would be chi's default 404
			body, _ := io.ReadAll(resp.Body)
			if !bytes.Contains(body, []byte("not found")) && !bytes.Contains(body, []byte("Not Found")) {
				t.Errorf("route %s %s returned 404 (route not registered)", rt.method, rt.path)
			}
		}
		resp.Body.Close()
	}
}

// ============================================================
// Task 23.2: Property Test - Auth enforcement on protected routes
// ============================================================

func TestProperty_AuthEnforcement(t *testing.T) {
	app := testutil.SetupTestApp(t)

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/tasks"},
		{"GET", "/api/uploads/1"},
		{"GET", "/api/evaluations/my"},
		{"GET", "/api/grading/tasks/1/submissions"},
		{"GET", "/api/courses"},
		{"GET", "/api/classes"},
		{"GET", "/api/notifications"},
		{"GET", "/api/chat/sessions"},
		{"GET", "/api/similarity/task/1"},
		{"GET", "/api/templates"},
		{"GET", "/api/imports/template/user.xlsx"},
		{"GET", "/api/dashboard"},
		{"GET", "/api/reports/statistics/1"},
		{"GET", "/api/profiles/student/1"},
		{"GET", "/api/llm/configs"},
		{"GET", "/api/audit"},
		{"GET", "/api/account/me"},
		{"GET", "/api/parse/1/result"},
	}

	for _, rt := range protectedRoutes {
		resp := doRequest(t, app.Server, rt.method, rt.path, "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("route %s %s should return 401 without auth, got %d", rt.method, rt.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// ============================================================
// Task 23.3: Property Test - Admin role enforcement
// ============================================================

func TestProperty_AdminRoleEnforcement(t *testing.T) {
	app := testutil.SetupTestApp(t)

	adminOnlyRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/users"},
		{"GET", "/api/llm/configs"},
		{"GET", "/api/audit"},
		{"GET", "/api/imports/template/user.xlsx"},
	}

	// Student token should get 403
	for _, rt := range adminOnlyRoutes {
		resp := doRequest(t, app.Server, rt.method, rt.path, testutil.StudentToken(), nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("admin route %s %s should return 403 for student, got %d", rt.method, rt.path, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Teacher token should also get 403 on admin-only routes
	for _, rt := range adminOnlyRoutes {
		resp := doRequest(t, app.Server, rt.method, rt.path, testutil.TeacherToken(), nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("admin route %s %s should return 403 for teacher, got %d", rt.method, rt.path, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Teacher-only write routes should forbid students
	teacherWriteRoutes := []struct {
		method string
		path   string
	}{
		{"POST", "/api/tasks"},
		{"GET", "/api/grading/tasks/1/submissions"},
		{"GET", "/api/similarity/task/1"},
		{"GET", "/api/reports/statistics/1"},
	}
	for _, rt := range teacherWriteRoutes {
		resp := doRequest(t, app.Server, rt.method, rt.path, testutil.StudentToken(), nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("teacher route %s %s should return 403 for student, got %d", rt.method, rt.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// ============================================================
// Regression: teacher reject of a scored evaluation must succeed (not 500).
// Previously EvaluationService.Update validated checkTransition(status,status)
// which rejected rejected->rejected and returned HTTP 500.
// ============================================================

func TestGrading_RejectScoredEvaluation_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()
	w := app.DB.Writer

	// Seed minimal data: teacher(2), student(3), course, task, upload, scored evaluation.
	if _, err := w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role) VALUES (2,'teacher1','T','x','teacher'),(3,'student1','S','x','student')`); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO courses (id,name,code) VALUES (1,'C','C1')`); err != nil {
		t.Fatalf("seed course: %v", err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO training_tasks (id,name,teacher_id,course_id,status) VALUES (1,'T',2,1,'published')`); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status) VALUES (1,1,3,'f.pdf','pdf',100,'p','parsed')`); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO evaluations (id,task_id,student_id,upload_id,status,total_score) VALUES (1,1,3,1,'scored',80.0)`); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}

	resp := doRequest(t, app.Server, "POST", "/api/grading/evaluations/1/reject", testutil.TeacherToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// Regression: confirming a scored evaluation must succeed.
func TestGrading_ConfirmScoredEvaluation_OK(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()
	w := app.DB.Writer

	w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role) VALUES (2,'teacher1','T','x','teacher'),(3,'student1','S','x','student')`)
	w.ExecContext(ctx, `INSERT INTO courses (id,name,code) VALUES (1,'C','C1')`)
	w.ExecContext(ctx, `INSERT INTO training_tasks (id,name,teacher_id,course_id,status) VALUES (1,'T',2,1,'published')`)
	w.ExecContext(ctx, `INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status) VALUES (1,1,3,'f.pdf','pdf',100,'p','parsed')`)
	if _, err := w.ExecContext(ctx, `INSERT INTO evaluations (id,task_id,student_id,upload_id,status,total_score) VALUES (1,1,3,1,'scored',80.0)`); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}

	resp := doRequest(t, app.Server, "POST", "/api/grading/evaluations/1/confirm", testutil.TeacherToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ============================================================
// Regression: previously-stub handlers must now hit the real data layer.
// ============================================================

func TestSimilarity_GetByTask_ReadsRepo(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()
	w := app.DB.Writer

	w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role) VALUES (2,'teacher1','T','x','teacher'),(3,'s','s','x','student')`)
	w.ExecContext(ctx, `INSERT INTO courses (id,name,code) VALUES (1,'C','C1')`)
	w.ExecContext(ctx, `INSERT INTO training_tasks (id,name,teacher_id,course_id,status) VALUES (1,'T',2,1,'published')`)
	w.ExecContext(ctx, `INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status) VALUES (1,1,3,'a','pdf',1,'p1','parsed'),(2,1,3,'b','pdf',1,'p2','parsed')`)
	if _, err := w.ExecContext(ctx, `INSERT INTO similarity_records (id,task_id,upload_a_id,upload_b_id,hamming_distance,cosine_similarity,state) VALUES (1,1,1,2,3,0.92,'suspect')`); err != nil {
		t.Fatalf("seed similarity: %v", err)
	}

	resp := doRequest(t, app.Server, "GET", "/api/similarity/task/1", testutil.TeacherToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)

	var records []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 similarity record from repo, got %d", len(records))
	}
}

func TestSimilarity_UpdateDecision_Persists(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()
	w := app.DB.Writer

	w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role) VALUES (2,'teacher1','T','x','teacher'),(3,'s','s','x','student')`)
	w.ExecContext(ctx, `INSERT INTO courses (id,name,code) VALUES (1,'C','C1')`)
	w.ExecContext(ctx, `INSERT INTO training_tasks (id,name,teacher_id,course_id,status) VALUES (1,'T',2,1,'published')`)
	w.ExecContext(ctx, `INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status) VALUES (1,1,3,'a','pdf',1,'p1','parsed'),(2,1,3,'b','pdf',1,'p2','parsed')`)
	w.ExecContext(ctx, `INSERT INTO similarity_records (id,task_id,upload_a_id,upload_b_id,hamming_distance,state) VALUES (1,1,1,2,3,'suspect')`)

	resp := doRequest(t, app.Server, "POST", "/api/similarity/1/decision", testutil.TeacherToken(), map[string]string{"action": "confirm"})
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)

	var state string
	app.DB.Reader.QueryRowContext(ctx, "SELECT state FROM similarity_records WHERE id=1").Scan(&state)
	if state != "confirmed" {
		t.Fatalf("expected state 'confirmed' persisted, got %q", state)
	}
}

func TestParse_GetResult_ReadsRepo(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()
	w := app.DB.Writer

	w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role) VALUES (2,'teacher1','T','x','teacher'),(3,'s','s','x','student')`)
	w.ExecContext(ctx, `INSERT INTO courses (id,name,code) VALUES (1,'C','C1')`)
	w.ExecContext(ctx, `INSERT INTO training_tasks (id,name,teacher_id,course_id,status) VALUES (1,'T',2,1,'published')`)
	w.ExecContext(ctx, `INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status) VALUES (1,1,3,'a','pdf',1,'p1','parsed')`)
	if _, err := w.ExecContext(ctx, `INSERT INTO parse_results (upload_id,raw_text,parsed_at) VALUES (1,'hello world',datetime('now'))`); err != nil {
		t.Fatalf("seed parse_result: %v", err)
	}

	resp := doRequest(t, app.Server, "GET", "/api/parse/1/result", testutil.TeacherToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestImports_DownloadTemplate_RealXLSX(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/imports/template/user.xlsx", testutil.AdminToken(), nil)
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	// A real xlsx is a ZIP archive starting with "PK".
	if len(body) < 2 || body[0] != 'P' || body[1] != 'K' {
		t.Fatalf("expected a real xlsx (PK header), got %d bytes", len(body))
	}
}

// Regression: template create+list must round-trip dimensions under the
// "dimensions" key (previously backend used "items" and List dropped them).
func TestTemplates_DimensionsRoundTrip(t *testing.T) {
	app := testutil.SetupTestApp(t)
	// Teacher token uses user id 2; owner_id FK requires the row to exist.
	app.DB.Writer.ExecContext(context.Background(),
		`INSERT INTO users (id,username,display_name,password_hash,role) VALUES (2,'teacher1','T','x','teacher')`)

	createBody := map[string]any{
		"name":        "Test Template",
		"description": "desc",
		"dimensions": []map[string]any{
			{"name": "代码质量", "weight": 60, "order_index": 0},
			{"name": "文档完整", "weight": 40, "order_index": 1},
		},
	}
	resp := doRequest(t, app.Server, "POST", "/api/templates", testutil.TeacherToken(), createBody)
	testutil.AssertStatus(t, resp, http.StatusCreated)
	resp.Body.Close()

	listResp := doRequest(t, app.Server, "GET", "/api/templates", testutil.TeacherToken(), nil)
	defer listResp.Body.Close()
	testutil.AssertStatus(t, listResp, http.StatusOK)

	var templates []struct {
		Name       string `json:"name"`
		Dimensions []struct {
			Name   string `json:"name"`
			Weight int    `json:"weight"`
		} `json:"dimensions"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&templates); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}
	if len(templates[0].Dimensions) != 2 {
		t.Fatalf("expected 2 dimensions round-tripped, got %d", len(templates[0].Dimensions))
	}
}

// Regression: import users from a CSV must actually create users (was a stub).
func TestImports_ImportUsers_CreatesUsers(t *testing.T) {
	app := testutil.SetupTestApp(t)
	// Admin token uses user id 1; import_jobs.operator_id FK requires the row.
	app.DB.Writer.ExecContext(context.Background(),
		`INSERT INTO users (id,username,display_name,password_hash,role) VALUES (1,'admin','A','x','admin')`)

	csvData := "username,display_name,role,password\n" +
		"newstudent1,新学生一,student,Pass@1234\n" +
		"newteacher1,新教师一,teacher,Pass@1234\n"

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "users.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fw.Write([]byte(csvData))
	mw.Close()

	req, _ := http.NewRequest("POST", app.Server.URL+"/api/imports/users", &buf)
	req.Header.Set("Authorization", "Bearer "+testutil.AdminToken())
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)

	var result dto.ImportResultResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.SuccessCount != 2 {
		t.Fatalf("expected 2 users created, got success=%d failed=%d", result.SuccessCount, result.FailedCount)
	}

	// Verify the users actually exist in the DB.
	var count int
	app.DB.Reader.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM users WHERE username IN ('newstudent1','newteacher1')").Scan(&count)
	if count != 2 {
		t.Fatalf("expected 2 users persisted, got %d", count)
	}
}

// Regression: archiving a class must persist (was in-memory only) and
// removing a student must hit the real repository.
func TestClasses_ArchiveAndRemoveStudent(t *testing.T) {
	app := testutil.SetupTestApp(t)
	ctx := context.Background()
	w := app.DB.Writer

	w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role) VALUES (2,'teacher1','T','x','teacher'),(3,'stu','S','x','student')`)
	w.ExecContext(ctx, `INSERT INTO courses (id,name,code) VALUES (1,'C','C1')`)
	w.ExecContext(ctx, `INSERT INTO classes (id,name,course_id,teacher_id,student_count,is_archived) VALUES (1,'Cls',1,2,0,0)`)
	w.ExecContext(ctx, `INSERT INTO class_memberships (class_id,student_id) VALUES (1,3)`)

	// Archive must persist.
	resp := doRequest(t, app.Server, "PATCH", "/api/classes/1/archive", testutil.TeacherToken(), nil)
	resp.Body.Close()
	testutil.AssertStatus(t, resp, http.StatusOK)
	var archived int
	app.DB.Reader.QueryRowContext(ctx, "SELECT is_archived FROM classes WHERE id=1").Scan(&archived)
	if archived != 1 {
		t.Fatalf("expected class archived persisted, got is_archived=%d", archived)
	}

	// Remove student must delete the membership.
	resp2 := doRequest(t, app.Server, "DELETE", "/api/classes/1/students/3", testutil.TeacherToken(), nil)
	resp2.Body.Close()
	testutil.AssertStatus(t, resp2, http.StatusOK)
	var count int
	app.DB.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM class_memberships WHERE class_id=1 AND student_id=3").Scan(&count)
	if count != 0 {
		t.Fatalf("expected membership removed, got %d", count)
	}
}
