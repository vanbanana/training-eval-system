// Package pipeline provides the chat tool orchestrator with 7 Function Calling tools
// for AI-assisted evaluation Q&A (requirement 22.2).
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

// LLMCompleter is the interface for LLM completion calls.
// Both *llm.Client (production) and test mocks satisfy this.
type LLMCompleter interface {
	Complete(ctx context.Context, messages []llm.ChatMessage, tools []llm.Tool) (*llm.ChatResponse, error)
}

// ChatToolContext holds all context needed by chat tools.
type ChatToolContext struct {
	StudentID   int64
	Evaluation  *model.Evaluation
	Task        *model.TrainingTask
	Upload      *model.Upload
	ParseResult *model.ParseResult
	Dimensions  []model.Dimension
	// OnToolCall is an optional per-request callback for SSE progress events.
	// Prevents the data race of using the shared ChatOrchestrator.OnToolCall.
	OnToolCall func(toolName string)
}

// ToolResult is the result of a tool call.
type ToolResult struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ChatOrchestrator manages the Function Calling loop for AI chat.
type ChatOrchestrator struct {
	client        LLMCompleter
	evalRepo      repository.EvaluationRepo
	uploadRepo    repository.UploadRepo
	taskRepo      repository.TaskRepo
	profileRepo   repository.ProfileRepo
	classRepo     repository.ClassRepo      // optional, for teacher tools
	courseRepo    repository.CourseRepo     // optional, for teacher tools
	simRepo       repository.SimilarityRepo // optional, for teacher tools
	userRepo      repository.UserRepo       // optional, for admin tools
	llmConfigRepo repository.LLMConfigRepo  // optional, for admin tools
	auditRepo     repository.AuditRepo      // optional, for admin tools
	usageRepo     repository.UsageRepo      // optional, for admin tools (T8.3)
	// OnToolCall is an optional callback invoked when a tool is dispatched.
	// The handler uses it to emit SSE progress events to the frontend.
	OnToolCall func(toolName string)
	// maxToolRounds is the configurable max number of tool-call cycles (T8.2).
	// Defaults to MaxToolRounds constant; can be overridden via SetMaxToolRounds.
	maxToolRounds int
}

// NewChatOrchestrator creates a chat orchestrator.
func NewChatOrchestrator(
	client LLMCompleter,
	evalRepo repository.EvaluationRepo,
	uploadRepo repository.UploadRepo,
	taskRepo repository.TaskRepo,
	profileRepo repository.ProfileRepo,
) *ChatOrchestrator {
	return &ChatOrchestrator{
		client:        client,
		evalRepo:      evalRepo,
		uploadRepo:    uploadRepo,
		taskRepo:      taskRepo,
		profileRepo:   profileRepo,
		maxToolRounds: MaxToolRounds,
	}
}

// SetMaxToolRounds configures the max number of tool-call cycles per request (T8.2).
func (co *ChatOrchestrator) SetMaxToolRounds(n int) {
	if n > 0 {
		co.maxToolRounds = n
	}
}

// MaxToolRounds is the max number of tool-call cycles.
const MaxToolRounds = 5

// MaxToolResultBytes limits tool result size.
const MaxToolResultBytes = 8 * 1024

// llmMaxRetries is the max number of LLM call retries per round.
const llmMaxRetries = 2

// llmRetryBaseDelay is the base delay for exponential backoff between retries.
const llmRetryBaseDelay = 1 * time.Second

// toolCallTimeout is the max duration for a single tool execution.
const toolCallTimeout = 15 * time.Second

// maxConsecutiveFailedRounds aborts the loop if this many consecutive rounds
// have all tools fail.
const maxConsecutiveFailedRounds = 2

// ChatToolSchemas returns all 8 registered chat tools as OpenAI function definitions.
func ChatToolSchemas() []llm.Tool {
	return []llm.Tool{
		toolGetParseSegment(),
		toolGetDimensionDetail(),
		toolGetClassStatistics(),
		toolGetDimensionClassStatistics(),
		toolGetDimensionHistory(),
		toolGetExcellentSampleSummary(),
		toolGetWeaknessList(),
		toolGetLearningResources(),
	}
}

