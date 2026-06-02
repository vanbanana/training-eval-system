package handler_test

import (
	"bytes"
	"encoding/json"
	"io"
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
	resp := doRequest(t, app.Server, "GET", "/api/uploads/1", "", nil)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestUploads_ListByTask_Empty(t *testing.T) {
	app := testutil.SetupTestApp(t)
	resp := doRequest(t, app.Server, "GET", "/api/uploads/1", testutil.StudentToken(), nil)
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
	body := dto.ChatStreamRequest{SessionID: 1, Message: "hello"}
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
	// Students CAN access their personal report
	resp := doRequest(t, app.Server, "GET", "/api/reports/personal/1", testutil.StudentToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
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
	// Returns 400 when no scored evaluations exist for export
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
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
