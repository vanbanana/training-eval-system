// Package pipeline — Admin tools for the AI-assisted evaluation system (T4.1–T4.4).
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// AdminToolContext holds context for admin tool calls.
type AdminToolContext struct {
	AdminID int64
	// OnToolCall is an optional per-request callback for SSE progress events.
	OnToolCall func(toolName string)
}

// SetUserRepo sets the optional UserRepo on the orchestrator (for admin tools).
func (co *ChatOrchestrator) SetUserRepo(r repository.UserRepo) { co.userRepo = r }

// SetLLMConfigRepo sets the optional LLMConfigRepo on the orchestrator (for admin tools).
func (co *ChatOrchestrator) SetLLMConfigRepo(r repository.LLMConfigRepo) { co.llmConfigRepo = r }

// SetAuditRepo sets the optional AuditRepo on the orchestrator (for admin tools).
func (co *ChatOrchestrator) SetAuditRepo(r repository.AuditRepo) { co.auditRepo = r }

// SetUsageRepo sets the optional UsageRepo on the orchestrator (for admin tools, T8.3).
func (co *ChatOrchestrator) SetUsageRepo(r repository.UsageRepo) { co.usageRepo = r }

// adminToolRequiredParams maps admin tool names to their required parameter names.
var adminToolRequiredParams = map[string][]string{
	// T4.2
	"admin_get_system_overview": {},
	"admin_get_usage_metrics":   {},
	"admin_check_llm_status":    {},
	"admin_get_recent_failures": {},
	// T4.3
	"admin_get_user_summary":                {},
	"admin_find_inactive_users":             {},
	"admin_get_course_class_summary":        {},
	"admin_generate_governance_suggestions": {},
	// T4.4
	"admin_search_audit_logs":         {},
	"admin_summarize_audit_anomalies": {},
	"admin_explain_user_activity":     {"user_id"},
	// T8.3
	"admin_get_ai_usage_summary": {},
}

// AdminToolSchemas returns all 12 registered admin tools as OpenAI function definitions.
func AdminToolSchemas() []llm.Tool {
	return []llm.Tool{
		// T4.2 — System overview & health check
		makeTool("admin_get_system_overview",
			"获取系统总览信息，包括用户数、任务数、评价数、角色分布等聚合统计。",
			map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		makeTool("admin_get_usage_metrics",
			"获取系统使用指标，包括上传数、评价数、AI 调用量等。",
			map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		makeTool("admin_check_llm_status",
			"检查 LLM 配置状态，包括是否已配置、是否激活（不会返回 API Key）。",
			map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		makeTool("admin_get_recent_failures",
			"获取最近的失败记录摘要，包括解析失败、导入失败等（不返回敏感 payload）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer", "description": "返回条数限制，默认 20，最大 50"},
				},
			}),
		// T4.3 — User/Course/Class governance
		makeTool("admin_get_user_summary",
			"获取用户统计摘要，包括各角色数量、活跃/停用账号数。",
			map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		makeTool("admin_find_inactive_users",
			"查找长时间未登录的用户，用于系统治理建议（不会自动停用用户）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"days":  map[string]any{"type": "integer", "description": "未登录天数阈值，默认 30"},
					"limit": map[string]any{"type": "integer", "description": "返回条数限制，默认 20"},
				},
			}),
		makeTool("admin_get_course_class_summary",
			"获取课程和班级的汇总统计信息。",
			map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		makeTool("admin_generate_governance_suggestions",
			"基于系统状态生成治理改进建议（仅供参考，不会自动执行任何操作）。",
			map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		// T4.4 — Audit log explanation
		makeTool("admin_search_audit_logs",
			"搜索审计日志，支持按动作类型和用户筛选。参数 days 控制搜索天数范围（默认 7，最大 90）；行动作和用户无筛选时返回近期日志。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action":  map[string]any{"type": "string", "description": "筛选动作类型（可选）"},
					"user_id": map[string]any{"type": "integer", "description": "筛选用户ID（可选）"},
					"days":    map[string]any{"type": "integer", "description": "搜索天数范围，默认 7，最大 90"},
					"limit":   map[string]any{"type": "integer", "description": "返回条数限制，默认 20，最大 50"},
				},
			}),
		makeTool("admin_summarize_audit_anomalies",
			"分析审计日志中的异常模式，如频繁登录失败等（仅描述异常，不自动定性恶意行为）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"days": map[string]any{"type": "integer", "description": "分析天数范围，默认 7，最大 90"},
				},
			}),
		makeTool("admin_explain_user_activity",
			"查看指定用户的近期活动记录，帮助管理员了解用户行为。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user_id": map[string]any{"type": "integer", "description": "用户ID"},
					"limit":   map[string]any{"type": "integer", "description": "返回条数限制，默认 20"},
				},
				"required": []string{"user_id"},
			}),
		// T8.3 — AI usage summary
		makeTool("admin_get_ai_usage_summary",
			"获取 AI 调用统计摘要，包括总调用次数、Token 用量、按角色分布、高用量用户等（不返回 API Key 或用户输入内容）。",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"days": map[string]any{"type": "integer", "description": "查询天数范围，默认 1 天，最大 90"},
				},
			}),
	}
}