// toolRequiredParams maps tool names to their required parameter names,
// used for argument validation before dispatch.
var toolRequiredParams = map[string][]string{
	"get_parse_segment":              {"topic"},
	"get_dimension_detail":           {"dimension_name"},
	"get_dimension_history":          {"dimension_name"},
	"get_dimension_class_statistics": {"dimension_name"},
	"get_excellent_sample_summary":   {},
	"get_weakness_list":              {},
	"get_class_statistics":           {},
	"get_learning_resources":         {"keyword"},
}

// DispatchTool dispatches a tool call by name with argument validation.
func (co *ChatOrchestrator) DispatchTool(ctx context.Context, name string, args map[string]any, tctx *ChatToolContext) *ToolResult {
	// Validate required parameters
	if required, ok := toolRequiredParams[name]; ok {
		for _, param := range required {
			v, exists := args[param]
			if !exists || v == nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("工具 %s 缺少必填参数 %q，请提供该参数后重试", name, param),
				}
			}
			if s, ok := v.(string); ok && s == "" {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("工具 %s 的参数 %q 不能为空，请提供有效值", name, param),
				}
			}
		}
	}

	switch name {
	case "get_parse_segment":
		return co.getParseSegment(tctx, args)
	case "get_dimension_detail":
		return co.getDimensionDetail(tctx, args)
	case "get_class_statistics":
		return co.getClassStatistics(ctx, tctx, args)
	case "get_dimension_class_statistics":
		return co.getDimensionClassStatistics(ctx, tctx, args)
	case "get_dimension_history":
		return co.getDimensionHistory(ctx, tctx, args)
	case "get_excellent_sample_summary":
		return co.getExcellentSampleSummary(ctx, tctx, args)
	case "get_weakness_list":
		return co.getWeaknessList(ctx, tctx)
	case "get_learning_resources":
		return co.getLearningResources(args)
	default:
		return &ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", name)}
	}
}

func (co *ChatOrchestrator) retryLLMCall(
	ctx context.Context,
	messages []llm.ChatMessage,
	tools []llm.Tool,
) (*llm.ChatResponse, error) {
	if co.client == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}
	var lastErr error
	for attempt := 0; attempt <= llmMaxRetries; attempt++ {
		if attempt > 0 {
			delay := llmRetryBaseDelay * (1 << (attempt - 1)) // 1s, 2s
			slog.Warn("llm retry",
				"attempt", attempt+1,
				"delay_ms", delay.Milliseconds(),
				"error", lastErr.Error(),
			)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		resp, err := co.client.Complete(ctx, messages, tools)
		if err == nil {
			if attempt > 0 {
				slog.Info("llm retry succeeded", "attempt", attempt+1)
			}
			return resp, nil
		}
		lastErr = err

		errMsg := err.Error()
		// Non-retryable: 4xx errors except 429 (rate limit)
		if strings.Contains(errMsg, "status 4") && !strings.Contains(errMsg, "status 429") {
			slog.Error("llm non-retryable error", "error", errMsg)
			return nil, err
		}

		slog.Warn("llm call failed, will retry",
			"attempt", attempt+1,
			"max_retries", llmMaxRetries,
			"error", errMsg,
		)
	}
	return nil, fmt.Errorf("llm call failed after %d attempts: %w", llmMaxRetries+1, lastErr)
}

