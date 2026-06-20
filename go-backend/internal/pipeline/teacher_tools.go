// Package pipeline — Teacher tools for the AI-assisted evaluation system (T3.3–T3.6).
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// TeacherToolContext holds context for teacher tool calls.
type TeacherToolContext struct {
	TeacherID int64
	TaskID    *int64
	ClassID   *int64
	CourseID  *int64
	// OnToolCall is an optional per-request callback for SSE progress events.
	OnToolCall func(toolName string)
}

// SetClassRepo sets the optional ClassRepo on the orchestrator.
func (co *ChatOrchestrator) SetClassRepo(r repository.ClassRepo) { co.classRepo = r }

// SetCourseRepo sets the optional CourseRepo on the orchestrator.
func (co *ChatOrchestrator) SetCourseRepo(r repository.CourseRepo) { co.courseRepo = r }

// SetSimilarityRepo sets the optional SimilarityRepo on the orchestrator.
func (co *ChatOrchestrator) SetSimilarityRepo(r repository.SimilarityRepo) { co.simRepo = r }

// teacherToolRequiredParams maps tool names to their required parameter names.
var teacherToolRequiredParams = map[string][]string{
	"teacher_get_task_summary":           {"task_id"},
	"teacher_list_pending_submissions":   {"task_id"},
	"teacher_get_class_performance":      {"class_id"},
	"teacher_get_dimension_distribution": {"task_id"},
	"teacher_get_evaluation_detail":      {"evaluation_id"},
	"teacher_generate_feedback_draft":    {"evaluation_id"},
	"teacher_suggest_score_review":       {"evaluation_id"},
	"teacher_compare_with_rubric":        {"evaluation_id"},
	// T3.5 — Similarity explanation & teaching suggestions
	"teacher_get_similarity_summary":        {"task_id"},
	"teacher_explain_similarity_case":       {"record_id"},
	"teacher_generate_teaching_suggestions": {"task_id"},
	// T3.6 — Task & dimension rubric draft generation
	"teacher_generate_task_draft":             {},
	"teacher_generate_dimension_rubric_draft": {"task_id"},
	"teacher_generate_report_outline_draft":   {"task_id"},
}

// TeacherToolSchemas returns all 14 registered teacher tools as OpenAI function definitions.
func TeacherToolSchemas() []llm.Tool {
	return []llm.Tool{
		makeTool("teacher_get_task_summary",
			"获取指定任务的整体提交和评价统计信息，包括提交人数、已评分数、已确认数和平均分。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		makeTool("teacher_list_pending_submissions",
			"列出指定任务中待教师确认（状态为 scored）的评价列表。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		makeTool("teacher_get_class_performance",
			"获取指定班级中所有学生的评价表现汇总。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"class_id": map[string]any{"type": "integer", "description": "班级ID"},
				},
				"required": []string{"class_id"},
			}),
		makeTool("teacher_get_dimension_distribution",
			"获取指定任务中各评价维度的得分分布和统计信息。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		makeTool("teacher_get_evaluation_detail",
			"获取指定评价的详细信息，包括各维度得分、评语和解析摘要。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"evaluation_id": map[string]any{"type": "integer", "description": "评价ID"},
				},
				"required": []string{"evaluation_id"},
			}),
		makeTool("teacher_generate_feedback_draft",
			"基于评价结果和学生提交内容，生成反馈评语草稿（仅供教师审阅，不会自动保存）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"evaluation_id": map[string]any{"type": "integer", "description": "评价ID"},
				},
				"required": []string{"evaluation_id"},
			}),
		makeTool("teacher_suggest_score_review",
			"分析评价中各维度得分，识别可能需要教师复核的维度（不会自动改分）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"evaluation_id": map[string]any{"type": "integer", "description": "评价ID"},
				},
				"required": []string{"evaluation_id"},
			}),
		makeTool("teacher_compare_with_rubric",
			"将评价结果与任务评分标准进行对比，检查各维度是否满足评分要求（不会自动确认）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"evaluation_id": map[string]any{"type": "integer", "description": "评价ID"},
				},
				"required": []string{"evaluation_id"},
			}),
		// T3.5 — Similarity explanation & teaching suggestions
		makeTool("teacher_get_similarity_summary",
			"获取指定任务的查重记录汇总，包括疑似、已确认、已忽略的数量和概要。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		makeTool("teacher_explain_similarity_case",
			"解释某条查重记录的详细情况，包括相似度指标和片段摘要（不会自动定性作弊）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"record_id": map[string]any{"type": "integer", "description": "查重记录ID"},
				},
				"required": []string{"record_id"},
			}),
		makeTool("teacher_generate_teaching_suggestions",
			"基于任务统计和薄弱维度，生成教学改进建议（仅供参考，需教师确认）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		// T3.6 — Task & dimension rubric draft generation
		makeTool("teacher_generate_task_draft",
			"生成新实训任务的草稿，包括名称、描述、要求和评分维度（不会自动创建任务）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"course_id": map[string]any{"type": "integer", "description": "课程ID（可选）"},
					"brief":     map[string]any{"type": "string", "description": "任务简要描述"},
				},
			}),
		makeTool("teacher_generate_dimension_rubric_draft",
			"为指定任务生成评分维度草稿，包括维度名称、权重和评分标准（不会自动替换现有维度）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		makeTool("teacher_generate_report_outline_draft",
			"为指定任务生成评价报告大纲草稿，包括总体分析和建议（不会自动写入报告）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "integer", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
	}
}

