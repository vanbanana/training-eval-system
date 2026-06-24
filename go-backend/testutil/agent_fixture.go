package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/smartedu/training-eval-system/internal/store"
)

// AgentFixture holds IDs for the three-role test dataset.
type AgentFixture struct {
	AdminAID   int64 // 10
	TeacherAID int64 // 11
	TeacherBID int64 // 12
	StudentAID int64 // 13
	StudentBID int64 // 14

	CourseID int64 // 200
	ClassAID int64 // 200
	ClassBID int64 // 201

	TaskAID int64 // 200
	TaskBID int64 // 201
	TaskCID int64 // 300 (second task owned by TeacherA)

	UploadAID int64 // 200 (studentA → taskA)
	UploadBID int64 // 201 (studentB → taskB)

	EvalAID int64 // 200
	EvalBID int64 // 201

	DimIDs [4]int64 // 200,201 (taskA), 202,203 (taskB)
}

// AdminAToken returns a JWT for admin A (id=10).
func AdminAToken() string { return GenerateTestToken(10, "admin_a", "admin") }

// TeacherAToken returns a JWT for teacher A (id=11).
func TeacherAToken() string { return GenerateTestToken(11, "teacher_a", "teacher") }

// TeacherBToken returns a JWT for teacher B (id=12).
func TeacherBToken() string { return GenerateTestToken(12, "teacher_b", "teacher") }

// StudentAToken returns a JWT for student A (id=13).
func StudentAToken() string { return GenerateTestToken(13, "student_a", "student") }

// StudentBToken returns a JWT for student B (id=14).
func StudentBToken() string { return GenerateTestToken(14, "student_b", "student") }

// QueryInt64 is a helper to query a single int64 scalar.
func QueryInt64(db *store.DB, query string, args ...any) (int64, error) {
	var val sql.NullInt64
	err := db.Reader.QueryRow(query, args...).Scan(&val)
	if err != nil {
		return 0, err
	}
	if !val.Valid {
		return 0, fmt.Errorf("null result")
	}
	return val.Int64, nil
}