// Run executes the robust chat orchestrator loop with retry, tool timeout,
// consecutive-error circuit breaking, and enhanced error feedback.
func (co *ChatOrchestrator) Run(
	ctx context.Context,
	history []llm.ChatMessage,
	userMessage string,
	tctx *ChatToolContext,
) (*llm.ChatResponse, error) {
	messages := make([]llm.ChatMessage, 0, len(history)+10)

	// Build system prompt with full evaluation context
	sysPrompt := BuildChatSystemPrompt(tctx.Task, tctx.Evaluation, tctx.ParseResult, tctx.Dimensions)
	messages = append(messages, llm.NewTextMessage("system", sysPrompt))

	// Add conversation history (last 20 messages)
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	messages = append(messages, history...)

	// Add current user message
	messages = append(messages, llm.NewTextMessage("user", userMessage))

	tools := ChatToolSchemas()

	// Track repeated tool failures across rounds for circuit breaking
	consecutiveFailedRounds := 0
	// Track which tools have failed twice in a row (to warn LLM)
	repeatedFailures := map[string]int{}

	for round := 0; round < co.maxToolRounds; round++ {
		// --- LLM call with retry ---
		resp, err := co.retryLLMCall(ctx, messages, tools)
		if err != nil {
			slog.Error("chat orchestrator: all LLM retries exhausted", "round", round+1, "error", err.Error())
			// Graceful degradation: if we already have conversation context,
			// return a fallback message instead of failing.
			if round > 0 {
				return co.fallbackResponse(ctx, messages)
			}
			return nil, fmt.Errorf("chat orchestrator: LLM call failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			slog.Warn("chat orchestrator: empty LLM response", "round", round+1)
			if round > 0 {
				return co.fallbackResponse(ctx, messages)
			}
			return nil, fmt.Errorf("chat orchestrator: empty response")
		}

		choice := resp.Choices[0]

		// If the model returns content directly (no tool calls), we're done
		if choice.FinishReason == "stop" && choice.Message.Content != "" {
			return resp, nil
		}

		// If no tool calls, we're done
		if len(choice.Message.ToolCalls) == 0 {
			return resp, nil
		}

		// --- Process tool calls ---
		messages = append(messages, llm.ChatMessage{
			Role:      "assistant",
			ToolCalls: choice.Message.ToolCalls,
		})

		roundToolFailures := 0
		roundToolTotal := len(choice.Message.ToolCalls)

		for _, tc := range choice.Message.ToolCalls {
			slog.Info("chat tool called", "tool", tc.Function.Name, "round", round+1)

			// Fire callback for frontend progress events (per-request context)
			cb := tctx.OnToolCall
			if cb == nil {
				cb = co.OnToolCall // fallback to shared callback for legacy compatibility
			}
			if cb != nil {
				cb(tc.Function.Name)
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				slog.Warn("tool argument parse failed", "tool", tc.Function.Name, "error", err.Error())
				args = map[string]any{}
			}

			// Dispatch with per-tool timeout
			toolCtx, toolCancel := context.WithTimeout(ctx, toolCallTimeout)
			result := co.DispatchTool(toolCtx, tc.Function.Name, args, tctx)
			toolCancel()

			if result == nil {
				result = &ToolResult{Success: false, Error: "tool returned nil result"}
			}

			// Track per-tool failure counts for repeated-failure warning
			if !result.Success {
				roundToolFailures++
				repeatedFailures[tc.Function.Name]++
			} else {
				repeatedFailures[tc.Function.Name] = 0
			}

			// Enhanced error feedback: when a tool fails, prepend a clear
			// instruction so the LLM knows to try something else.
			if !result.Success {
				hint := fmt.Sprintf("工具调用失败（%s）。", tc.Function.Name)
				if repeatedFailures[tc.Function.Name] >= 2 {
					hint += "该工具已多次失败，请不要再次调用它，尝试其他方式回答用户问题。"
				} else {
					hint += "请检查参数是否正确，或尝试使用其他工具。"
				}
				result.Error = hint + " 错误详情：" + result.Error
			}

resultJSON, _ := json.Marshal(result)
				if len(resultJSON) > MaxToolResultBytes {
					// Truncate the Error message first, then fall back to truncating Data fields
					if len(result.Error) > 500 {
						result.Error = result.Error[:500] + "...[truncated]"
					}
					if result.Data != nil {
						if dataStr, ok := result.Data.(string); ok && len(dataStr) > 1000 {
							result.Data = dataStr[:1000] + "...[truncated]"
						}
					}
					resultJSON, _ = json.Marshal(result)
					if len(resultJSON) > MaxToolResultBytes {
						// Last resort: generate a minimal valid JSON with truncated error
						resultJSON = []byte(fmt.Sprintf(
							`{"success":false,"error":"tool result too large (%d bytes), truncated","data":null}`,
							len(resultJSON),
						))
					}
					slog.Warn("chat tool result truncated", "tool", tc.Function.Name, "original_size", len(resultJSON))
				}

			messages = append(messages, llm.ChatMessage{
				Role:       "tool",
				Content:    string(resultJSON),
				ToolCallID: tc.ID,
			})
		}

		// --- Consecutive-round failure tracking ---
		if roundToolFailures == roundToolTotal {
			consecutiveFailedRounds++
			slog.Warn("all tools failed in round",
				"round", round+1,
				"consecutive_failed_rounds", consecutiveFailedRounds,
			)
			if consecutiveFailedRounds >= maxConsecutiveFailedRounds {
				slog.Warn("circuit breaker: too many consecutive failed rounds, aborting loop")
				break
			}
		} else {
			consecutiveFailedRounds = 0
		}

		// Reset per-round failure map entries for tools that succeeded
		for name := range repeatedFailures {
			if repeatedFailures[name] > 0 && repeatedFailures[name] < 2 {
				// Keep counting; only reset if tool wasn't called this round
			}
		}
	}

	// Max rounds reached or circuit breaker tripped — force final answer
	messages = append(messages, llm.NewTextMessage("user",
		"请基于已有的工具返回结果直接给出回答。如果某些工具调用失败，请说明无法获取该部分数据，并基于已有信息尽力回答。不要再调用工具。"))
	resp, err := co.retryLLMCall(ctx, messages, nil)
	if err != nil {
		return co.fallbackResponse(ctx, messages)
	}
	return resp, nil
}

// fallbackResponse generates a graceful degradation response when all retries fail.
func (co *ChatOrchestrator) fallbackResponse(
	ctx context.Context,
	messages []llm.ChatMessage,
) (*llm.ChatResponse, error) {
	fallback := &llm.ChatResponse{
		Choices: []llm.ChatChoice{
			{
				Index:        0,
				FinishReason: "stop",
				Message: llm.ChatMessage{
					Role:    "assistant",
					Content: "抱歉，系统暂时遇到了问题，无法完成查询。请稍后再试，或者换一种方式描述你的问题。",
				},
			},
		},
	}
	slog.Warn("chat orchestrator: returned fallback response")
	return fallback, nil
}

// BuildChatSystemPrompt creates the evaluation-aware system prompt.
func BuildChatSystemPrompt(task *model.TrainingTask, eval *model.Evaluation, pr *model.ParseResult, dims []model.Dimension) string {
	var sb strings.Builder
	sb.WriteString("你是实训评价 AI 助手，帮助学生理解他们的评价结果并提供改进建议。")
	sb.WriteString("你可以使用以下工具查询具体数据，回答时必须基于工具返回值。")
	sb.WriteString("回答要求简洁、准确、给出具体改进方向。")
	sb.WriteString("不可暴露其他学生姓名或身份信息。\n\n")

	if task != nil {
		sb.WriteString(fmt.Sprintf("## 任务信息\n- 名称：%s\n- 要求：%s\n\n", task.Name, task.Requirements))
	}

	if eval != nil {
		sb.WriteString(fmt.Sprintf("## 评价状态\n- 状态：%s\n", eval.Status))
		if eval.TotalScore != nil {
			sb.WriteString(fmt.Sprintf("- 综合得分：%.1f\n", *eval.TotalScore))
		}
		sb.WriteString("\n")
	}

	if dims != nil {
		sb.WriteString("## 评价维度\n")
		for _, d := range dims {
			sb.WriteString(fmt.Sprintf("- %s（权重 %d%%）: %s\n", d.Name, d.Weight, d.Description))
		}
		sb.WriteString("\n")
	}

if pr != nil && pr.RawText != "" {
			text := pr.RawText
			textRunes := []rune(text)
			if len(textRunes) > 2000 {
				text = string(textRunes[:2000])
			}
			sb.WriteString(fmt.Sprintf("## 学生提交内容摘要\n%s\n\n", text))
		}

	return sb.String()
}

// --- Tool implementations ---

func (co *ChatOrchestrator) getParseSegment(tctx *ChatToolContext, args map[string]any) *ToolResult {
	topic, _ := args["topic"].(string)
	maxChars := 500
	if mc, ok := args["max_chars"].(float64); ok {
		maxChars = int(mc)
	}

if tctx.ParseResult == nil || tctx.ParseResult.RawText == "" {
			return &ToolResult{Success: false, Error: "no parsed content available"}
		}

		text := tctx.ParseResult.RawText
		textRunes := []rune(text)
		if len(textRunes) > maxChars {
			textRunes = textRunes[:maxChars]
		}
		text = string(textRunes)

	// Case-insensitive search for topic
	if topic != "" {
		lower := strings.ToLower(text)
		lowerTopic := strings.ToLower(topic)
		if idx := strings.Index(lower, lowerTopic); idx >= 0 {
			start := idx
			end := idx + len(topic) + 200
			if end > len(text) {
				end = len(text)
			}
			// Expand back to sentence boundary
			if start > 100 {
				start -= 100
			} else {
				start = 0
			}
			text = text[start:end]
		}
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"segments": []map[string]any{{"text": text}}},
	}
}