// DispatchTeacherTool dispatches a teacher tool call by name with argument validation.
func (co *ChatOrchestrator) DispatchTeacherTool(ctx context.Context, name string, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	// Validate required parameters
	if required, ok := teacherToolRequiredParams[name]; ok {
		for _, param := range required {
			if v, exists := args[param]; !exists || v == nil {
				return &ToolResult{Success: false, Error: fmt.Sprintf("missing required parameter: %s", param)}
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
	// T3.5
	case "teacher_get_similarity_summary":
		return co.teacherGetSimilaritySummary(ctx, args, ttctx)
	case "teacher_explain_similarity_case":
		return co.teacherExplainSimilarityCase(ctx, args, ttctx)
	case "teacher_generate_teaching_suggestions":
		return co.teacherGenerateTeachingSuggestions(ctx, args, ttctx)
	// T3.6
	case "teacher_generate_task_draft":
		return co.teacherGenerateTaskDraft(ctx, args, ttctx)
	case "teacher_generate_dimension_rubric_draft":
		return co.teacherGenerateDimensionRubricDraft(ctx, args, ttctx)
	case "teacher_generate_report_outline_draft":
		return co.teacherGenerateReportOutlineDraft(ctx, args, ttctx)
	default:
		return &ToolResult{Success: false, Error: fmt.Sprintf("unknown teacher tool: %s", name)}
	}
}

// RunTeacher runs the teacher tool-calling loop with retry and timeout.
func (co *ChatOrchestrator) RunTeacher(ctx context.Context, history []llm.ChatMessage, userMessage string, ttctx *TeacherToolContext, systemPrompt string) (*llm.ChatResponse, error) {
	if co.client == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}

	tools := TeacherToolSchemas()
	messages := []llm.ChatMessage{llm.NewTextMessage("system", systemPrompt)}
	messages = append(messages, history...)
	messages = append(messages, llm.NewTextMessage("user", userMessage))

	consecutiveFailedRounds := 0

	maxRounds := co.maxToolRounds
	if maxRounds <= 0 {
		maxRounds = MaxToolRounds
	}
	for round := 0; round < maxRounds; round++ {
		var resp *llm.ChatResponse
		var err error

		for retry := 0; retry <= llmMaxRetries; retry++ {
			resp, err = co.client.Complete(ctx, messages, tools)
			if err == nil {
				break
			}
			if retry < llmMaxRetries {
				delay := llmRetryBaseDelay * time.Duration(1<<uint(retry))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("LLM call failed after %d attempts: %w", llmMaxRetries+1, err)
		}

		if len(resp.Choices) == 0 {
			return resp, nil
		}

choice := resp.Choices[0]
			if len(choice.Message.ToolCalls) == 0 {
				return resp, nil
			}

			// MUST append the assistant tool_call message BEFORE tool result messages
			// (OpenAI chat protocol: every tool result must follow the assistant message carrying the tool_calls).
			messages = append(messages, llm.ChatMessage{
				Role:       "assistant",
				ToolCalls:  choice.Message.ToolCalls,
			})

			// Process tool calls
			allFailed := true
			for _, tc := range choice.Message.ToolCalls {
			// Fire per-request callback for SSE progress events
			cb := ttctx.OnToolCall
			if cb == nil {
				cb = co.OnToolCall
			}
			if cb != nil {
				cb(tc.Function.Name)
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{}
			}

			toolCtx, cancel := context.WithTimeout(ctx, toolCallTimeout)
			result := co.DispatchTeacherTool(toolCtx, tc.Function.Name, args, ttctx)
			cancel()

			resultJSON, _ := json.Marshal(result)
			messages = append(messages, llm.ChatMessage{
				Role:       "tool",
				Content:    string(resultJSON),
				ToolCallID: tc.ID,
			})

			if result.Success {
				allFailed = false
			}
		}

		if allFailed {
			consecutiveFailedRounds++
			if consecutiveFailedRounds >= maxConsecutiveFailedRounds {
				slog.Warn("teacher tool loop: too many consecutive failures, aborting")
				break
			}
		} else {
			consecutiveFailedRounds = 0
		}
	}

	// Final call without tools
	resp, err := co.client.Complete(ctx, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("final LLM call failed: %w", err)
	}
	return resp, nil
}

// ============================================================
// T3.3 — Task/Class/Submission Statistics
// ============================================================

func (co *ChatOrchestrator) teacherGetTaskSummary(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this task"}
	}

	uploads, _, _ := co.uploadRepo.List(ctx, repository.UploadListParams{TaskID: &taskID})
	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID})

	var scored, confirmed int
	var sum float64
	var count int
	for _, e := range evals {
		switch e.Status {
		case "scored":
			scored++
		case "confirmed":
			confirmed++
		}
		if e.TotalScore != nil {
			sum += *e.TotalScore
			count++
		}
	}

	var avg float64
	if count > 0 {
		avg = sum / float64(count)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"submitted":     len(uploads),
			"scored":        scored,
			"confirmed":     confirmed,
			"average_score": avg,
		},
	}
}

