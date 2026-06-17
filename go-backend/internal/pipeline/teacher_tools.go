// Package pipeline — teacher-side tools for AI-assisted grading and feedback (T3.3, T3.4).
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// TeacherToolContext carries teacher-specific context for tool dispatch.
type TeacherToolContext struct {
	TeacherID int64
	TaskID    *int64
	ClassID   *int64
	CourseID  *int64
}

// SetClassRepo sets the optional class repository for teacher tools.
func (co *ChatOrchestrator) SetClassRepo(r repository.ClassRepo) { co.classRepo = r }

// SetCourseRepo sets the optional course repository for teacher tools.
func (co *ChatOrchestrator) SetCourseRepo(r repository.CourseRepo) { co.courseRepo = r }

// SetSimilarityRepo sets the optional similarity repository for teacher tools.
func (co *ChatOrchestrator) SetSimilarityRepo(r repository.SimilarityRepo) { co.simRepo = r }

// TeacherToolSchemas returns all 8 teacher tool definitions.
func TeacherToolSchemas() []llm.Tool {
	return []llm.Tool{
		toolTeacherGetTaskSummary(),
		toolTeacherListPendingSubmissions(),
		toolTeacherGetClassPerformance(),
		toolTeacherGetDimensionDistribution(),
		toolTeacherGetEvaluationDetail(),
		toolTeacherGenerateFeedbackDraft(),
		toolTeacherSuggestScoreReview(),
		toolTeacherCompareWithRubric(),
	}
}

// teacherToolRequiredParams maps teacher tool names to their required parameters.
var teacherToolRequiredParams = map[string][]string{
	"teacher_get_task_summary":           {"task_id"},
	"teacher_list_pending_submissions":   {"task_id"},
	"teacher_get_class_performance":      {"class_id"},
	"teacher_get_dimension_distribution": {"task_id"},
	"teacher_get_evaluation_detail":      {"evaluation_id"},
	"teacher_generate_feedback_draft":    {"evaluation_id"},
	"teacher_suggest_score_review":       {"evaluation_id"},
	"teacher_compare_with_rubric":        {"evaluation_id"},
}

// DispatchTeacherTool dispatches a teacher tool call by name with argument validation.
func (co *ChatOrchestrator) DispatchTeacherTool(ctx context.Context, name string, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	// Validate required parameters
	if required, ok := teacherToolRequiredParams[name]; ok {
		for _, param := range required {
			v, exists := args[param]
			if !exists || v == nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("工具 %s 缺少必填参数 %q，请提供该参数后重试", name, param),
				}
			}
		}
	}

	switch name {
	case "teacher_get_task_summary":
		return co.teacherGetTaskSummary(ctx, args, ttctx)
	case "teacher_list_pending_submissions":
		return co.teacherListPendingSubmissions(ctx, args, ttctx)
	case "teacher_get_class_performance":
		return co.teacherGetClassPerformance(ctx, args, ttctx)
	case "teacher_get_dimension_distribution":
		return co.teacherGetDimensionDistribution(ctx, args, ttctx)
	case "teacher_get_evaluation_detail":
		return co.teacherGetEvaluationDetail(ctx, args, ttctx)
	case "teacher_generate_feedback_draft":
		return co.teacherGenerateFeedbackDraft(ctx, args, ttctx)
	case "teacher_suggest_score_review":
		return co.teacherSuggestScoreReview(ctx, args, ttctx)
	case "teacher_compare_with_rubric":
		return co.teacherCompareWithRubric(ctx, args, ttctx)
	default:
		return &ToolResult{Success: false, Error: fmt.Sprintf("unknown teacher tool: %s", name)}
	}
}