func (co *ChatOrchestrator) getDimensionDetail(tctx *ChatToolContext, args map[string]any) *ToolResult {
	dimName, _ := args["dimension_name"].(string)

	if tctx.Evaluation == nil || tctx.Evaluation.Scores == nil {
		return &ToolResult{Success: false, Error: "no scores available"}
	}

	dimNameMap := make(map[int64]string)
	for _, d := range tctx.Dimensions {
		dimNameMap[d.ID] = d.Name
	}

	var results []map[string]any
	for _, s := range tctx.Evaluation.Scores {
		name := dimNameMap[s.DimensionID]
		if dimName != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(dimName)) {
			continue
		}
		score := 0.0
		if s.TeacherScore != nil {
			score = *s.TeacherScore
		} else if s.AIScore != nil {
			score = *s.AIScore
		}
		results = append(results, map[string]any{
			"dimension":    name,
			"score":        score,
			"rationale":    s.Rationale,
			"dimension_id": s.DimensionID,
		})
	}

	if len(results) == 0 {
		return &ToolResult{Success: false, Error: fmt.Sprintf("dimension %q not found", dimName)}
	}

	return &ToolResult{Success: true, Data: map[string]any{"details": results}}
}

func (co *ChatOrchestrator) getClassStatistics(ctx context.Context, tctx *ChatToolContext, args map[string]any) *ToolResult {
	if tctx.Task == nil {
		return &ToolResult{Success: false, Error: "no task context"}
	}

	params := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
	}
	params.TaskID = &tctx.Task.ID
	evals, _, err := co.evalRepo.List(ctx, params)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("query failed: %v", err)}
	}

	var scores []float64
	for _, e := range evals {
		if e.TotalScore != nil && e.Status != "rejected" {
			scores = append(scores, *e.TotalScore)
		}
	}

	if len(scores) == 0 {
		return &ToolResult{Success: false, Error: "no scores available for comparison"}
	}

	sort.Float64s(scores)
	n := len(scores)
	mean := average(scores)
	median := scores[n/2]
	if n%2 == 0 {
		median = (scores[n/2-1] + scores[n/2]) / 2
	}

	p75 := scores[int(math.Ceil(float64(n)*0.75))-1]
	if p75idx := int(math.Ceil(float64(n)*0.75)) - 1; p75idx < n {
		p75 = scores[p75idx]
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"count":  n,
			"mean":   math.Round(mean*10) / 10,
			"median": math.Round(median*10) / 10,
			"p75":    math.Round(p75*10) / 10,
		},
	}
}