func (co *ChatOrchestrator) teacherListPendingSubmissions(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this task"}
	}

	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID})

	type pendingItem struct {
		EvalID    int64   `json:"evaluation_id"`
		StudentID int64   `json:"student_id"`
		Score     float64 `json:"total_score"`
		Status    string  `json:"status"`
	}

	var items []pendingItem
	for _, e := range evals {
		if e.Status == "scored" {
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
	classID := toInt64(args["class_id"])
	if co.classRepo == nil {
		return &ToolResult{Success: false, Error: "class repository not available"}
	}
	cls, err := co.classRepo.GetByID(ctx, classID)
	if err != nil {
		return &ToolResult{Success: false, Error: "class not found"}
	}
	if cls.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this class"}
	}

	members, _ := co.classRepo.GetMembers(ctx, classID)
	if len(members) == 0 {
		return &ToolResult{
			Success: true,
			Data: map[string]any{
				"student_count": 0,
				"message":       "该班级暂无学生，无法生成表现报告。",
			},
		}
	}

	// Collect evaluations for all students in the class
	// Scope to teacher-owned tasks only (prevents cross-teacher data leakage)
	teacherTasks, _, _ := co.taskRepo.List(ctx, repository.TaskListParams{TeacherID: &ttctx.TeacherID})
	taskOwned := make(map[int64]bool, len(teacherTasks))
	for _, t := range teacherTasks {
		taskOwned[t.ID] = true
	}
	type studentPerf struct {
		StudentID int64   `json:"student_id"`
		EvalCount int     `json:"eval_count"`
		AvgScore  float64 `json:"average_score"`
	}
	var perfs []studentPerf
	for _, m := range members {
		evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{StudentID: &m.StudentID})
		var sum float64
		var count int
		for _, e := range evals {
			// Only count evaluations for teacher-owned tasks
			if !taskOwned[e.TaskID] {
				continue
			}
			if e.TotalScore != nil {
				sum += *e.TotalScore
				count++
			}
		}
		avg := 0.0
		if count > 0 {
			avg = sum / float64(count)
		}
		perfs = append(perfs, studentPerf{
			StudentID: m.StudentID,
			EvalCount: count,
			AvgScore:  math.Round(avg*10) / 10,
		})
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"student_count": len(members),
			"students":      perfs,
		},
	}
}