// DispatchAdminTool dispatches an admin tool call by name with argument validation.
func (co *ChatOrchestrator) DispatchAdminTool(ctx context.Context, name string, args map[string]any, actx *AdminToolContext) *ToolResult {
	// Validate required parameters
	if required, ok := adminToolRequiredParams[name]; ok {
		for _, param := range required {
			if v, exists := args[param]; !exists || v == nil {
				return &ToolResult{Success: false, Error: fmt.Sprintf("missing required parameter: %s", param)}
			}
		}
	}

	switch name {
	// T4.2
	case "admin_get_system_overview":
		return co.adminGetSystemOverview(ctx, args, actx)
	case "admin_get_usage_metrics":
		return co.adminGetUsageMetrics(ctx, args, actx)
	case "admin_check_llm_status":
		return co.adminCheckLLMStatus(ctx, args, actx)
	case "admin_get_recent_failures":
		return co.adminGetRecentFailures(ctx, args, actx)
	// T4.3
	case "admin_get_user_summary":
		return co.adminGetUserSummary(ctx, args, actx)
	case "admin_find_inactive_users":
		return co.adminFindInactiveUsers(ctx, args, actx)
	case "admin_get_course_class_summary":
		return co.adminGetCourseClassSummary(ctx, args, actx)
	case "admin_generate_governance_suggestions":
		return co.adminGenerateGovernanceSuggestions(ctx, args, actx)
	// T4.4
	case "admin_search_audit_logs":
		return co.adminSearchAuditLogs(ctx, args, actx)
	case "admin_summarize_audit_anomalies":
		return co.adminSummarizeAuditAnomalies(ctx, args, actx)
	case "admin_explain_user_activity":
		return co.adminExplainUserActivity(ctx, args, actx)
	// T8.3
	case "admin_get_ai_usage_summary":
		return co.adminGetAIUsageSummary(ctx, args, actx)
	default:
		return &ToolResult{Success: false, Error: fmt.Sprintf("unknown admin tool: %s", name)}
	}
}

// RunAdmin runs the admin tool-calling loop with retry and timeout.
func (co *ChatOrchestrator) RunAdmin(ctx context.Context, history []llm.ChatMessage, userMessage string, actx *AdminToolContext, systemPrompt string) (*llm.ChatResponse, error) {
	if co.client == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}

	tools := AdminToolSchemas()
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
			cb := actx.OnToolCall
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
			result := co.DispatchAdminTool(toolCtx, tc.Function.Name, args, actx)
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
				slog.Warn("admin tool loop: too many consecutive failures, aborting")
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
// T4.2 — System Overview & Health Check
// ============================================================

func (co *ChatOrchestrator) adminGetSystemOverview(ctx context.Context, _ map[string]any, _ *AdminToolContext) *ToolResult {
// User stats
		var adminCount, teacherCount, studentCount int
		if co.userRepo != nil {
			// Paginate through all users to count by role (avoids PageSize ceiling issues)
			const pageSize = 500
			for page := 1; ; page++ {
				batch, _, err := co.userRepo.List(ctx, repository.ListParams{Page: page, PageSize: pageSize})
				if err != nil || len(batch) == 0 {
					break
				}
				for _, u := range batch {
					switch u.Role {
					case "admin":
						adminCount++
					case "teacher":
						teacherCount++
					case "student":
						studentCount++
					}
				}
				if len(batch) < pageSize {
					break
				}
			}
		}

		// Task stats
		_, taskTotal, _ := co.taskRepo.List(ctx, repository.TaskListParams{ListParams: repository.ListParams{Page: 1, PageSize: 1}})

		// Eval stats
		_, evalTotal, _ := co.evalRepo.List(ctx, repository.EvalListParams{ListParams: repository.ListParams{Page: 1, PageSize: 1}})

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"user_count":    adminCount + teacherCount + studentCount,
			"admin_count":   adminCount,
			"teacher_count": teacherCount,
			"student_count": studentCount,
			"task_count":    taskTotal,
			"eval_count":    evalTotal,
		},
	}
}