// getDimensionClassStatistics returns per-dimension class mean/median/p75 for a specific dimension.
func (co *ChatOrchestrator) getDimensionClassStatistics(ctx context.Context, tctx *ChatToolContext, args map[string]any) *ToolResult {
	if tctx.Task == nil {
		return &ToolResult{Success: false, Error: "no task context"}
	}

	dimName, _ := args["dimension_name"].(string)
	if dimName == "" {
		return &ToolResult{Success: false, Error: "dimension_name is required"}
	}

	// Find the dimension ID by name
	var targetDimID int64
	for _, d := range tctx.Dimensions {
		if strings.Contains(strings.ToLower(d.Name), strings.ToLower(dimName)) {
			targetDimID = d.ID
			break
		}
	}
	if targetDimID == 0 {
		return &ToolResult{Success: false, Error: fmt.Sprintf("dimension %q not found in task", dimName)}
	}

	params := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
	}
	params.TaskID = &tctx.Task.ID
	evals, _, err := co.evalRepo.List(ctx, params)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("query failed: %v", err)}
	}

	var scores []float64
	for _, e := range evals {
		if e.Status == "rejected" || e.Scores == nil {
			continue
		}
		for _, s := range e.Scores {
			if s.DimensionID == targetDimID {
				score := 0.0
				if s.TeacherScore != nil {
					score = *s.TeacherScore
				} else if s.AIScore != nil {
					score = *s.AIScore
				}
				if score > 0 {
					scores = append(scores, score)
				}
				break
			}
		}
	}

	if len(scores) == 0 {
		return &ToolResult{Success: false, Error: fmt.Sprintf("no scores found for dimension %q", dimName)}
	}

	sort.Float64s(scores)
	n := len(scores)
	mean := average(scores)
	median := scores[n/2]
	if n%2 == 0 {
		median = (scores[n/2-1] + scores[n/2]) / 2
	}
	p75Idx := int(math.Ceil(float64(n)*0.75)) - 1
	if p75Idx < 0 {
		p75Idx = 0
	}
	if p75Idx >= n {
		p75Idx = n - 1
	}
	p75 := scores[p75Idx]

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"dimension": dimName,
			"count":     n,
			"mean":      math.Round(mean*10) / 10,
			"median":    math.Round(median*10) / 10,
			"p75":       math.Round(p75*10) / 10,
		},
	}
}

