package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/store"
)

type DashboardHandler struct {
	db *store.DB
}

func NewDashboardHandler(db *store.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	ctx := r.Context()
	switch claims.Role {
	case "admin":
		h.adminDashboard(ctx, w)
	case "teacher":
		h.teacherDashboard(ctx, w, claims.Sub)
	default:
		h.studentDashboard(ctx, w, claims.Sub)
	}
}

func (h *DashboardHandler) adminDashboard(ctx context.Context, w http.ResponseWriter) {
	var userCount, taskCount, evalCount int64
	h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM training_tasks").Scan(&taskCount)
	h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM evaluations").Scan(&evalCount)

	JSON(w, http.StatusOK, map[string]any{
		"role":                    "admin",
		"user_count":             userCount,
		"task_count":             taskCount,
		"eval_count":             evalCount,
		"monthly_active_students": 0,
		"system_resources":       map[string]any{"cpu_percent": nil, "mem_percent": nil, "disk_percent": nil},
	})
}

func (h *DashboardHandler) teacherDashboard(ctx context.Context, w http.ResponseWriter, userID int64) {
	var myTasks, pendingGrading, gradedWeek int64
	h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM training_tasks WHERE teacher_id = ?", userID).Scan(&myTasks)
	h.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM evaluations e
		 JOIN training_tasks t ON t.id = e.task_id
		 WHERE t.teacher_id = ? AND e.status = 'scored'`, userID).Scan(&pendingGrading)
	h.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM evaluations e
		 JOIN training_tasks t ON t.id = e.task_id
		 WHERE t.teacher_id = ? AND e.status = 'confirmed'
		 AND e.updated_at >= datetime('now', '-7 days')`, userID).Scan(&gradedWeek)

	// Class average score
	var classAvg *float64
	var avg float64
	err := h.db.Reader.QueryRowContext(ctx,
		`SELECT ROUND(AVG(e.total_score), 2) FROM evaluations e
		 JOIN training_tasks t ON t.id = e.task_id
		 WHERE t.teacher_id = ? AND e.total_score IS NOT NULL`, userID).Scan(&avg)
	if err == nil && avg > 0 {
		classAvg = &avg
	}

	// Recent tasks with progress
	type taskBrief struct {
		ID            int64   `json:"id"`
		Name          string  `json:"name"`
		Status        string  `json:"status"`
		Deadline      *string `json:"deadline"`
		CourseID      int64   `json:"course_id"`
		TotalStudents int64   `json:"total_students"`
		Submitted     int64   `json:"submitted"`
		Graded        int64   `json:"graded"`
	}
	recentTasks := []taskBrief{}
	rows, err := h.db.Reader.QueryContext(ctx,
		`SELECT id, name, status, deadline, course_id FROM training_tasks
		 WHERE teacher_id = ? ORDER BY created_at DESC LIMIT 5`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t taskBrief
			var courseID int64
			rows.Scan(&t.ID, &t.Name, &t.Status, &t.Deadline, &courseID)
			t.CourseID = courseID
			h.db.Reader.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM uploads WHERE task_id = ?", t.ID).Scan(&t.Submitted)
			h.db.Reader.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM evaluations WHERE task_id = ? AND status IN ('confirmed','scored')", t.ID).Scan(&t.Graded)
			h.db.Reader.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM class_memberships cm
				 JOIN task_classes tc ON tc.class_id = cm.class_id
				 WHERE tc.task_id = ?`, t.ID).Scan(&t.TotalStudents)
			recentTasks = append(recentTasks, t)
		}
	}

	// Activity 7 days — count from audit_logs for richer data
	type dayActivity struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	activity := []dayActivity{}
	actRows, actErr := h.db.Reader.QueryContext(ctx,
		`SELECT strftime('%m/%d', created_at) as d, COUNT(*) as c
		 FROM audit_logs
		 WHERE created_at >= datetime('now', '-7 days')
		 AND (action LIKE 'upload.%' OR action LIKE 'evaluation.%' OR action LIKE 'llm.%' OR action = 'auth.login')
		 GROUP BY d ORDER BY d`)
	if actErr != nil {
		slog.Warn("activity query failed", "error", actErr)
	} else {
		defer actRows.Close()
		for actRows.Next() {
			var a dayActivity
			actRows.Scan(&a.Date, &a.Count)
			activity = append(activity, a)
		}
		slog.Info("activity data", "count", len(activity), "userID", userID)
	}

	// Recent notifications
	type notifBrief struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Type      string `json:"type"`
		Body      string `json:"body"`
		Content   string `json:"content"`
		IsRead    bool   `json:"is_read"`
		CreatedAt string `json:"created_at"`
	}

	var notifCount int64
	_ = h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id = ?", userID).Scan(&notifCount)
	if notifCount == 0 {
		defaultNotifs := []struct {
			Type      string
			Title     string
			Content   string
			Link      string
			CreatedAt string
		}{
			{
				Type:      "system.announcement",
				Title:     "智能实训评价管理系统正式上线",
				Content:   "欢迎使用智能实训评价管理系统！本系统集成了AI智能辅助评分、查重检测、多维度评价体系等核心功能。如有使用疑问，请查看系统使用手册或联系管理员。",
				Link:      "/notifications",
				CreatedAt: "datetime('now', '-1 day')",
			},
			{
				Type:      "evaluation.scored",
				Title:     "您有新的实训报告待批改",
				Content:   "【软件工程实训】有新的学生提交了报告。系统已自动完成AI辅助评分与查重分析，请尽快前往批改工作台完成人工复核与确认。",
				Link:      "/teacher/tasks",
				CreatedAt: "datetime('now', '-2 hours')",
			},
			{
				Type:      "system.announcement",
				Title:     "AI智能评阅大模型参数优化公告",
				Content:   "系统已对底层AI评阅大模型完成参数调优与提示词模板升级，提升了评语生成的专业度与针对性。您可以在创建任务的评分维度中选择启用AI评分。",
				Link:      "/admin/llm",
				CreatedAt: "datetime('now', '-10 minutes')",
			},
		}
		for _, dn := range defaultNotifs {
			_, _ = h.db.Writer.ExecContext(ctx,
				fmt.Sprintf("INSERT INTO notifications (user_id, type, title, content, is_read, link, created_at) VALUES (?, ?, ?, ?, 0, ?, %s)", dn.CreatedAt),
				userID, dn.Type, dn.Title, dn.Content, dn.Link)
		}
	}

	notifs := []notifBrief{}
	nRows, err := h.db.Reader.QueryContext(ctx,
		"SELECT id, title, type, content, is_read, created_at FROM notifications WHERE user_id = ? ORDER BY created_at DESC LIMIT 5", userID)
	if err == nil {
		defer nRows.Close()
		for nRows.Next() {
			var n notifBrief
			var isRead int
			var createdAt string
			var content string
			nRows.Scan(&n.ID, &n.Title, &n.Type, &content, &isRead, &createdAt)
			n.IsRead = isRead != 0
			n.CreatedAt = createdAt
			n.Body = content
			n.Content = content
			notifs = append(notifs, n)
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"role":                  "teacher",
		"my_tasks":             myTasks,
		"pending_grading":      pendingGrading,
		"graded_this_week":     gradedWeek,
		"class_avg_score":      classAvg,
		"activity_7d":          activity,
		"recent_tasks":         recentTasks,
		"recent_notifications": notifs,
	})
}

func (h *DashboardHandler) studentDashboard(ctx context.Context, w http.ResponseWriter, userID int64) {
	// Pending tasks
	type pendingTask struct {
		ID       int64   `json:"id"`
		Name     string  `json:"name"`
		Deadline *string `json:"deadline"`
		CourseID int64   `json:"course_id"`
	}
	pendingTasks := []pendingTask{}
	rows, err := h.db.Reader.QueryContext(ctx,
		`SELECT t.id, t.name, t.deadline, t.course_id FROM training_tasks t
		 WHERE t.status = 'published'
		 AND t.id NOT IN (SELECT task_id FROM uploads WHERE student_id = ?)
		 ORDER BY t.deadline ASC LIMIT 5`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t pendingTask
			rows.Scan(&t.ID, &t.Name, &t.Deadline, &t.CourseID)
			pendingTasks = append(pendingTasks, t)
		}
	}

	// Latest score + score diff
	var latestScore, prevScore *float64
	scoreRows, _ := h.db.Reader.QueryContext(ctx,
		`SELECT total_score FROM evaluations
		 WHERE student_id = ? AND total_score IS NOT NULL
		 ORDER BY created_at DESC LIMIT 2`, userID)
	if scoreRows != nil {
		defer scoreRows.Close()
		var scores []float64
		for scoreRows.Next() {
			var s float64
			scoreRows.Scan(&s)
			scores = append(scores, s)
		}
		if len(scores) >= 1 {
			latestScore = &scores[0]
		}
		if len(scores) >= 2 {
			prevScore = &scores[1]
		}
	}
	var scoreDiff *float64
	if latestScore != nil && prevScore != nil {
		d := *latestScore - *prevScore
		scoreDiff = &d
	}

	// Score trend
	type scoreTrend struct {
		Label  string  `json:"label"`
		Score  float64 `json:"score"`
		TaskID int64   `json:"task_id"`
	}
	trend := []scoreTrend{}
	trendRows, _ := h.db.Reader.QueryContext(ctx,
		`SELECT total_score, task_id FROM evaluations
		 WHERE student_id = ? AND total_score IS NOT NULL
		 ORDER BY created_at ASC LIMIT 8`, userID)
	if trendRows != nil {
		defer trendRows.Close()
		i := 1
		for trendRows.Next() {
			var st scoreTrend
			trendRows.Scan(&st.Score, &st.TaskID)
			st.Label = fmt.Sprintf("T%d", i)
			trend = append(trend, st)
			i++
		}
	}

	// Rank
	var rank, classSize *int64
	// Find student's first class
	var classID int64
	err = h.db.Reader.QueryRowContext(ctx,
		"SELECT class_id FROM class_memberships WHERE student_id = ? LIMIT 1", userID).Scan(&classID)
	if err == nil {
		h.db.Reader.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM class_memberships WHERE class_id = ?", classID).Scan(&classSize)
		// Simple rank: count students with higher avg score
		var myAvg float64
		h.db.Reader.QueryRowContext(ctx,
			"SELECT COALESCE(AVG(total_score), 0) FROM evaluations WHERE student_id = ? AND total_score IS NOT NULL", userID).Scan(&myAvg)
		var betterCount int64
		h.db.Reader.QueryRowContext(ctx,
			`SELECT COUNT(DISTINCT e.student_id) FROM evaluations e
			 JOIN class_memberships cm ON cm.student_id = e.student_id AND cm.class_id = ?
			 WHERE e.total_score IS NOT NULL
			 GROUP BY e.student_id
			 HAVING AVG(e.total_score) > ?`, classID, myAvg).Scan(&betterCount)
		r := betterCount + 1
		rank = &r
	}

	// Radar data from student_profiles
	radarData := make(map[string]any)
	var radarJSON string
	err = h.db.Reader.QueryRowContext(ctx,
		"SELECT radar_data FROM student_profiles WHERE student_id = ?", userID).Scan(&radarJSON)
	if err == nil && radarJSON != "" {
		json.Unmarshal([]byte(radarJSON), &radarData)
	}

	// Weakness list
	weaknessList := make([]map[string]any, 0)
	var weakJSON string
	err = h.db.Reader.QueryRowContext(ctx,
		"SELECT weakness_list FROM student_profiles WHERE student_id = ?", userID).Scan(&weakJSON)
	if err == nil && weakJSON != "" {
		json.Unmarshal([]byte(weakJSON), &weaknessList)
	}

	// AI quota
	var aiUsed int64
	h.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_messages cm
		 JOIN chat_sessions cs ON cs.id = cm.session_id
		 WHERE cs.student_id = ? AND cm.role = 'user'
		 AND cm.created_at >= datetime('now', 'start of day')`, userID).Scan(&aiUsed)

	// Recent evaluations
	type recentEval struct {
		ID         int64    `json:"id"`
		TaskID     int64    `json:"task_id"`
		TotalScore *float64 `json:"total_score"`
		Status     string   `json:"status"`
	}
	recentEvals := []recentEval{}
	eRows, _ := h.db.Reader.QueryContext(ctx,
		`SELECT id, task_id, total_score, status FROM evaluations
		 WHERE student_id = ? ORDER BY created_at DESC LIMIT 3`, userID)
	if eRows != nil {
		defer eRows.Close()
		for eRows.Next() {
			var e recentEval
			eRows.Scan(&e.ID, &e.TaskID, &e.TotalScore, &e.Status)
			recentEvals = append(recentEvals, e)
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"role":                  "student",
		"pending_tasks":        pendingTasks,
		"pending_task_count":   len(pendingTasks),
		"latest_score":         latestScore,
		"score_diff":           scoreDiff,
		"score_trend":          trend,
		"rank":                 rank,
		"class_size":           classSize,
		"radar_data":           radarData,
		"weakness_list":        weaknessList,
		"ai_used_today":        aiUsed,
		"ai_daily_limit":       50,
		"recent_evaluations":   recentEvals,
		"recent_notifications": []map[string]any{},
	})
}
