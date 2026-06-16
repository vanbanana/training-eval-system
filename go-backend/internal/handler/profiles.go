package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/service"
	"github.com/smartedu/training-eval-system/internal/store"

	"log/slog"
	"net/http"
)

type ProfilesHandler struct {
	svc       *service.ProfileService
	db        *store.DB
	llmClient *llm.Client
}

func NewProfilesHandler(svc *service.ProfileService, db *store.DB, llmClient *llm.Client) *ProfilesHandler {
	return &ProfilesHandler{svc: svc, db: db, llmClient: llmClient}
}

func (h *ProfilesHandler) GetStudent(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "userId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	profile, err := h.svc.GetByStudentID(r.Context(), id)
	if err != nil || profile == nil {
		Error(w, http.StatusNotFound, "Profile not found")
		return
	}
	JSON(w, http.StatusOK, dto.StudentProfileResponse{
		ID: profile.ID, StudentID: profile.StudentID,
		RadarData: profile.RadarData, WeaknessList: profile.WeaknessList,
		Suggestions: profile.Suggestions, ScoreTrend: profile.ScoreTrend,
		SourceEvaluationCount: profile.SourceEvaluationCount,
		ComputedAt:            profile.ComputedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetSchool returns school-level teaching quality profile with LLM summary (requirement 14).
func (h *ProfilesHandler) GetSchool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	school := h.computeSchoolProfile(ctx)
	JSON(w, http.StatusOK, school)
}

// GetCourse returns course-level teaching quality profile with LLM summary (requirement 14).
func (h *ProfilesHandler) GetCourse(w http.ResponseWriter, r *http.Request) {
	courseID, err := PathInt64(r, "courseId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid course ID")
		return
	}
	ctx := r.Context()

	course := h.computeCourseProfile(ctx, courseID)
	JSON(w, http.StatusOK, course)
}

func (h *ProfilesHandler) computeSchoolProfile(ctx context.Context) dto.SchoolProfileResponse {
	resp := dto.SchoolProfileResponse{}

	// Total students enrolled
	h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role='student' AND is_active=1").Scan(&resp.TotalStudents)

	// Average score across all evaluations
	h.db.Reader.QueryRowContext(ctx,
		"SELECT COALESCE(AVG(total_score), 0) FROM evaluations WHERE total_score IS NOT NULL AND status IN ('scored','confirmed')").Scan(&resp.AverageScore)

	// Completion rate (scored / total uploads)
	var totalUploads, scoredEvals int64
	h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM uploads WHERE parse_status='parsed'").Scan(&totalUploads)
	h.db.Reader.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM evaluations WHERE status IN ('scored','confirmed')").Scan(&scoredEvals)
	if totalUploads > 0 {
		resp.CompletionRate = float64(scoredEvals) / float64(totalUploads) * 100
	}

	// Score distribution
	type bucket struct {
		label    string
		min, max float64
	}
	buckets := []bucket{{"0-59", 0, 59}, {"60-69", 60, 69}, {"70-79", 70, 79}, {"80-89", 80, 89}, {"90-100", 90, 100}}
	distribution := make([]int, len(buckets))
	for i, b := range buckets {
		var count int64
		h.db.Reader.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM evaluations WHERE total_score >= ? AND total_score <= ? AND status IN ('scored','confirmed')",
			b.min, b.max).Scan(&count)
		distribution[i] = int(count)
	}
	resp.ScoreDistribution = distribution

	// Per-dimension averages across all tasks — detect common weaknesses (>30% students avg<60 per dim)
	type dimStat struct {
		Name                 string  `json:"name"`
		AverageScore         float64 `json:"average_score"`
		WeakStudentRatio     float64 `json:"weak_student_ratio"`
		WeakStudentThreshold int     `json:"-"`
	}
	rows, err := h.db.Reader.QueryContext(ctx,
		`SELECT d.name, AVG(ds.ai_score) as avg_score,
		   SUM(CASE WHEN ds.ai_score < 60 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as weak_ratio
		 FROM dimension_scores ds
		 JOIN dimensions d ON d.id = ds.dimension_id
		 WHERE ds.ai_score IS NOT NULL
		 GROUP BY d.name ORDER BY avg_score ASC`)
	if err == nil {
		defer rows.Close()
		commonThreshold := 30.0
		for rows.Next() {
			var ds dimStat
			rows.Scan(&ds.Name, &ds.AverageScore, &ds.WeakStudentRatio)
			resp.TopDimensions = append(resp.TopDimensions, ds)
			if ds.WeakStudentRatio > commonThreshold {
				resp.CommonWeaknesses = append(resp.CommonWeaknesses, map[string]any{
					"dimension":  ds.Name,
					"avg_score":  ds.AverageScore,
					"weak_ratio": ds.WeakStudentRatio,
				})
				resp.RecommendTeachingFor = append(resp.RecommendTeachingFor, ds.Name)
			}
		}
	}

	// LLM summary (requirement 14.4)
	if h.llmClient != nil {
		llmSummary := h.generateTeachingSummary(ctx, "学校", []string{}, resp.AverageScore, resp.ScoreDistribution, resp.RecommendTeachingFor)
		resp.LLMSummary = llmSummary
	}

	return resp
}