func (co *ChatOrchestrator) getDimensionHistory(ctx context.Context, tctx *ChatToolContext, args map[string]any) *ToolResult {
	if tctx.StudentID == 0 {
		return &ToolResult{Success: false, Error: "no student context"}
	}

	params := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 50},
		StudentID:  &tctx.StudentID,
	}
	evals, _, err := co.evalRepo.List(ctx, params)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("query failed: %v", err)}
	}

	dimName, _ := args["dimension_name"].(string)
	var points []map[string]any

	for _, e := range evals {
		if e.Status == "rejected" || e.Status == "pending" || e.Scores == nil {
			continue
		}
		for _, s := range e.Scores {
			// Resolve dimension name
			found := false
			for _, d := range tctx.Dimensions {
				if d.ID == s.DimensionID && (dimName == "" || strings.Contains(strings.ToLower(d.Name), strings.ToLower(dimName))) {
					score := 0.0
					if s.TeacherScore != nil {
						score = *s.TeacherScore
					} else if s.AIScore != nil {
						score = *s.AIScore
					}
					points = append(points, map[string]any{
						"eval_id": e.ID,
						"score":   score,
						"date":    e.CreatedAt.Format("2006-01-02"),
					})
					found = true
					break
				}
			}
			_ = found
		}
	}

	if len(points) > 10 {
		points = points[len(points)-10:]
	}

	return &ToolResult{Success: true, Data: map[string]any{"scores": points}}
}

func (co *ChatOrchestrator) getExcellentSampleSummary(ctx context.Context, tctx *ChatToolContext, args map[string]any) *ToolResult {
	if tctx == nil || tctx.Evaluation == nil {
		return &ToolResult{Success: false, Error: "评价上下文缺失，无法查询优秀示例。"}
	}
	if tctx.Task == nil {
		return &ToolResult{Success: false, Error: "no task context"}
	}

	params := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 50},
	}
	params.TaskID = &tctx.Task.ID
	evals, _, err := co.evalRepo.List(ctx, params)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("query failed: %v", err)}
	}

	type scored struct {
		eval  model.Evaluation
		score float64
	}
	var top []scored
	for _, e := range evals {
		if e.TotalScore != nil && e.Status != "rejected" && e.UploadID != tctx.Evaluation.UploadID {
			top = append(top, scored{e, *e.TotalScore})
		}
	}
	sort.Slice(top, func(i, j int) bool { return top[i].score > top[j].score })

	topN := 1
	if n, ok := args["top_n"].(float64); ok {
		topN = int(n)
	}
	if topN > len(top) {
		topN = len(top)
	}

	var summaries []map[string]any
	for i := 0; i < topN; i++ {
		e := top[i].eval
		summaries = append(summaries, map[string]any{
			"score": e.TotalScore,
			// anonymized: no student name or ID
		})
	}

	return &ToolResult{Success: true, Data: map[string]any{"summaries": summaries}}
}