// RunTeacher executes the teacher tool-calling loop similar to Run() but with teacher tools.
func (co *ChatOrchestrator) RunTeacher(
	ctx context.Context,
	history []llm.ChatMessage,
	userMessage string,
	ttctx *TeacherToolContext,
	systemPrompt string,
) (*llm.ChatResponse, error) {
	messages := make([]llm.ChatMessage, 0, len(history)+10)
	messages = append(messages, llm.NewTextMessage("system", systemPrompt))
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	messages = append(messages, history...)
	messages = append(messages, llm.NewTextMessage("user", userMessage))

	tools := TeacherToolSchemas()
	consecutiveFailedRounds := 0

	for round := 0; round < MaxToolRounds; round++ {
		resp, err := co.retryLLMCall(ctx, messages, tools)
		if err != nil {
			if round > 0 {
				return co.fallbackResponse(ctx, messages)
			}
			return nil, fmt.Errorf("teacher orchestrator: LLM call failed: %w", err)
		}
		if len(resp.Choices) == 0 {
			if round > 0 {
				return co.fallbackResponse(ctx, messages)
			}
			return nil, fmt.Errorf("teacher orchestrator: empty response")
		}

		choice := resp.Choices[0]
		if choice.FinishReason == "stop" && choice.Message.Content != "" {
			return resp, nil
		}
		if len(choice.Message.ToolCalls) == 0 {
			return resp, nil
		}

		messages = append(messages, llm.ChatMessage{
			Role:      "assistant",
			ToolCalls: choice.Message.ToolCalls,
		})

		roundToolFailures := 0
		roundToolTotal := len(choice.Message.ToolCalls)

		for _, tc := range choice.Message.ToolCalls {
			if co.OnToolCall != nil {
				co.OnToolCall(tc.Function.Name)
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{}
			}
			result := co.DispatchTeacherTool(ctx, tc.Function.Name, args, ttctx)
			if result == nil {
				result = &ToolResult{Success: false, Error: "tool returned nil result"}
			}
			if !result.Success {
				roundToolFailures++
				result.Error = fmt.Sprintf("工具调用失败（%s）。请检查参数或尝试其他方式。错误详情：%s", tc.Function.Name, result.Error)
			}
			resultJSON, _ := json.Marshal(result)
			if len(resultJSON) > MaxToolResultBytes {
				resultJSON = resultJSON[:MaxToolResultBytes]
			}
			messages = append(messages, llm.ChatMessage{
				Role:       "tool",
				Content:    string(resultJSON),
				ToolCallID: tc.ID,
			})
		}

		if roundToolFailures == roundToolTotal {
			consecutiveFailedRounds++
			if consecutiveFailedRounds >= maxConsecutiveFailedRounds {
				break
			}
		} else {
			consecutiveFailedRounds = 0
		}
	}

	// Force final answer
	messages = append(messages, llm.NewTextMessage("user",
		"请基于已有的工具返回结果直接给出回答。不要再调用工具。"))
	resp, err := co.retryLLMCall(ctx, messages, nil)
	if err != nil {
		return co.fallbackResponse(ctx, messages)
	}
	return resp, nil
}

// --- Tool Implementations ---

// loadEvalWithOwnership loads an evaluation and verifies the teacher owns the associated task.
func (co *ChatOrchestrator) loadEvalWithOwnership(ctx context.Context, evalID, teacherID int64) (*model.Evaluation, error) {
	eval, err := co.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return nil, fmt.Errorf("evaluation not found")
	}
	task, err := co.taskRepo.GetByID(ctx, eval.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}
	if task.TeacherID != teacherID {
		return nil, fmt.Errorf("forbidden: teacher %d does not own task %d", teacherID, eval.TaskID)
	}
	return eval, nil
}

// fetchUploadSummary returns a truncated parse result summary for an evaluation's upload.
func (co *ChatOrchestrator) fetchUploadSummary(ctx context.Context, eval *model.Evaluation) (string, string) {
	upload, err := co.uploadRepo.GetByID(ctx, eval.UploadID)
	if err != nil || upload == nil {
		return "", "上传文件不可用"
	}
	pr, err := co.uploadRepo.GetParseResult(ctx, upload.ID)
	if err != nil || pr == nil {
		return "", "解析结果不可用"
	}
	text := pr.RawText
	if len(text) > 500 {
		text = text[:500] + "..."
	}
	return text, ""
}