func (co *ChatOrchestrator) teacherGetDimensionDistribution(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this task"}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, taskID)
	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID})

	type dimDist struct {
		Name    string  `json:"name"`
		Average float64 `json:"average"`
		Count   int     `json:"count"`
		Min     float64 `json:"min"`
		Max     float64 `json:"max"`
	}

	var distributions []dimDist
	for _, d := range dims {
		var scores []float64
		for _, e := range evals {
			for _, s := range e.Scores {
				if s.DimensionID == d.ID {
					// Teacher score takes precedence over AI score
					if s.TeacherScore != nil {
						scores = append(scores, *s.TeacherScore)
					} else if s.AIScore != nil {
						scores = append(scores, *s.AIScore)
					}
				}
			}
		}

		dist := dimDist{Name: d.Name}
		if len(scores) > 0 {
			sum := 0.0
			minVal := scores[0]
			maxVal := scores[0]
			for _, v := range scores {
				sum += v
				if v < minVal {
					minVal = v
				}
				if v > maxVal {
					maxVal = v
				}
			}
			dist.Average = math.Round(sum/float64(len(scores))*10) / 10
			dist.Count = len(scores)
			dist.Min = minVal
			dist.Max = maxVal
		}
		distributions = append(distributions, dist)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"dimensions": distributions,
		},
	}
}

// ============================================================
// T3.4 — Assisted Grading & Feedback Drafts
// ============================================================

// loadEvalWithOwnership loads an evaluation and verifies teacher ownership via the task.
func (co *ChatOrchestrator) loadEvalWithOwnership(ctx context.Context, evalID int64, teacherID int64) (*model.Evaluation, *model.TrainingTask, error) {
	eval, err := co.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return nil, nil, fmt.Errorf("evaluation not found")
	}
	task, err := co.taskRepo.GetByID(ctx, eval.TaskID)
	if err != nil {
		return nil, nil, fmt.Errorf("task not found")
	}
	if task.TeacherID != teacherID {
		return nil, nil, fmt.Errorf("forbidden: you do not own this evaluation")
	}
	return eval, task, nil
}

// fetchUploadSummary returns a truncated summary of the upload content.
func (co *ChatOrchestrator) fetchUploadSummary(ctx context.Context, uploadID int64) (string, string) {
	upload, err := co.uploadRepo.GetByID(ctx, uploadID)
	if err != nil || upload == nil {
		return "", "上传文件信息不可用。"
	}

	pr, err := co.uploadRepo.GetParseResult(ctx, uploadID)
	if err != nil || pr == nil {
		return fmt.Sprintf("文件: %s (%d bytes)", upload.Filename, upload.FileSize), "解析结果不可用。"
	}

	summary := fmt.Sprintf("文件: %s (%d bytes)", upload.Filename, upload.FileSize)
	text := truncateStr(pr.RawText, 500)
	return summary, text
}

// confirmedWarning returns a warning string if the evaluation is confirmed.
func confirmedWarning(status string) string {
	if status == "confirmed" {
		return "此评价已确认，修改后需要重新确认。"
	}
	return ""
}

func (co *ChatOrchestrator) teacherGetEvaluationDetail(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	evalID := toInt64(args["evaluation_id"])
	eval, task, err := co.loadEvalWithOwnership(ctx, evalID, ttctx.TeacherID)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, task.ID)
	uploadSummary, parseNote := co.fetchUploadSummary(ctx, eval.UploadID)

	// Build dimension details
	type dimInfo struct {
		Name         string   `json:"name"`
		AIScore      *float64 `json:"ai_score,omitempty"`
		TeacherScore *float64 `json:"teacher_score,omitempty"`
		Rationale    string   `json:"rationale,omitempty"`
	}
	var dimDetails []dimInfo
	dimMap := make(map[int64]string)
	for _, d := range dims {
		dimMap[d.ID] = d.Name
	}
	for _, s := range eval.Scores {
		dimDetails = append(dimDetails, dimInfo{
			Name:         dimMap[s.DimensionID],
			AIScore:      s.AIScore,
			TeacherScore: s.TeacherScore,
			Rationale:    s.Rationale,
		})
	}

	data := map[string]any{
		"status":         eval.Status,
		"total_score":    eval.TotalScore,
		"scores":         dimDetails,
		"upload_summary": uploadSummary,
		"task_name":      task.Name,
	}

	if cw := confirmedWarning(eval.Status); cw != "" {
		data["confirmed_warning"] = cw
	}

	if parseNote != "" {
		// Check if parse result was unavailable
		if strings.Contains(parseNote, "不可用") {
			data["parse_result_note"] = parseNote
		}
	}

	return &ToolResult{Success: true, Data: data}
}

