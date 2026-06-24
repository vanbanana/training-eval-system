package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/smartedu/training-eval-system/internal/store"
)

// GradingFixture holds the IDs and metadata of all entities created by
// BuildGradingWorkflowFixture so tests can reference them without raw IDs.
type GradingFixture struct {
	AdminID    int64
	TeacherAID int64
	TeacherBID int64
	StudentAID int64
	StudentBID int64
	StudentCID int64

	CourseAID int64
	CourseBID int64

	ClassA1ID int64
	ClassA2ID int64
	ClassB1ID int64

	TaskAID int64
	TaskBID int64

	// UploadA: student A → task A, parsed + scored evaluation
	UploadAID      int64
	EvalAID        int64
	EvalADimScores []DimScoreFixture

	// UploadB: student B → task A, parsed + no evaluation
	UploadBID int64

	// UploadC: student C → task B, parsed + scored evaluation
	UploadCID      int64
	EvalCID        int64
	EvalCDimScores []DimScoreFixture

	// Normal and garbled parse results
	NormalRawText  string
	GarbledRawText string

	// Task dimensions (shared across all tasks in this fixture)
	TaskADimensions []DimFixture
	TaskBDimensions []DimFixture
}

// DimFixture describes a task dimension.
type DimFixture struct {
	ID     int64
	Name   string
	Weight int
}

// DimScoreFixture describes a single dimension score row.
type DimScoreFixture struct {
	DimensionID  int64
	AIScore      float64
	TeacherScore *float64 // nil = not overridden
}