func (co *ChatOrchestrator) adminGetUsageMetrics(ctx context.Context, _ map[string]any, _ *AdminToolContext) *ToolResult {
	_, uploadTotal, _ := co.uploadRepo.List(ctx, repository.UploadListParams{ListParams: repository.ListParams{Page: 1, PageSize: 1}})
	_, evalTotal, _ := co.evalRepo.List(ctx, repository.EvalListParams{ListParams: repository.ListParams{Page: 1, PageSize: 1}})

	// Count confirmed evaluations
	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{ListParams: repository.ListParams{Page: 1, PageSize: 10000}})
	var confirmedCount int
	for _, e := range evals {
		if e.Status == "confirmed" {
			confirmedCount++
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"upload_count":    uploadTotal,
			"eval_count":      evalTotal,
			"confirmed_count": confirmedCount,
		},
	}
}

func (co *ChatOrchestrator) adminCheckLLMStatus(ctx context.Context, _ map[string]any, _ *AdminToolContext) *ToolResult {
	if co.llmConfigRepo == nil {
		return &ToolResult{
			Success: true,
			Data: map[string]any{
				"status":  "unknown",
				"message": "LLM 配置仓库不可用，请检查系统配置。",
			},
		}
	}

	config, err := co.llmConfigRepo.GetActive(ctx)
	if err != nil || config == nil {
		return &ToolResult{
			Success: true,
			Data: map[string]any{
				"status":  "missing",
				"message": "当前没有激活的 LLM 配置，请在系统设置中配置 LLM。",
			},
		}
	}

	// Return config info WITHOUT the API key
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"status":      "active",
			"provider":    config.Provider,
			"chat_model":  config.ChatModel,
			"embed_model": config.EmbedModel,
			"message":     "LLM 已配置且处于激活状态。API Key 出于安全原因不在此显示。",
		},
	}
}

func (co *ChatOrchestrator) adminGetRecentFailures(ctx context.Context, args map[string]any, _ *AdminToolContext) *ToolResult {
	limit := 20
	if v, ok := args["limit"]; ok {
		l := int(toInt64(v))
		if l > 0 && l <= 50 {
			limit = l
		}
	}

	if co.auditRepo == nil {
		return &ToolResult{
			Success: true,
			Data: map[string]any{
				"items":   []any{},
				"message": "审计日志仓库不可用。",
			},
		}
	}

	failureAction := "failure"
	logs, total, _ := co.auditRepo.List(ctx, repository.ListParams{Page: 1, PageSize: limit}, nil, &failureAction)

	type failItem struct {
		ID         int64  `json:"id"`
		OccurredAt string `json:"occurred_at"`
		Action     string `json:"action"`
		Username   string `json:"username"`
		Result     string `json:"result"`
		Detail     string `json:"detail,omitempty"`
	}

	var items []failItem
	for _, l := range logs {
		// Redact sensitive data — mask known secret patterns then truncate
detail := redactSensitive(l.Detail)
			detailRunes := []rune(detail)
			if len(detailRunes) > 200 {
				detail = string(detailRunes[:200]) + "..."
		}
		items = append(items, failItem{
			ID:         l.ID,
			OccurredAt: l.OccurredAt.Format(time.RFC3339),
			Action:     l.Action,
			Username:   l.Username,
			Result:     l.Result,
			Detail:     detail,
		})
	}

	truncated := total > int64(limit)
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"items":     items,
			"total":     total,
			"truncated": truncated,
		},
	}
}

// ============================================================
// T4.3 — User/Course/Class Governance
// ============================================================

func (co *ChatOrchestrator) adminGetUserSummary(ctx context.Context, _ map[string]any, _ *AdminToolContext) *ToolResult {
	if co.userRepo == nil {
		return &ToolResult{Success: false, Error: "user repository not available"}
	}

	users, total, _ := co.userRepo.List(ctx, repository.ListParams{Page: 1, PageSize: 10000})

	var adminCount, teacherCount, studentCount int
	var activeCount, inactiveCount int
	for _, u := range users {
		switch u.Role {
		case "admin":
			adminCount++
		case "teacher":
			teacherCount++
		case "student":
			studentCount++
		}
		if u.IsActive {
			activeCount++
		} else {
			inactiveCount++
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"total":          total,
			"admin_count":    adminCount,
			"teacher_count":  teacherCount,
			"student_count":  studentCount,
			"active_count":   activeCount,
			"inactive_count": inactiveCount,
			"note":           "不会自动执行任何用户管理操作，需管理员在页面确认。",
		},
	}
}