func (co *ChatOrchestrator) teacherGenerateFeedbackDraft(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	evalID := toInt64(args["evaluation_id"])
	eval, task, err := co.loadEvalWithOwnership(ctx, evalID, ttctx.TeacherID)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, task.ID)
	_, parseContent := co.fetchUploadSummary(ctx, eval.UploadID)

	// Build draft text
	var sb strings.Builder
	sb.WriteString("【反馈评语草稿 — 仅供教师审阅确认】\n\n")

	if cw := confirmedWarning(eval.Status); cw != "" {
		sb.WriteString("⚠ " + cw + "（此评价已确认，请注意）\n\n")
	}

	sb.WriteString(fmt.Sprintf("任务：%s\n", task.Name))
	if eval.TotalScore != nil {
		sb.WriteString(fmt.Sprintf("总分：%.1f\n\n", *eval.TotalScore))
	}

	// Dimension feedback
	dimMap := make(map[int64]model.Dimension)
	for _, d := range dims {
		dimMap[d.ID] = d
	}
	for _, s := range eval.Scores {
		d, ok := dimMap[s.DimensionID]
		if !ok {
			continue
		}
		score := safeFloat(s.TeacherScore, s.AIScore)
		sb.WriteString(fmt.Sprintf("- %s (%.1f分): ", d.Name, score))
		if s.Rationale != "" {
			sb.WriteString(s.Rationale)
		} else {
			sb.WriteString("无详细评语")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Parse content summary
	if parseContent != "" && !strings.Contains(parseContent, "不可用") {
		sb.WriteString("学生提交内容摘要：\n")
		sb.WriteString(truncateStr(parseContent, 300) + "\n\n")
	} else {
		sb.WriteString("注：解析结果不可用，无法展示学生提交内容摘要。\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("以上为 AI 生成的草稿，需教师确认后方可发送给学生。\n")

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"draft":    sb.String(),
			"is_draft": true,
		},
	}
}

func (co *ChatOrchestrator) teacherSuggestScoreReview(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	evalID := toInt64(args["evaluation_id"])
	eval, task, err := co.loadEvalWithOwnership(ctx, evalID, ttctx.TeacherID)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, task.ID)
	dimMap := make(map[int64]model.Dimension)
	for _, d := range dims {
		dimMap[d.ID] = d
	}

	type reviewItem struct {
		DimensionName string  `json:"dimension_name"`
		AIScore       float64 `json:"ai_score"`
		Reason        string  `json:"reason"`
	}

	var reviews []reviewItem

	// Calculate overall average for comparison
	var allScores []float64
	for _, s := range eval.Scores {
		if s.AIScore != nil {
			allScores = append(allScores, *s.AIScore)
		}
	}
	avg := 0.0
	if len(allScores) > 0 {
		sum := 0.0
		for _, v := range allScores {
			sum += v
		}
		avg = sum / float64(len(allScores))
	}

	for _, s := range eval.Scores {
		if s.AIScore == nil {
			continue
		}
		dim := dimMap[s.DimensionID]
		score := *s.AIScore

		// Flag dimensions with very low scores (< 40) or large deviation from average
		var reasons []string
		if score < 40 {
			reasons = append(reasons, fmt.Sprintf("AI 评分较低 (%.0f < 40)，建议复核", score))
		}
		if len(allScores) > 1 && math.Abs(score-avg) > 25 {
			reasons = append(reasons, fmt.Sprintf("与平均分偏差较大 (%.0f vs 均值 %.0f)", score, avg))
		}

		if len(reasons) > 0 {
			reviews = append(reviews, reviewItem{
				DimensionName: dim.Name,
				AIScore:       score,
				Reason:        strings.Join(reasons, "; "),
			})
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"reviews":          reviews,
			"review_suggested": len(reviews),
			"note":             "以上为 AI 建议，不自动改分，需教师手动确认是否调整。",
		},
	}
}