func (h *ProfilesHandler) computeCourseProfile(ctx context.Context, courseID int64) dto.CourseProfileResponse {
	resp := dto.CourseProfileResponse{CourseID: courseID}

	// Course name
	h.db.Reader.QueryRowContext(ctx, "SELECT name FROM courses WHERE id=?", courseID).Scan(&resp.CourseName)

	// Total students in this course
	h.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT cm.student_id) FROM class_memberships cm
		 JOIN classes c ON c.id = cm.class_id WHERE c.course_id = ?`, courseID).Scan(&resp.TotalStudents)

	// Average score for tasks in this course
	h.db.Reader.QueryRowContext(ctx,
		`SELECT COALESCE(AVG(e.total_score), 0) FROM evaluations e
		 JOIN training_tasks t ON t.id = e.task_id WHERE t.course_id = ? AND e.total_score IS NOT NULL`, courseID).Scan(&resp.AverageScore)

	// Completion rate
	var totalUploads, scoredEvals int64
	h.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM uploads u JOIN training_tasks t ON t.id=u.task_id WHERE t.course_id=? AND u.parse_status='parsed'`, courseID).Scan(&totalUploads)
	h.db.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM evaluations e JOIN training_tasks t ON t.id=e.task_id WHERE t.course_id=? AND e.status IN ('scored','confirmed')`, courseID).Scan(&scoredEvals)
	if totalUploads > 0 {
		resp.CompletionRate = float64(scoredEvals) / float64(totalUploads) * 100
	}

	// Score distribution
	type bucket struct {
		label    string
		min, max float64
	}
	buckets := []bucket{{"0-59", 0, 59}, {"60-69", 60, 69}, {"70-79", 70, 79}, {"80-89", 80, 89}, {"90-100", 90, 100}}
	distribution := make([]int, len(buckets))
	for i, b := range buckets {
		var count int64
		h.db.Reader.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM evaluations e JOIN training_tasks t ON t.id=e.task_id
			 WHERE t.course_id=? AND e.total_score >= ? AND e.total_score <= ?`, courseID, b.min, b.max).Scan(&count)
		distribution[i] = int(count)
	}
	resp.ScoreDistribution = distribution

	// Class comparisons
	type classBrief struct {
		ClassID      int64   `json:"class_id"`
		ClassName    string  `json:"class_name"`
		AvgScore     float64 `json:"avg_score"`
		StudentCount int     `json:"student_count"`
	}
	classRows, err := h.db.Reader.QueryContext(ctx,
		`SELECT c.id, c.name,
		   COALESCE(AVG(e.total_score), 0) as avg,
		   COUNT(DISTINCT cm.student_id) as cnt
		 FROM classes c
		 LEFT JOIN class_memberships cm ON cm.class_id = c.id
		 LEFT JOIN training_tasks t ON t.course_id = c.course_id
		 LEFT JOIN evaluations e ON e.task_id = t.id AND e.student_id = cm.student_id
		 WHERE c.course_id = ?
		 GROUP BY c.id ORDER BY avg DESC`, courseID)
	if err == nil {
		defer classRows.Close()
		for classRows.Next() {
			var cb classBrief
			classRows.Scan(&cb.ClassID, &cb.ClassName, &cb.AvgScore, &cb.StudentCount)
			resp.ClassComparisons = append(resp.ClassComparisons, cb)
		}
	}

	// Per-dimension common weaknesses (>30% students with avg<60)
	dimRows, err := h.db.Reader.QueryContext(ctx,
		`SELECT d.name,
		   AVG(ds.ai_score) as avg_score,
		   SUM(CASE WHEN ds.ai_score < 60 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as weak_ratio
		 FROM dimension_scores ds
		 JOIN dimensions d ON d.id = ds.dimension_id
		 JOIN evaluations e ON e.id = ds.evaluation_id
		 JOIN training_tasks t ON t.id = e.task_id
		 WHERE t.course_id = ? AND ds.ai_score IS NOT NULL
		 GROUP BY d.name ORDER BY avg_score ASC`, courseID)
	if err == nil {
		defer dimRows.Close()
		for dimRows.Next() {
			var name string
			var avg, weakRatio float64
			dimRows.Scan(&name, &avg, &weakRatio)
			if weakRatio > 30 {
				resp.CommonWeaknesses = append(resp.CommonWeaknesses, map[string]any{
					"dimension": name, "avg_score": avg, "weak_ratio": weakRatio,
				})
				resp.RecommendTeachingFor = append(resp.RecommendTeachingFor, name)
			}
		}
	}

	// LLM summary (requirement 14.4)
	if h.llmClient != nil {
		llmSummary := h.generateTeachingSummary(ctx, resp.CourseName, resp.RecommendTeachingFor, resp.AverageScore, resp.ScoreDistribution, resp.RecommendTeachingFor)
		resp.LLMSummary = llmSummary
	}

	return resp
}