func (co *ChatOrchestrator) adminFindInactiveUsers(ctx context.Context, args map[string]any, _ *AdminToolContext) *ToolResult {
	if co.userRepo == nil {
		return &ToolResult{Success: false, Error: "user repository not available"}
	}

	days := 30
	if v, ok := args["days"]; ok {
		d := int(toInt64(v))
		if d > 0 {
			days = d
		}
	}
	limit := 20
	if v, ok := args["limit"]; ok {
		l := int(toInt64(v))
		if l > 0 && l <= 100 {
			limit = l
		}
	}

	threshold := time.Now().AddDate(0, 0, -days)
	users, _, _ := co.userRepo.List(ctx, repository.ListParams{Page: 1, PageSize: 10000})

	type inactiveUser struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		LastLoginAt string `json:"last_login_at,omitempty"`
	}

	var items []inactiveUser
	for _, u := range users {
		if u.LastLoginAt != nil && u.LastLoginAt.After(threshold) {
			continue
		}
		if u.LastLoginAt == nil && time.Since(u.CreatedAt) < time.Duration(days)*24*time.Hour {
			continue // recently created, never logged in but within threshold
		}
		item := inactiveUser{
			ID:          u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Role:        u.Role,
		}
		if u.LastLoginAt != nil {
			item.LastLoginAt = u.LastLoginAt.Format(time.RFC3339)
		}
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}

	truncated := len(items) >= limit
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"items":          items,
			"days_threshold": days,
			"truncated":      truncated,
			"note":           "不会自动停用用户，需管理员在页面手动操作。",
		},
	}
}

func (co *ChatOrchestrator) adminGetCourseClassSummary(ctx context.Context, _ map[string]any, _ *AdminToolContext) *ToolResult {
	data := map[string]any{}

	if co.courseRepo != nil {
		courses, courseTotal, _ := co.courseRepo.List(ctx, repository.ListParams{Page: 1, PageSize: 1})
		_ = courses
		data["course_count"] = courseTotal
	} else {
		data["course_count"] = "unavailable"
	}

	if co.classRepo != nil {
		// ClassRepo doesn't have a List method in the standard interface.
		// We return a note that detailed class listing requires specific course context.
		data["class_note"] = "班级统计需要按课程查看，请使用具体课程上下文。"
	} else {
		data["class_note"] = "班级仓库不可用。"
	}

	return &ToolResult{Success: true, Data: data}
}

func (co *ChatOrchestrator) adminGenerateGovernanceSuggestions(ctx context.Context, _ map[string]any, _ *AdminToolContext) *ToolResult {
	var sb strings.Builder
	sb.WriteString("【系统治理建议 — 仅供参考，不会自动执行任何操作】\n\n")

	// Check user stats
	if co.userRepo != nil {
		users, _, _ := co.userRepo.List(ctx, repository.ListParams{Page: 1, PageSize: 10000})
		var inactiveCount int
		for _, u := range users {
			if !u.IsActive {
				inactiveCount++
			}
		}
		if inactiveCount > 0 {
			sb.WriteString(fmt.Sprintf("1. 发现 %d 个停用账号，建议在页面中检查是否需要清理。\n\n", inactiveCount))
		}
	}

	// Check LLM config
	if co.llmConfigRepo != nil {
		config, err := co.llmConfigRepo.GetActive(ctx)
		if err != nil || config == nil {
			sb.WriteString("2. 当前没有激活的 LLM 配置，AI 功能将不可用。请在系统设置中配置 LLM。\n\n")
		}
	}

	// Check eval stats
	evals, _, _ := co.evalRepo.List(ctx, repository.EvalListParams{ListParams: repository.ListParams{Page: 1, PageSize: 10000}})
	var pendingCount int
	for _, e := range evals {
		if e.Status == "scored" {
			pendingCount++
		}
	}
	if pendingCount > 10 {
		sb.WriteString(fmt.Sprintf("3. 有 %d 条评价处于「已评分」状态等待教师确认，建议提醒相关教师处理。\n\n", pendingCount))
	}

	if sb.Len() < 80 {
		sb.WriteString("当前系统运行状态良好，暂无特别建议。\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("以上为 AI 生成的治理建议，所有操作需管理员在页面手动执行。\n")

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"suggestions": sb.String(),
			"is_draft":    true,
		},
	}
}