func (co *ChatOrchestrator) teacherCompareWithRubric(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	evalID := toInt64(args["evaluation_id"])
	eval, task, err := co.loadEvalWithOwnership(ctx, evalID, ttctx.TeacherID)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, task.ID)
	dimMap := make(map[int64]model.Dimension)
	for _, d := range dims {
		dimMap[d.ID] = d
	}

	type criteriaItem struct {
		Dimension   string  `json:"dimension"`
		Description string  `json:"description"`
		Score       float64 `json:"score"`
		Level       string  `json:"level"`
	}

	var items []criteriaItem
	for _, s := range eval.Scores {
		dim := dimMap[s.DimensionID]
		score := safeFloat(s.TeacherScore, s.AIScore)

		// Simple rubric level classification
		var level string
		switch {
		case score >= 90:
			level = "优秀"
		case score >= 75:
			level = "良好"
		case score >= 60:
			level = "及格"
		default:
			level = "不及格"
		}

		items = append(items, criteriaItem{
			Dimension:   dim.Name,
			Description: dim.Description,
			Score:       score,
			Level:       level,
		})
	}

	// Sort by dimension name for stable output
	sort.Slice(items, func(i, j int) bool {
		return items[i].Dimension < items[j].Dimension
	})

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"criteria_comparison": items,
			"task_criteria":       task.EvaluationCriteria,
			"note":                "以上为 AI 对比结果，不自动确认，需教师手动确认评价。",
		},
	}
}

// ============================================================
// T3.5 — Similarity Explanation & Teaching Suggestions
// ============================================================

func (co *ChatOrchestrator) teacherGetSimilaritySummary(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this task"}
	}
	if co.simRepo == nil {
		return &ToolResult{Success: false, Error: "similarity repository not available"}
	}

	records, err := co.simRepo.List(ctx, taskID, nil)
	if err != nil {
		return &ToolResult{Success: false, Error: "failed to list similarity records"}
	}

	var suspect, confirmed, ignored int
	for _, r := range records {
		switch r.State {
		case "suspect":
			suspect++
		case "confirmed":
			confirmed++
		case "ignored":
			ignored++
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"total":     len(records),
			"suspect":   suspect,
			"confirmed": confirmed,
			"ignored":   ignored,
			"note":      "疑似记录仅供参考，需教师人工复核后方可确认。",
		},
	}
}

func (co *ChatOrchestrator) teacherExplainSimilarityCase(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	recordID := toInt64(args["record_id"])
	if co.simRepo == nil {
		return &ToolResult{Success: false, Error: "similarity repository not available"}
	}

	record, err := co.simRepo.GetByID(ctx, recordID)
	if err != nil {
		return &ToolResult{Success: false, Error: "similarity record not found"}
	}

	// Verify teacher owns the task this record belongs to
	task, err := co.taskRepo.GetByID(ctx, record.TaskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this task"}
	}

	// Build state label (use "疑似" not definitive language)
	stateLabel := "未知"
	switch record.State {
	case "suspect":
		stateLabel = "疑似"
	case "confirmed":
		stateLabel = "已确认"
	case "ignored":
		stateLabel = "已忽略"
	}

	data := map[string]any{
		"record_id":  record.ID,
		"task_id":    record.TaskID,
		"state":      stateLabel,
		"upload_a":   record.UploadAID,
		"upload_b":   record.UploadBID,
		"created_at": record.CreatedAt.Format(time.RFC3339),
	}
	if record.HammingDistance != nil {
		data["hamming_distance"] = *record.HammingDistance
	}
	if record.CosineSimilarity != nil {
		data["cosine_similarity"] = math.Round(*record.CosineSimilarity*1000) / 1000
	}

	data["note"] = "查重结果仅供参考，不自动定性作弊，需教师结合实际情况复核。"

	return &ToolResult{Success: true, Data: data}
}