// generateTeachingSummary calls LLM to produce a natural-language teaching quality summary (requirement 14.4).
func (h *ProfilesHandler) generateTeachingSummary(ctx context.Context, scope string, commonDim []string, avgScore float64, distribution []int, recommend []string) string {
	if h.llmClient == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("你是教学质量管理分析专家。请为以下教学质量画像生成中文总结（不少于150字）：\n\n"))
	sb.WriteString(fmt.Sprintf("分析范围：%s\n", scope))
	sb.WriteString(fmt.Sprintf("平均得分：%.1f\n", avgScore))
	sb.WriteString(fmt.Sprintf("分数分布（0-59, 60-69, 70-79, 80-89, 90-100）：[%s]\n", joinInts(distribution)))

	if len(commonDim) > 0 {
		sb.WriteString(fmt.Sprintf("需关注的共性薄弱维度（超过30%%学生得分<60）：%s\n", strings.Join(commonDim, "、")))
	}
	if len(recommend) > 0 {
		sb.WriteString(fmt.Sprintf("建议加强教学的维度：%s\n", strings.Join(recommend, "、")))
	}

	sb.WriteString("\n请给出教学评价总结、存在的共性问题分析、以及改进建议。")

	messages := []llm.ChatMessage{
		llm.NewTextMessage("system", "你是教学质量管理分析专家。请基于数据生成简洁、有洞察力的中文教学分析总结。直接输出总结内容，不要附加JSON格式。"),
		llm.NewTextMessage("user", sb.String()),
	}

	resp, err := h.llmClient.Complete(ctx, messages, nil)
	if err != nil {
		slog.Warn("teaching_summary: LLM call failed", "error", err.Error())
		return "教学画像数据已生成。LLM 总结暂不可用。"
	}

	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
		return resp.Choices[0].Message.Content
	}

	return ""
}

func joinInts(vals []int) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ", ")
}

// suppress unused import
var _ = json.Marshal
var _ = middleware.GetClaims