// ============================================================
// T4.4 — Audit Log Explanation
// ============================================================

// maxAuditDays is the maximum allowed audit log query range.
const maxAuditDays = 90

func (co *ChatOrchestrator) adminSearchAuditLogs(ctx context.Context, args map[string]any, _ *AdminToolContext) *ToolResult {
	if co.auditRepo == nil {
		return &ToolResult{Success: false, Error: "audit repository not available"}
	}

	limit := 20
	if v, ok := args["limit"]; ok {
		l := int(toInt64(v))
		if l > 0 && l <= 50 {
			limit = l
		}
	}

	var userIDPtr *int64
	if v, ok := args["user_id"]; ok {
		uid := toInt64(v)
		if uid > 0 {
			userIDPtr = &uid
		}
	}

	var actionPtr *string
	if v, ok := args["action"]; ok {
		if s, ok := v.(string); ok && s != "" {
			actionPtr = &s
		}
	}

	logs, total, _ := co.auditRepo.List(ctx, repository.ListParams{Page: 1, PageSize: limit}, userIDPtr, actionPtr)

	type logItem struct {
		ID         int64  `json:"id"`
		OccurredAt string `json:"occurred_at"`
		Username   string `json:"username"`
		Role       string `json:"role"`
		Action     string `json:"action"`
		Result     string `json:"result"`
		TargetType string `json:"target_type,omitempty"`
		ClientIP   string `json:"client_ip,omitempty"`
	}

	var items []logItem
	for _, l := range logs {
		// Redact sensitive fields — don't include payload or full detail
		items = append(items, logItem{
			ID:         l.ID,
			OccurredAt: l.OccurredAt.Format(time.RFC3339),
			Username:   l.Username,
			Role:       l.Role,
			Action:     l.Action,
			Result:     l.Result,
			TargetType: l.TargetType,
			ClientIP:   maskIP(l.ClientIP),
		})
	}

	truncated := total > int64(limit)
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"items":     items,
			"total":     total,
			"truncated": truncated,
		},
	}
}

func (co *ChatOrchestrator) adminSummarizeAuditAnomalies(ctx context.Context, args map[string]any, _ *AdminToolContext) *ToolResult {
	if co.auditRepo == nil {
		return &ToolResult{Success: false, Error: "audit repository not available"}
	}

	days := 7
	if v, ok := args["days"]; ok {
		d := int(toInt64(v))
		if d > maxAuditDays {
			d = maxAuditDays
		}
		if d > 0 {
			days = d
		}
	}

	// Fetch recent failure logs
	failureAction := "failure"
	logs, total, _ := co.auditRepo.List(ctx, repository.ListParams{Page: 1, PageSize: 200}, nil, &failureAction)

	// Count failures by username
	failCountByUsername := make(map[string]int)
	for _, l := range logs {
		failCountByUsername[l.Username]++
	}

	type anomalyItem struct {
		Username   string `json:"username"`
		FailCount  int    `json:"fail_count"`
		Suggestion string `json:"suggestion"`
	}

	var anomalies []anomalyItem
	for username, count := range failCountByUsername {
		if count >= 3 {
			anomalies = append(anomalies, anomalyItem{
				Username:   username,
				FailCount:  count,
				Suggestion: "频繁操作失败，建议检查该用户账号状态。",
			})
		}
	}

	var sb strings.Builder
	sb.WriteString("【审计异常摘要 — 仅供参考，不自动定性】\n\n")
	sb.WriteString(fmt.Sprintf("分析范围：最近 %d 天\n", days))
	sb.WriteString(fmt.Sprintf("失败日志总数：%d\n\n", total))

	if len(anomalies) > 0 {
		sb.WriteString("发现以下异常：\n")
		for _, a := range anomalies {
			sb.WriteString(fmt.Sprintf("  - 用户「%s」：%d 次失败操作，%s\n", a.Username, a.FailCount, a.Suggestion))
		}
	} else {
		sb.WriteString("未发现明显异常模式。\n")
	}

	sb.WriteString("\n---\n")
	sb.WriteString("以上分析基于审计日志统计，不自动定性不当行为，需管理员结合实际判断。\n")

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"summary":   sb.String(),
			"anomalies": anomalies,
			"is_draft":  true,
		},
	}
}