// confirmedWarning returns a warning string if the evaluation is confirmed.
func confirmedWarning(eval *model.Evaluation) string {
	if eval.Status == "confirmed" {
		return "该评价已确认，修改需重新审核流程。"
	}
	return ""
}

// toInt64 safely converts a JSON number to int64.
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// safeFloat returns the float64 value or 0.
func safeFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// truncateStr truncates a string to maxLen.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// suppress unused import for strings (used in containsStr checks)
var _ = strings.Contains

// ============================================================
// T3.3 Tools
// ============================================================

func (co *ChatOrchestrator) teacherGetTaskSummary(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID, _ := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: fmt.Sprintf("forbidden: teacher %d does not own task %d", ttctx.TeacherID, taskID)}
	}

	// Get uploads
	uploads, _, _ := co.uploadRepo.List(ctx, repository.UploadListParams{TaskID: &taskID})
	submitted := len(uploads)

	// Get evaluations
	statusScored := "scored"
	statusConfirmed := "confirmed"
	scoredEvals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID, Status: &statusScored})
	confirmedEvals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID, Status: &statusConfirmed})

	scoredCount := len(scoredEvals)
	confirmedCount := len(confirmedEvals)

	// Calculate average score from scored + confirmed evals
	var totalScore float64
	var scoreCount int
	for _, e := range scoredEvals {
		if e.TotalScore != nil {
			totalScore += *e.TotalScore
			scoreCount++
		}
	}
	for _, e := range confirmedEvals {
		if e.TotalScore != nil {
			totalScore += *e.TotalScore
			scoreCount++
		}
	}
	var avg float64
	if scoreCount > 0 {
		avg = totalScore / float64(scoreCount)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"task_name":     task.Name,
			"submitted":     submitted,
			"scored":        scoredCount,
			"confirmed":     confirmedCount,
			"pending":       submitted - scoredCount - confirmedCount,
			"average_score": avg,
		},
	}
}

type pendingItem struct {
	EvalID    int64   `json:"evaluation_id"`
	StudentID int64   `json:"student_id"`
	Score     float64 `json:"total_score"`
	Status    string  `json:"status"`
}

func (co *ChatOrchestrator) teacherListPendingSubmissions(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID, _ := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: fmt.Sprintf("forbidden: teacher %d does not own task %d", ttctx.TeacherID, taskID)}
	}

	statusScored := "scored"
	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID, Status: &statusScored})

	items := make([]pendingItem, 0, len(evals))
	for _, e := range evals {
		score := 0.0
		if e.TotalScore != nil {
			score = *e.TotalScore
		}
		items = append(items, pendingItem{
			EvalID:    e.ID,
			StudentID: e.StudentID,
			Score:     score,
			Status:    e.Status,
		})
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"items": items,
			"total": len(items),
		},
	}
}

func (co *ChatOrchestrator) teacherGetClassPerformance(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	classID, _ := toInt64(args["class_id"])
	if co.classRepo == nil {
		return &ToolResult{Success: false, Error: "class repository not configured"}
	}
	classObj, err := co.classRepo.GetByID(ctx, classID)
	if err != nil {
		return &ToolResult{Success: false, Error: "class not found"}
	}
	if classObj.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: fmt.Sprintf("forbidden: teacher %d does not own class %d", ttctx.TeacherID, classID)}
	}

	members, _ := co.classRepo.GetMembers(ctx, classID)
	if len(members) == 0 {
		return &ToolResult{
			Success: true,
			Data: map[string]any{
				"student_count": 0,
				"message":       "该班级暂无学生或暂无评价数据",
			},
		}
	}

	// Aggregate evaluation data for class members
	var totalScore float64
	var scoreCount int
	for _, m := range members {
		evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{StudentID: &m.StudentID})
		for _, e := range evals {
			if e.TotalScore != nil {
				totalScore += *e.TotalScore
				scoreCount++
			}
		}
	}

	var avg float64
	if scoreCount > 0 {
		avg = totalScore / float64(scoreCount)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"student_count": len(members),
			"average_score": avg,
			"class_name":    classObj.Name,
		},
	}
}