func (co *ChatOrchestrator) getWeaknessList(ctx context.Context, tctx *ChatToolContext) *ToolResult {
	if tctx.StudentID == 0 {
		return &ToolResult{Success: false, Error: "no student context"}
	}

	profile, err := co.profileRepo.GetByStudentID(ctx, tctx.StudentID)
	if err != nil || profile == nil {
		return &ToolResult{Success: false, Error: "no profile data yet — complete more evaluations first"}
	}

	var weaknesses []map[string]any
	if profile.WeaknessList != nil {
		// profile.WeaknessList is stored as any ([]map[string]any from service layer)
		if wl, ok := profile.WeaknessList.([]map[string]any); ok {
			weaknesses = wl
		} else if wlStr, ok := profile.WeaknessList.(string); ok {
			_ = json.Unmarshal([]byte(wlStr), &weaknesses)
		}
	}

	return &ToolResult{Success: true, Data: map[string]any{"weaknesses": weaknesses}}
}

func (co *ChatOrchestrator) getLearningResources(args map[string]any) *ToolResult {
	keyword, _ := args["keyword"].(string)
	if keyword == "" {
		return &ToolResult{Success: false, Error: "keyword is required"}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"resources": []map[string]any{
				{"type": "online_course", "title": fmt.Sprintf("%s 入门教程", keyword), "url": fmt.Sprintf("https://www.bilibili.com/search?keyword=%s", keyword)},
				{"type": "documentation", "title": fmt.Sprintf("%s 官方文档", keyword)},
				{"type": "practice", "title": fmt.Sprintf("%s 实战练习", keyword)},
			},
		},
	}
}

// --- Tool schema generators ---

func toolGetParseSegment() llm.Tool {
	return makeTool("get_parse_segment", "查询当前评价对应实训成果中某主题的原文片段", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"topic":     map[string]any{"type": "string", "description": "搜索主题关键词"},
			"max_chars": map[string]any{"type": "integer", "description": "最大返回字符数，默认500"},
		},
		"required": []string{"topic"},
	})
}

func toolGetDimensionDetail() llm.Tool {
	return makeTool("get_dimension_detail", "查询当前评价某维度的详细评分依据与扣分项", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dimension_name": map[string]any{"type": "string", "description": "评价维度名称"},
		},
		"required": []string{"dimension_name"},
	})
}

func toolGetClassStatistics() llm.Tool {
	return makeTool("get_class_statistics", "查询当前任务在班级范围内的统计（不暴露他人姓名）", map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	})
}

func toolGetDimensionClassStatistics() llm.Tool {
	return makeTool("get_dimension_class_statistics", "查询当前任务某个具体维度在班级范围内的均分、中位数、P75（用于维度级对比）", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dimension_name": map[string]any{"type": "string", "description": "评价维度名称"},
		},
		"required": []string{"dimension_name"},
	})
}

func toolGetDimensionHistory() llm.Tool {
	return makeTool("get_dimension_history", "查询学生该维度在过往任务中的得分轨迹", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dimension_name": map[string]any{"type": "string", "description": "维度名称"},
			"limit":          map[string]any{"type": "integer", "description": "返回条数，默认10"},
		},
		"required": []string{"dimension_name"},
	})
}

func toolGetExcellentSampleSummary() llm.Tool {
	return makeTool("get_excellent_sample_summary", "获取该任务下匿名化的高分样例摘要", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"top_n": map[string]any{"type": "integer", "description": "返回前N名，默认1"},
		},
	})
}

func toolGetWeaknessList() llm.Tool {
	return makeTool("get_weakness_list", "获取学生当前已识别的薄弱点列表", map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	})
}

func toolGetLearningResources() llm.Tool {
	return makeTool("get_learning_resources", "根据知识点关键词推荐学习资源", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{"type": "string", "description": "知识点关键词"},
		},
		"required": []string{"keyword"},
	})
}

func makeTool(name, description string, params map[string]any) llm.Tool {
	paramsJSON, _ := json.Marshal(params)
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  paramsJSON,
		},
	}
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