func (co *ChatOrchestrator) adminExplainUserActivity(ctx context.Context, args map[string]any, _ *AdminToolContext) *ToolResult {
	if co.auditRepo == nil {
		return &ToolResult{Success: false, Error: "audit repository not available"}
	}

	userID := toInt64(args["user_id"])
	limit := 20
	if v, ok := args["limit"]; ok {
		l := int(toInt64(v))
		if l > 0 && l <= 50 {
			limit = l
		}
	}

	// Get user info (without password hash)
	var username, role string
	if co.userRepo != nil {
		u, err := co.userRepo.GetByID(ctx, userID)
		if err == nil && u != nil {
			username = u.Username
			role = u.Role
		}
	}

	logs, total, _ := co.auditRepo.List(ctx, repository.ListParams{Page: 1, PageSize: limit}, &userID, nil)

	type activityItem struct {
		ID         int64  `json:"id"`
		OccurredAt string `json:"occurred_at"`
		Action     string `json:"action"`
		Result     string `json:"result"`
		TargetType string `json:"target_type,omitempty"`
		Target     string `json:"target,omitempty"`
	}

	var items []activityItem
	for _, l := range logs {
		// Redact: don't include payload, detail, IP, or user-agent
		items = append(items, activityItem{
			ID:         l.ID,
			OccurredAt: l.OccurredAt.Format(time.RFC3339),
			Action:     l.Action,
			Result:     l.Result,
			TargetType: l.TargetType,
			Target:     l.Target,
		})
	}

	truncated := total > int64(limit)
	data := map[string]any{
		"user_id":   userID,
		"items":     items,
		"total":     total,
		"truncated": truncated,
	}
	if username != "" {
		data["username"] = username
		data["role"] = role
	}

	return &ToolResult{Success: true, Data: data}
}

// ============================================================
// T8.3 — AI Usage Summary
// ============================================================

func (co *ChatOrchestrator) adminGetAIUsageSummary(ctx context.Context, args map[string]any, _ *AdminToolContext) *ToolResult {
	if co.usageRepo == nil {
		return &ToolResult{Success: false, Error: "usage repository not available"}
	}

	days := 1
	if v, ok := args["days"]; ok {
		d := int(toInt64(v))
		if d > 90 {
			d = 90
		}
		if d > 0 {
			days = d
		}
	}

	to := time.Now()
	from := to.AddDate(0, 0, -days)

	summary, err := co.usageRepo.Summary(ctx, from, to)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("failed to get usage summary: %v", err)}
	}

	byRole, err := co.usageRepo.ByRole(ctx, from, to)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("failed to get usage by role: %v", err)}
	}

	topUsers, err := co.usageRepo.TopUsers(ctx, from, to, 10)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("failed to get top users: %v", err)}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"period_days": days,
			"summary":     summary,
			"by_role":     byRole,
			"top_users":   topUsers,
			"note":        "不返回 API Key、用户输入内容等敏感信息。",
		},
	}
}

// ============================================================
// Helpers
// ============================================================

// maskIP partially masks an IP address for privacy.
func maskIP(ip string) string {
	if ip == "" {
		return ""
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".*.*"
	}
	// IPv6 or other formats — just return first segment
	if idx := strings.Index(ip, ":"); idx > 0 {
		return ip[:idx] + ":****"
	}
	return "***"
}

// redactSensitive masks known sensitive key=value patterns in a string.
// Handles key=value, key:value, key: value, JSON "key":"value", and Bearer tokens.
func redactSensitive(s string) string {
	// Pattern: (password|token|secret|api_key|bearer|authorization|access_token|private_key|client_secret|refresh_token|pwd)
	// followed by optional space/colon/equals and the value part.
	patterns := []struct {
		re      *regexp.Regexp
		replace string
	}{
		{regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9._-]{8,}`), "${1}***"},
		{regexp.MustCompile(`(?i)(password|pwd|passwd)\s*[:=]\s*"?([^\s",}]+)"?`), "${1}=***"},
		{regexp.MustCompile(`(?i)(token|api_key|apikey|access_token|refresh_token)\s*[:=]\s*"?([^\s",}]+)"?`), "${1}=***"},
		{regexp.MustCompile(`(?i)(secret|private_key|client_secret|jwt_secret|master_key)\s*[:=]\s*"?([^\s",}]+)"?`), "${1}=***"},
		{regexp.MustCompile(`(?i)(authorization)\s*[:=]\s*"?([^\s",}]+)"?`), "${1}=***"},
	}
	result := s
	for _, p := range patterns {
		result = p.re.ReplaceAllString(result, p.replace)
	}
	return result
}