type dimDist struct {
	Name    string  `json:"name"`
	Average float64 `json:"average"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Count   int     `json:"count"`
}

func (co *ChatOrchestrator) teacherGetDimensionDistribution(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID, _ := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: fmt.Sprintf("forbidden: teacher %d does not own task %d", ttctx.TeacherID, taskID)}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, taskID)
	if len(dims) == 0 {
		return &ToolResult{Success: true, Data: map[string]any{"dimensions": []dimDist{}}}
	}

	// Get all evaluations for this task
	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID})

	// Build per-dimension stats
	dimStats := make(map[int64]*dimDist)
	for _, d := range dims {
		dimStats[d.ID] = &dimDist{Name: d.Name, Min: math.MaxFloat64, Max: -1}
	}

	for _, e := range evals {
		for _, s := range e.Scores {
			ds, ok := dimStats[s.DimensionID]
			if !ok {
				continue
			}
			// Teacher score takes precedence over AI score
			var score float64
			if s.TeacherScore != nil {
				score = *s.TeacherScore
			} else if s.AIScore != nil {
				score = *s.AIScore
			} else {
				continue
			}
			ds.Count++
			ds.Average += score
			if score < ds.Min {
				ds.Min = score
			}
			if score > ds.Max {
				ds.Max = score
			}
		}
	}

	result := make([]dimDist, 0, len(dims))
	for _, d := range dims {
		ds := dimStats[d.ID]
		if ds.Count > 0 {
			ds.Average = ds.Average / float64(ds.Count)
		} else {
			ds.Min = 0
			ds.Max = 0
		}
		result = append(result, *ds)
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"dimensions": result},
	}
}

// ============================================================
// T3.4 Tools
// ============================================================

func (co *ChatOrchestrator) teacherGetEvaluationDetail(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	evalID, _ := toInt64(args["evaluation_id"])
	eval, err := co.loadEvalWithOwnership(ctx, evalID, ttctx.TeacherID)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			return &ToolResult{Success: false, Error: err.Error()}
		}
		return &ToolResult{Success: false, Error: "evaluation not found"}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, eval.TaskID)
	dimNameMap := make(map[int64]string)
	for _, d := range dims {
		dimNameMap[d.ID] = d.Name
	}

	type scoreDetail struct {
		DimensionName string   `json:"dimension_name"`
		AIScore       *float64 `json:"ai_score,omitempty"`
		TeacherScore  *float64 `json:"teacher_score,omitempty"`
		Rationale     string   `json:"rationale"`
	}
	scores := make([]scoreDetail, 0, len(eval.Scores))
	for _, s := range eval.Scores {
		scores = append(scores, scoreDetail{
			DimensionName: dimNameMap[s.DimensionID],
			AIScore:       s.AIScore,
			TeacherScore:  s.TeacherScore,
			Rationale:     s.Rationale,
		})
	}

	uploadSummary, parseNote := co.fetchUploadSummary(ctx, eval)

	data := map[string]any{
		"status":   eval.Status,
		"scores":   scores,
	}
	if eval.TotalScore != nil {
		data["total_score"] = eval.TotalScore
	}
	if uploadSummary != "" {
		data["upload_summary"] = uploadSummary
	}
	if parseNote != "" {
		data["parse_result_note"] = parseNote
	}
	if w := confirmedWarning(eval); w != "" {
		data["confirmed_warning"] = w
	}

	return &ToolResult{Success: true, Data: data}
}

func (co *ChatOrchestrator) teacherGenerateFeedbackDraft(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	evalID, _ := toInt64(args["evaluation_id"])
	eval, err := co.loadEvalWithOwnership(ctx, evalID, ttctx.TeacherID)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			return &ToolResult{Success: false, Error: err.Error()}
		}
		return &ToolResult{Success: false, Error: "evaluation not found"}
	}

	task, _ := co.taskRepo.GetByID(ctx, eval.TaskID)
	dims, _ := co.taskRepo.GetDimensions(ctx, eval.TaskID)
	dimNameMap := make(map[int64]string)
	for _, d := range dims {
		dimNameMap[d.ID] = d.Name
	}

	var sb strings.Builder
	sb.WriteString("【评语草稿 — 需教师确认后方可生效】\n\n")

	if task != nil {
		sb.WriteString(fmt.Sprintf("任务：%s\n", task.Name))
	}
	if eval.TotalScore != nil {
		sb.WriteString(fmt.Sprintf("综合得分：%.1f\n", *eval.TotalScore))
	}

	sb.WriteString("\n各维度表现：\n")
	for _, s := range eval.Scores {
		name := dimNameMap[s.DimensionID]
		if name == "" {
			name = fmt.Sprintf("维度%d", s.DimensionID)
		}
		var score float64
		if s.TeacherScore != nil {
			score = *s.TeacherScore
		} else if s.AIScore != nil {
			score = *s.AIScore
		}
		sb.WriteString(fmt.Sprintf("- %s：%.1f分", name, score))
		if s.Rationale != "" {
			sb.WriteString(fmt.Sprintf("（%s）", s.Rationale))
		}
		sb.WriteString("\n")
	}

	uploadSummary, parseNote := co.fetchUploadSummary(ctx, eval)
	if uploadSummary != "" {
		sb.WriteString(fmt.Sprintf("\n提交摘要：%s\n", truncateStr(uploadSummary, 200)))
	}
	if parseNote != "" {
		sb.WriteString(fmt.Sprintf("\n注意：%s\n", parseNote))
	}

	if w := confirmedWarning(eval); w != "" {
		sb.WriteString(fmt.Sprintf("\n⚠️ %s\n", w))
	}

	sb.WriteString("\n（此草稿仅供教师参考，请教师审阅后在评价页面确认。）")

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"draft":    sb.String(),
			"is_draft": true,
		},
	}
}

func (co *ChatOrchestrator) teacherSuggestScoreReview(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	evalID, _ := toInt64(args["evaluation_id"])
	eval, err := co.loadEvalWithOwnership(ctx, evalID, ttctx.TeacherID)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			return &ToolResult{Success: false, Error: err.Error()}
		}
		return &ToolResult{Success: false, Error: "evaluation not found"}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, eval.TaskID)
	dimNameMap := make(map[int64]string)
	for _, d := range dims {
		dimNameMap[d.ID] = d.Name
	}

	type reviewItem struct {
		DimensionName string  `json:"dimension_name"`
		AIScore       float64 `json:"ai_score"`
		Reason        string  `json:"reason"`
	}

	var suggestions []reviewItem
	for _, s := range eval.Scores {
		if s.AIScore == nil {
			continue
		}
		score := *s.AIScore
		name := dimNameMap[s.DimensionID]
		if name == "" {
			name = fmt.Sprintf("维度%d", s.DimensionID)
		}
		// Flag very low scores or significant AI/teacher discrepancies
		if score < 40 {
			suggestions = append(suggestions, reviewItem{
				DimensionName: name,
				AIScore:       score,
				Reason:        fmt.Sprintf("AI 评分较低（%.1f），建议教师复核是否合理", score),
			})
		} else if s.TeacherScore != nil && math.Abs(score-*s.TeacherScore) > 15 {
			suggestions = append(suggestions, reviewItem{
				DimensionName: name,
				AIScore:       score,
				Reason:        fmt.Sprintf("AI（%.1f）与教师（%.1f）评分差异较大，建议复核", score, *s.TeacherScore),
			})
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"review_suggested": len(suggestions),
			"suggestions":      suggestions,
			"note":             "以上建议仅供参考，不自动改分。请在评分页面手动确认修改。",
		},
	}
}

type criteriaItem struct {
	Criterion     string  `json:"criterion"`
	Fulfillment   string  `json:"fulfillment"`
	Score         float64 `json:"score"`
}

func (co *ChatOrchestrator) teacherCompareWithRubric(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	evalID, _ := toInt64(args["evaluation_id"])
	eval, err := co.loadEvalWithOwnership(ctx, evalID, ttctx.TeacherID)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			return &ToolResult{Success: false, Error: err.Error()}
		}
		return &ToolResult{Success: false, Error: "evaluation not found"}
	}

	task, _ := co.taskRepo.GetByID(ctx, eval.TaskID)
	dims, _ := co.taskRepo.GetDimensions(ctx, eval.TaskID)
	dimNameMap := make(map[int64]string)
	dimDescMap := make(map[int64]string)
	for _, d := range dims {
		dimNameMap[d.ID] = d.Name
		dimDescMap[d.ID] = d.Description
	}

	var comparison []criteriaItem
	for _, s := range eval.Scores {
		name := dimNameMap[s.DimensionID]
		if name == "" {
			name = fmt.Sprintf("维度%d", s.DimensionID)
		}
		desc := dimDescMap[s.DimensionID]
		if desc == "" {
			desc = name
		}

		var score float64
		if s.TeacherScore != nil {
			score = *s.TeacherScore
		} else if s.AIScore != nil {
			score = *s.AIScore
		}

		fulfillment := "partial"
		if score >= 80 {
			fulfillment = "excellent"
		} else if score >= 60 {
			fulfillment = "adequate"
		} else if score >= 40 {
			fulfillment = "needs_improvement"
		} else {
			fulfillment = "poor"
		}

		criterion := desc
		if task != nil && task.EvaluationCriteria != "" {
			criterion = desc + "（标准：" + truncateStr(task.EvaluationCriteria, 100) + "）"
		}

		comparison = append(comparison, criteriaItem{
			Criterion:   criterion,
			Fulfillment: fulfillment,
			Score:       score,
		})
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"criteria_comparison": comparison,
			"note":                "对比结果仅供参考，不自动确认或驳回评价。请在评价页面手动操作。",
		},
	}
}

// ============================================================
// Tool Schema Definitions
// ============================================================

func toolTeacherGetTaskSummary() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "teacher_get_task_summary",
			Description: "获取指定任务的整体提交和评价统计信息，包括提交人数、已评分数、已确认数和平均分。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			},
		},
	}
}

func toolTeacherListPendingSubmissions() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "teacher_list_pending_submissions",
			Description: "列出指定任务中待教师确认（状态为 scored）的评价列表。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			},
		},
	}
}

func toolTeacherGetClassPerformance() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "teacher_get_class_performance",
			Description: "获取指定班级的整体表现统计，包括学生人数和平均分。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"class_id": map[string]any{"type": "integer", "description": "班级ID"},
				},
				"required": []string{"class_id"},
			},
		},
	}
}

func toolTeacherGetDimensionDistribution() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "teacher_get_dimension_distribution",
			Description: "获取指定任务各评分维度的分布统计（平均分、最高分、最低分）。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			},
		},
	}
}

func toolTeacherGetEvaluationDetail() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "teacher_get_evaluation_detail",
			Description: "获取某个评价的详细信息，包括各维度得分、评语和学生提交摘要。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"evaluation_id": map[string]any{"type": "integer", "description": "评价ID"},
				},
				"required": []string{"evaluation_id"},
			},
		},
	}
}

func toolTeacherGenerateFeedbackDraft() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "teacher_generate_feedback_draft",
			Description: "基于评价数据生成评语草稿，需教师确认后方可生效。不会自动写入数据库。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"evaluation_id": map[string]any{"type": "integer", "description": "评价ID"},
				},
				"required": []string{"evaluation_id"},
			},
		},
	}
}

func toolTeacherSuggestScoreReview() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "teacher_suggest_score_review",
			Description: "分析评分数据，识别可能需要教师复核的维度（如AI评分过低或与教师评分差异大）。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"evaluation_id": map[string]any{"type": "integer", "description": "评价ID"},
				},
				"required": []string{"evaluation_id"},
			},
		},
	}
}

func toolTeacherCompareWithRubric() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "teacher_compare_with_rubric",
			Description: "将学生各维度表现与评分标准对比，给出达成度等级。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"evaluation_id": map[string]any{"type": "integer", "description": "评价ID"},
				},
				"required": []string{"evaluation_id"},
			},
		},
	}
}