func (co *ChatOrchestrator) teacherGenerateTeachingSuggestions(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this task"}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, taskID)
	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID})

	// Calculate per-dimension averages to identify weak areas
	type dimStat struct {
		Name   string
		Avg    float64
		Count  int
		IsWeak bool
	}
	var stats []dimStat
	for _, d := range dims {
		var scores []float64
		for _, e := range evals {
			for _, s := range e.Scores {
				if s.DimensionID == d.ID {
					scores = append(scores, safeFloat(s.TeacherScore, s.AIScore))
				}
			}
		}
		avg := 0.0
		if len(scores) > 0 {
			sum := 0.0
			for _, v := range scores {
				sum += v
			}
			avg = sum / float64(len(scores))
		}
		stats = append(stats, dimStat{Name: d.Name, Avg: math.Round(avg*10) / 10, Count: len(scores), IsWeak: avg < 60 && len(scores) > 0})
	}

	// Build suggestions
	var sb strings.Builder
	sb.WriteString("【教学改进建议 — 仅供参考，需教师确认】\n\n")
	sb.WriteString(fmt.Sprintf("任务：%s\n", task.Name))
	sb.WriteString(fmt.Sprintf("评价数：%d\n\n", len(evals)))

	weakCount := 0
	for _, s := range stats {
		if s.IsWeak {
			weakCount++
			sb.WriteString(fmt.Sprintf("⚠ 薄弱维度「%s」(均分 %.1f，%d 人)\n", s.Name, s.Avg, s.Count))
			sb.WriteString(fmt.Sprintf("  建议：加强该维度的教学指导和练习，可在后续任务中增设相关训练环节。\n\n"))
		}
	}

	if weakCount == 0 {
		sb.WriteString("当前各维度均分均在及格线以上，整体表现良好。\n")
		sb.WriteString("建议：可适当提高评分标准或增加挑战性任务以进一步提升学生能力。\n")
	}

	sb.WriteString("\n---\n")
	sb.WriteString("以上为 AI 生成的教学建议草稿，需教师结合实际教学情况确认。\n")

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"suggestions":     sb.String(),
			"weak_dimensions": weakCount,
			"is_draft":        true,
		},
	}
}

// ============================================================
// T3.6 — Task & Dimension Rubric Draft Generation
// ============================================================

func (co *ChatOrchestrator) teacherGenerateTaskDraft(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	brief, _ := args["brief"].(string)

	var sb strings.Builder
	sb.WriteString("【新实训任务草稿 — 仅供教师审阅，不会自动创建】\n\n")

	if brief != "" {
		sb.WriteString(fmt.Sprintf("任务简述：%s\n\n", brief))
	}

	sb.WriteString("建议任务名称：（请根据任务简述填写）\n")
	sb.WriteString("建议任务描述：（请补充详细背景和目标）\n")
	sb.WriteString("建议任务要求：\n")
	sb.WriteString("  1. 提交符合规范的文档\n")
	sb.WriteString("  2. 内容完整、逻辑清晰\n")
	sb.WriteString("  3. 体现专业技能和分析能力\n\n")

	sb.WriteString("建议评分维度（示例）：\n")
	sb.WriteString("  - 内容完整性 (25%%)\n")
	sb.WriteString("  - 逻辑结构 (20%%)\n")
	sb.WriteString("  - 专业准确性 (30%%)\n")
	sb.WriteString("  - 创新性 (15%%)\n")
	sb.WriteString("  - 格式规范 (10%%)\n\n")

	sb.WriteString("---\n")
	sb.WriteString("以上为 AI 生成的任务草稿，需教师在页面上手动创建任务。\n")

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"draft":    sb.String(),
			"is_draft": true,
		},
	}
}

func (co *ChatOrchestrator) teacherGenerateDimensionRubricDraft(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this task"}
	}

	existingDims, _ := co.taskRepo.GetDimensions(ctx, taskID)

	var sb strings.Builder
	sb.WriteString("【评分维度草稿 — 仅供教师审阅，不会自动替换现有维度】\n\n")
	sb.WriteString(fmt.Sprintf("任务：%s\n", task.Name))

	if len(existingDims) > 0 {
		sb.WriteString(fmt.Sprintf("\n当前已有 %d 个评分维度：\n", len(existingDims)))
		for _, d := range existingDims {
			sb.WriteString(fmt.Sprintf("  - %s (权重 %d%%)\n", d.Name, d.Weight))
		}
		sb.WriteString("\n")
	}

	// Generate draft dimensions that sum to 100
	type draftDim struct {
		Name     string `json:"name"`
		Weight   int    `json:"weight"`
		Criteria string `json:"criteria"`
	}
	drafts := []draftDim{
		{Name: "内容完整性", Weight: 25, Criteria: "提交内容涵盖所有要求，不遗漏关键部分"},
		{Name: "逻辑结构", Weight: 20, Criteria: "论述条理清晰，层次分明，过渡自然"},
		{Name: "专业准确性", Weight: 30, Criteria: "专业术语使用正确，分析方法合理，结论有据"},
		{Name: "创新性", Weight: 15, Criteria: "体现独立思考，有独到见解或创新方法"},
		{Name: "格式规范", Weight: 10, Criteria: "排版整洁，引用规范，符合格式要求"},
	}

	sb.WriteString("建议评分维度（权重合计 100%%）：\n")
	totalWeight := 0
	for _, d := range drafts {
		totalWeight += d.Weight
		sb.WriteString(fmt.Sprintf("\n  维度：%s (权重 %d%%)\n", d.Name, d.Weight))
		sb.WriteString(fmt.Sprintf("  评分标准：%s\n", d.Criteria))
	}
	sb.WriteString(fmt.Sprintf("\n  权重合计：%d%%\n", totalWeight))

	sb.WriteString("\n---\n")
	sb.WriteString("以上为 AI 生成的维度草稿，需教师在页面上手动配置评分维度。\n")

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"draft":            sb.String(),
			"draft_dimensions": drafts,
			"total_weight":     totalWeight,
			"is_draft":         true,
		},
	}
}