// BuildGradingWorkflowFixture seeds a comprehensive grading workflow dataset
// into the given DB writer. It returns a GradingFixture with all IDs populated.
//
// Entities created:
//
//	Admin A, Teacher A, Teacher B, Student A/B/C
//	Course A, Course B
//	Class A1 (course A, teacher A), Class A2 (course A, teacher A)
//	Class B1 (course B, teacher B)
//	Task A (course A → teacher A → classes A1, A2)
//	Task B (course B → teacher B → class B1)
//	Upload A (student A → task A, parsed + scored evaluation)
//	Upload B (student B → task A, parsed + no evaluation)
//	Upload C (student C → task B, parsed + scored evaluation)
//	Parse results: normal text (upload A/C), garbled text (upload B)
func BuildGradingWorkflowFixture(ctx context.Context, db *store.DB) (*GradingFixture, error) {
	f := &GradingFixture{
		NormalRawText:  "实验目的：掌握Go语言基础。\n实验步骤：\n1. 安装Go环境\n2. 编写Hello World\n3. 运行测试\n实验结论：成功掌握了Go语言的基本语法和工具链。",
		GarbledRawText: "����\x00\x01\x02\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00",
	}
	w := db.Writer
	now := time.Now()

	// ── Users ──────────────────────────────────────────────────────────────

	if _, err := w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (1,'admin','管理员','x','admin',1)`); err != nil {
		return nil, fmt.Errorf("seed admin: %w", err)
	}
	f.AdminID = 1

	if _, err := w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (2,'teacher_a','教师A','x','teacher',1)`); err != nil {
		return nil, fmt.Errorf("seed teacher_a: %w", err)
	}
	f.TeacherAID = 2

	if _, err := w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (3,'teacher_b','教师B','x','teacher',1)`); err != nil {
		return nil, fmt.Errorf("seed teacher_b: %w", err)
	}
	f.TeacherBID = 3

	if _, err := w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (4,'student_a','学生A','x','student',1)`); err != nil {
		return nil, fmt.Errorf("seed student_a: %w", err)
	}
	f.StudentAID = 4

	if _, err := w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (5,'student_b','学生B','x','student',1)`); err != nil {
		return nil, fmt.Errorf("seed student_b: %w", err)
	}
	f.StudentBID = 5

	if _, err := w.ExecContext(ctx, `INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (6,'student_c','学生C','x','student',1)`); err != nil {
		return nil, fmt.Errorf("seed student_c: %w", err)
	}
	f.StudentCID = 6

	// ── Courses ────────────────────────────────────────────────────────────

	if _, err := w.ExecContext(ctx, `INSERT INTO courses (id,name,code,is_archived) VALUES (1,'软件工程','SE',0)`); err != nil {
		return nil, fmt.Errorf("seed course_a: %w", err)
	}
	f.CourseAID = 1

	if _, err := w.ExecContext(ctx, `INSERT INTO courses (id,name,code,is_archived) VALUES (2,'数据结构','DS',0)`); err != nil {
		return nil, fmt.Errorf("seed course_b: %w", err)
	}
	f.CourseBID = 2

	// ── Classes ────────────────────────────────────────────────────────────

	if _, err := w.ExecContext(ctx, `INSERT INTO classes (id,name,course_id,teacher_id,student_count,is_archived) VALUES (1,'软件一班',1,2,0,0)`); err != nil {
		return nil, fmt.Errorf("seed class_a1: %w", err)
	}
	f.ClassA1ID = 1

	if _, err := w.ExecContext(ctx, `INSERT INTO classes (id,name,course_id,teacher_id,student_count,is_archived) VALUES (2,'软件二班',1,2,0,0)`); err != nil {
		return nil, fmt.Errorf("seed class_a2: %w", err)
	}
	f.ClassA2ID = 2

	if _, err := w.ExecContext(ctx, `INSERT INTO classes (id,name,course_id,teacher_id,student_count,is_archived) VALUES (3,'数据一班',2,3,0,0)`); err != nil {
		return nil, fmt.Errorf("seed class_b1: %w", err)
	}
	f.ClassB1ID = 3

	// ── Tasks ──────────────────────────────────────────────────────────────

	deadline := now.Add(7 * 24 * time.Hour)
	if _, err := w.ExecContext(ctx,
		`INSERT INTO training_tasks (id,name,description,requirements,teacher_id,course_id,status,deadline)
		 VALUES (1,'实训一-go基础','Go语言基础实训','完成一个Go语言命令行工具',2,1,'published',?)`, deadline); err != nil {
		return nil, fmt.Errorf("seed task_a: %w", err)
	}
	f.TaskAID = 1

	if _, err := w.ExecContext(ctx,
		`INSERT INTO training_tasks (id,name,description,requirements,teacher_id,course_id,status,deadline)
		 VALUES (2,'实训二-数据结构','数据结构实训','实现一个二叉树',3,2,'published',?)`, deadline); err != nil {
		return nil, fmt.Errorf("seed task_b: %w", err)
	}
	f.TaskBID = 2

	// ── Task dimensions ────────────────────────────────────────────────────

	// Task A dims: 代码质量(40), 文档完整(30), 功能完整(30)
	if _, err := w.ExecContext(ctx, `INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (1,1,'代码质量',40,0)`); err != nil {
		return nil, fmt.Errorf("seed dim_a1: %w", err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (2,1,'文档完整',30,1)`); err != nil {
		return nil, fmt.Errorf("seed dim_a2: %w", err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (3,1,'功能完整',30,2)`); err != nil {
		return nil, fmt.Errorf("seed dim_a3: %w", err)
	}
	f.TaskADimensions = []DimFixture{
		{ID: 1, Name: "代码质量", Weight: 40},
		{ID: 2, Name: "文档完整", Weight: 30},
		{ID: 3, Name: "功能完整", Weight: 30},
	}

	// Task B dims: 逻辑正确(60), 代码规范(40)
	if _, err := w.ExecContext(ctx, `INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (4,2,'逻辑正确',60,0)`); err != nil {
		return nil, fmt.Errorf("seed dim_b1: %w", err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (5,2,'代码规范',40,1)`); err != nil {
		return nil, fmt.Errorf("seed dim_b2: %w", err)
	}
	f.TaskBDimensions = []DimFixture{
		{ID: 4, Name: "逻辑正确", Weight: 60},
		{ID: 5, Name: "代码规范", Weight: 40},
	}

	// ── task_classes ───────────────────────────────────────────────────────

	if _, err := w.ExecContext(ctx, `INSERT INTO task_classes (task_id,class_id) VALUES (1,1),(1,2)`); err != nil {
		return nil, fmt.Errorf("seed task_a classes: %w", err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO task_classes (task_id,class_id) VALUES (2,3)`); err != nil {
		return nil, fmt.Errorf("seed task_b classes: %w", err)
	}

	// ── Uploads ────────────────────────────────────────────────────────────

	if _, err := w.ExecContext(ctx,
		`INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status,created_at)
		 VALUES (1,1,4,'report_a.pdf','pdf',1024,'/tmp/uploads/report_a.pdf','parsed',?)`, now); err != nil {
		return nil, fmt.Errorf("seed upload_a: %w", err)
	}
	f.UploadAID = 1

	if _, err := w.ExecContext(ctx,
		`INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status,created_at)
		 VALUES (2,1,5,'report_b.pdf','pdf',2048,'/tmp/uploads/report_b.pdf','parsed',?)`, now); err != nil {
		return nil, fmt.Errorf("seed upload_b: %w", err)
	}
	f.UploadBID = 2

	if _, err := w.ExecContext(ctx,
		`INSERT INTO uploads (id,task_id,student_id,filename,file_type,file_size,storage_path,parse_status,created_at)
		 VALUES (3,2,6,'report_c.pdf','pdf',1536,'/tmp/uploads/report_c.pdf','parsed',?)`, now); err != nil {
		return nil, fmt.Errorf("seed upload_c: %w", err)
	}
	f.UploadCID = 3

	// ── Parse results ─────────────────────────────────────────────────────

	// Normal raw text for upload A
	if _, err := w.ExecContext(ctx,
		`INSERT INTO parse_results (upload_id,raw_text,parsed_at) VALUES (1,?,?)`,
		f.NormalRawText, now); err != nil {
		return nil, fmt.Errorf("seed parse_a: %w", err)
	}

	// Garbled raw text for upload B (tests report degradation)
	if _, err := w.ExecContext(ctx,
		`INSERT INTO parse_results (upload_id,raw_text,parsed_at) VALUES (2,?,?)`,
		f.GarbledRawText, now); err != nil {
		return nil, fmt.Errorf("seed parse_b: %w", err)
	}

	// Normal raw text for upload C
	if _, err := w.ExecContext(ctx,
		`INSERT INTO parse_results (upload_id,raw_text,parsed_at) VALUES (3,?,?)`,
		f.NormalRawText, now); err != nil {
		return nil, fmt.Errorf("seed parse_c: %w", err)
	}

	// ── Evaluations ────────────────────────────────────────────────────────

	// Eval A: student A → task A, scored (AI weighted total = 32+22.5+27 = 81.5)
	if _, err := w.ExecContext(ctx,
		`INSERT INTO evaluations (id,task_id,student_id,upload_id,status,total_score,created_at)
		 VALUES (1,1,4,1,'scored',81.5,?)`, now); err != nil {
		return nil, fmt.Errorf("seed eval_a: %w", err)
	}
	f.EvalAID = 1
	f.EvalADimScores = []DimScoreFixture{
		{DimensionID: 1, AIScore: 80, TeacherScore: nil},
		{DimensionID: 2, AIScore: 75, TeacherScore: nil},
		{DimensionID: 3, AIScore: 90, TeacherScore: nil},
	}
	for _, ds := range f.EvalADimScores {
		var tScore any
		if ds.TeacherScore != nil {
			tScore = *ds.TeacherScore
		}
		if _, err := w.ExecContext(ctx,
			`INSERT INTO dimension_scores (evaluation_id,dimension_id,ai_score,teacher_score)
			 VALUES (1,?,?,?)`, ds.DimensionID, ds.AIScore, tScore); err != nil {
			return nil, fmt.Errorf("seed dim_score_eval_a dim=%d: %w", ds.DimensionID, err)
		}
	}

	// Eval C: student C → task B, scored (AI weighted total = 51+28 = 79.0)
	if _, err := w.ExecContext(ctx,
		`INSERT INTO evaluations (id,task_id,student_id,upload_id,status,total_score,created_at)
		 VALUES (2,2,6,3,'scored',79.0,?)`, now); err != nil {
		return nil, fmt.Errorf("seed eval_c: %w", err)
	}
	f.EvalCID = 2
	f.EvalCDimScores = []DimScoreFixture{
		{DimensionID: 4, AIScore: 85, TeacherScore: nil},
		{DimensionID: 5, AIScore: 70, TeacherScore: nil},
	}
	for _, ds := range f.EvalCDimScores {
		var tScore any
		if ds.TeacherScore != nil {
			tScore = *ds.TeacherScore
		}
		if _, err := w.ExecContext(ctx,
			`INSERT INTO dimension_scores (evaluation_id,dimension_id,ai_score,teacher_score)
			 VALUES (2,?,?,?)`, ds.DimensionID, ds.AIScore, tScore); err != nil {
			return nil, fmt.Errorf("seed dim_score_eval_c dim=%d: %w", ds.DimensionID, err)
		}
	}

	return f, nil
}