// BuildAgentFixture seeds a three-role dataset for agent API tests.
// Returns the fixture with all IDs populated.
func BuildAgentFixture(ctx context.Context, db *store.DB) (*AgentFixture, error) {
	f := &AgentFixture{}
	w := db.Writer
	now := time.Now()
	deadline := now.Add(7 * 24 * time.Hour)

	// ── Users ──────────────────────────────────────────────────────────────
	users := []struct {
		id   int64
		name string
		role string
	}{
		{10, "admin_a", "admin"},
		{11, "teacher_a", "teacher"},
		{12, "teacher_b", "teacher"},
		{13, "student_a", "student"},
		{14, "student_b", "student"},
	}
	for _, u := range users {
		if _, err := w.ExecContext(ctx,
			`INSERT INTO users (id,username,display_name,password_hash,role,is_active)
			 VALUES (?,?,'fixture-user','x',?,1)`, u.id, u.name, u.role); err != nil {
			return nil, fmt.Errorf("seed user %s: %w", u.name, err)
		}
	}
	f.AdminAID = 10
	f.TeacherAID = 11
	f.TeacherBID = 12
	f.StudentAID = 13
	f.StudentBID = 14

	// ── Course ─────────────────────────────────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO courses (id,name,code,is_archived) VALUES (200,'Agent Course','AC',0)`); err != nil {
		return nil, fmt.Errorf("seed course: %w", err)
	}
	f.CourseID = 200

	// ── Classes ────────────────────────────────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO classes (id,name,course_id,teacher_id,student_count,is_archived)
		 VALUES (200,'Class A',200,11,0,0)`); err != nil {
		return nil, fmt.Errorf("seed class A: %w", err)
	}
	f.ClassAID = 200

	if _, err := w.ExecContext(ctx,
		`INSERT INTO classes (id,name,course_id,teacher_id,student_count,is_archived)
		 VALUES (201,'Class B',200,12,0,0)`); err != nil {
		return nil, fmt.Errorf("seed class B: %w", err)
	}
	f.ClassBID = 201

	// ── Class memberships ──────────────────────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO class_memberships (class_id,student_id) VALUES (200,13)`); err != nil {
		return nil, fmt.Errorf("seed membership studentA→classA: %w", err)
	}
	if _, err := w.ExecContext(ctx,
		`INSERT INTO class_memberships (class_id,student_id) VALUES (201,14)`); err != nil {
		return nil, fmt.Errorf("seed membership studentB→classB: %w", err)
	}

	// ── Tasks ──────────────────────────────────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO training_tasks (id,name,description,requirements,teacher_id,course_id,status,deadline)
		 VALUES (200,'Agent Task A','Task for class A','Requirements A',11,200,'published',?)`, deadline); err != nil {
		return nil, fmt.Errorf("seed task A: %w", err)
	}
	f.TaskAID = 200

	if _, err := w.ExecContext(ctx,
		`INSERT INTO training_tasks (id,name,description,requirements,teacher_id,course_id,status,deadline)
		 VALUES (201,'Agent Task B','Task for class B','Requirements B',12,200,'published',?)`, deadline); err != nil {
		return nil, fmt.Errorf("seed task B: %w", err)
	}
	f.TaskBID = 201

	if _, err := w.ExecContext(ctx,
		`INSERT INTO training_tasks (id,name,description,requirements,teacher_id,course_id,status,deadline)
		 VALUES (300,'Agent Task C','Second task for TeacherA','Requirements C',11,200,'published',?)`, deadline); err != nil {
		return nil, fmt.Errorf("seed task C: %w", err)
	}
	f.TaskCID = 300

	// ── Task classes ───────────────────────────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO task_classes (task_id,class_id) VALUES (200,200)`); err != nil {
		return nil, fmt.Errorf("seed task_class A: %w", err)
	}
	if _, err := w.ExecContext(ctx,
		`INSERT INTO task_classes (task_id,class_id) VALUES (201,201)`); err != nil {
		return nil, fmt.Errorf("seed task_class B: %w", err)
	}

	// ── Dimensions ─────────────────────────────────────────────────────────
	dims := []struct {
		id, taskID  int64
		name        string
		weight, ord int
	}{
		{200, 200, "Code Quality", 50, 0},
		{201, 200, "Documentation", 50, 1},
		{202, 201, "Correctness", 60, 0},
		{203, 201, "Style", 40, 1},
	}
	for _, d := range dims {
		if _, err := w.ExecContext(ctx,
			`INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (?,?,?,?,?)`,
			d.id, d.taskID, d.name, d.weight, d.ord); err != nil {
			return nil, fmt.Errorf("seed dim %d: %w", d.id, err)
		}
	}
	f.DimIDs = [4]int64{200, 201, 202, 203}

	// ── Uploads ────────────────────────────────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status,created_at)
		 VALUES (200,200,13,'report_a.pdf','pdf',1024,'/tmp/uploads/ra.pdf','parsed',?)`, now); err != nil {
		return nil, fmt.Errorf("seed upload A: %w", err)
	}
	f.UploadAID = 200

	if _, err := w.ExecContext(ctx,
		`INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status,created_at)
		 VALUES (201,201,14,'report_b.pdf','pdf',2048,'/tmp/uploads/rb.pdf','parsed',?)`, now); err != nil {
		return nil, fmt.Errorf("seed upload B: %w", err)
	}
	f.UploadBID = 201

	// ── Parse results ──────────────────────────────────────────────────────
	rawText := "Experiment: Go basics.\nSteps: 1. Install Go 2. Write hello world 3. Run tests\nConclusion: Success."
	if _, err := w.ExecContext(ctx,
		`INSERT INTO parse_results (upload_id,raw_text,parsed_at) VALUES (200,?,?)`, rawText, now); err != nil {
		return nil, fmt.Errorf("seed parse A: %w", err)
	}
	if _, err := w.ExecContext(ctx,
		`INSERT INTO parse_results (upload_id,raw_text,parsed_at) VALUES (201,?,?)`, rawText, now); err != nil {
		return nil, fmt.Errorf("seed parse B: %w", err)
	}

	// ── Evaluations ────────────────────────────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO evaluations (id,task_id,student_id,upload_id,status,total_score,created_at,updated_at)
		 VALUES (200,200,13,200,'scored',85.0,?,?)`, now, now); err != nil {
		return nil, fmt.Errorf("seed eval A: %w", err)
	}
	f.EvalAID = 200

	if _, err := w.ExecContext(ctx,
		`INSERT INTO evaluations (id,task_id,student_id,upload_id,status,total_score,created_at,updated_at)
		 VALUES (201,201,14,201,'scored',78.0,?,?)`, now, now); err != nil {
		return nil, fmt.Errorf("seed eval B: %w", err)
	}
	f.EvalBID = 201

	// ── Dimension scores ───────────────────────────────────────────────────
	scores := []struct {
		evalID, dimID int64
		aiScore       float64
	}{
		{200, 200, 80}, {200, 201, 90},
		{201, 202, 75}, {201, 203, 82},
	}
	for _, s := range scores {
		if _, err := w.ExecContext(ctx,
			`INSERT INTO dimension_scores (evaluation_id,dimension_id,ai_score,teacher_score)
			 VALUES (?,?,?,NULL)`, s.evalID, s.dimID, s.aiScore); err != nil {
			return nil, fmt.Errorf("seed dim_score eval=%d dim=%d: %w", s.evalID, s.dimID, err)
		}
	}

	// ── Student profiles ───────────────────────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO student_profiles (student_id,radar_data,source_evaluation_count,computed_at)
		 VALUES (13,'{}',1,?)`, now); err != nil {
		return nil, fmt.Errorf("seed profile studentA: %w", err)
	}
	if _, err := w.ExecContext(ctx,
		`INSERT INTO student_profiles (student_id,radar_data,source_evaluation_count,computed_at)
		 VALUES (14,'{}',1,?)`, now); err != nil {
		return nil, fmt.Errorf("seed profile studentB: %w", err)
	}

	// ── Chat sessions (legacy, for context) ────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO chat_sessions (id,student_id,title,created_at) VALUES (200,13,'Student A chat',?)`, now); err != nil {
		return nil, fmt.Errorf("seed chat session A: %w", err)
	}
	if _, err := w.ExecContext(ctx,
		`INSERT INTO chat_sessions (id,student_id,title,created_at) VALUES (201,14,'Student B chat',?)`, now); err != nil {
		return nil, fmt.Errorf("seed chat session B: %w", err)
	}

	// ── LLM configs (mock, no real keys) ───────────────────────────────────
	if _, err := w.ExecContext(ctx,
		`INSERT INTO llm_configs (id,provider,base_url,api_key_encrypted,chat_model,embed_model,is_active,created_at)
		 VALUES (200,'openai','http://localhost:19999','ZW5jcnlwdGVkLWtleQ==','mock-model','',1,?)`, now); err != nil {
		return nil, fmt.Errorf("seed llm_config: %w", err)
	}

	return f, nil
}