func (co *ChatOrchestrator) teacherGenerateReportOutlineDraft(ctx context.Context, args map[string]any, ttctx *TeacherToolContext) *ToolResult {
	taskID := toInt64(args["task_id"])
	task, err := co.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}
	if task.TeacherID != ttctx.TeacherID {
		return &ToolResult{Success: false, Error: "forbidden: you do not own this task"}
	}

	dims, _ := co.taskRepo.GetDimensions(ctx, taskID)
	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{TaskID: &taskID})
	uploads, _, _ := co.uploadRepo.List(ctx, repository.UploadListParams{TaskID: &taskID})

	// Calculate overall stats
	var scored, confirmed int
	var sum float64
	var count int
	for _, e := range evals {
		switch e.Status {
		case "scored":
			scored++
		case "confirmed":
			confirmed++
		}
		if e.TotalScore != nil {
			sum += *e.TotalScore
			count++
		}
	}
	avg := 0.0
	if count > 0 {
		avg = math.Round(sum/float64(count)*10) / 10
	}

	var sb strings.Builder
	sb.WriteString("【评价报告大纲草稿 — 仅供教师参考，不会自动写入报告】\n\n")
	sb.WriteString(fmt.Sprintf("任务：%s\n", task.Name))
	sb.WriteString(fmt.Sprintf("状态：%s\n\n", task.Status))

	sb.WriteString("一、总体概况\n")
	sb.WriteString(fmt.Sprintf("  提交数：%d\n", len(uploads)))
	sb.WriteString(fmt.Sprintf("  已评分：%d\n", scored))
	sb.WriteString(fmt.Sprintf("  已确认：%d\n", confirmed))
	sb.WriteString(fmt.Sprintf("  平均分：%.1f\n\n", avg))

	sb.WriteString("二、各维度分析\n")
	for _, d := range dims {
		var scores []float64
		for _, e := range evals {
			for _, s := range e.Scores {
				if s.DimensionID == d.ID {
					scores = append(scores, safeFloat(s.TeacherScore, s.AIScore))
				}
			}
		}
		dimAvg := 0.0
		if len(scores) > 0 {
			s := 0.0
			for _, v := range scores {
				s += v
			}
			dimAvg = math.Round(s/float64(len(scores))*10) / 10
		}
		sb.WriteString(fmt.Sprintf("  - %s：均分 %.1f（%d 人）\n", d.Name, dimAvg, len(scores)))
	}

	sb.WriteString("\n三、薄弱维度与改进方向\n")
	sb.WriteString("  （请根据上述数据填写薄弱维度分析及改进建议）\n")

	sb.WriteString("\n四、优秀学生案例\n")
	sb.WriteString("  （请选取高分学生作为示范案例）\n")

	sb.WriteString("\n五、总结与建议\n")
	sb.WriteString("  （请根据整体情况撰写总结和未来教学建议）\n")

	sb.WriteString("\n---\n")
	sb.WriteString("以上为 AI 生成的报告大纲草稿，需教师确认后手动生成正式报告。\n")

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"outline":  sb.String(),
			"is_draft": true,
		},
	}
}

// ============================================================
// Helpers
// ============================================================

// toInt64 converts a JSON number (float64) to int64.
func toInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	default:
		return 0
	}
}

// safeFloat returns the teacher score if available, otherwise the AI score, otherwise 0.
func safeFloat(teacher, ai *float64) float64 {
	if teacher != nil {
		return *teacher
	}
	if ai != nil {
		return *ai
	}
	return 0
}

// truncateStr truncates a string to maxLen runes.
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
