package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smartedu/training-eval-system/internal/model"
)

// BuildScoringPrompt constructs the system+user messages for dimension scoring.
func BuildScoringPrompt(task *model.TrainingTask, dims []model.Dimension, rawText string) []ChatMessage {
	// Build dimension description
	var dimDesc strings.Builder
	for _, d := range dims {
		dimDesc.WriteString(fmt.Sprintf("- %s（权重 %d%%）: %s\n", d.Name, d.Weight, d.Description))
	}

	systemPrompt := `你是一位专业的实训评价专家。请根据提供的实训任务要求和评价维度，对学生提交的实训成果进行客观评分。

评分规则：
1. 每个维度给出 0-100 的整数分
2. 给出简短但具体的评分依据（rationale），说明为什么给这个分数
3. 评分应该客观、公正，基于实际内容质量
4. 对缺少的部分适当扣分，对创新和深入的内容适当加分

请调用 submit_scores 函数提交你的评分结果。`

	userPrompt := fmt.Sprintf(`## 实训任务信息

**任务名称：** %s

**任务描述：** %s

**任务要求：**
%s

## 评价维度

%s

## 学生提交内容

%s

---

请根据以上信息，对每个评价维度进行评分，并调用 submit_scores 函数提交结果。`,
		task.Name,
		task.Description,
		task.Requirements,
		dimDesc.String(),
		rawText,
	)

	return []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
}

// BuildVerificationPrompt constructs messages for requirement verification.
func BuildVerificationPrompt(requirements string, rawText string) []ChatMessage {
	systemPrompt := `你是一位实训教学核查专家。请对比学生提交内容与实训要求，进行完整性核查。

核查内容：
1. match_rate: 需求覆盖率（0-100 整数，表示学生完成了多少百分比的要求）
2. checkpoints: 已完成的检查点列表
3. missing_items: 缺失的内容列表
4. logic_issues: 发现的逻辑问题列表

请调用 submit_verification 函数提交核查结果。`

	userPrompt := fmt.Sprintf(`## 实训要求

%s

## 学生提交内容

%s

---

请核查学生提交内容是否覆盖了所有实训要求，并调用 submit_verification 函数提交核查结果。`,
		requirements,
		rawText,
	)

	return []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
}

// BuildChatSystemPrompt constructs the system prompt for evaluation-aware chat.
// Truncates raw_text to maxChars (default 4000).
func BuildChatSystemPrompt(task *model.TrainingTask, eval *model.Evaluation, rawText string) string {
	const maxChars = 4000

	truncatedText := rawText
	if len(truncatedText) > maxChars {
		truncatedText = truncatedText[:maxChars]
	}

	var sb strings.Builder
	sb.WriteString("你是实训评价 AI 助手，帮助学生理解他们的评价结果并提供改进建议。\n\n")

	if task != nil {
		sb.WriteString(fmt.Sprintf("## 任务信息\n- 名称：%s\n- 描述：%s\n- 要求：%s\n\n", task.Name, task.Description, task.Requirements))
	}

	if eval != nil && eval.Scores != nil {
		sb.WriteString("## 评价结果\n")
		for _, s := range eval.Scores {
			score := 0.0
			if s.AIScore != nil {
				score = *s.AIScore
			}
			if s.TeacherScore != nil {
				score = *s.TeacherScore
			}
			sb.WriteString(fmt.Sprintf("- 维度 %d: %.1f 分 — %s\n", s.DimensionID, score, s.Rationale))
		}
		if eval.TotalScore != nil {
			sb.WriteString(fmt.Sprintf("\n总分：%.1f\n", *eval.TotalScore))
		}
		sb.WriteString("\n")
	}

	if truncatedText != "" {
		sb.WriteString(fmt.Sprintf("## 学生提交内容（节选）\n%s\n\n", truncatedText))
	}

	sb.WriteString("请基于以上信息回答学生的问题，帮助他们理解得分原因并给出具体改进建议。回答要具体、有建设性、鼓励学生进步。")

	return sb.String()
}

// ScoringToolSchema returns the Function Calling tool definition for submit_scores.
func ScoringToolSchema(dims []model.Dimension) Tool {
	// Build dimension_id enum
	dimIDs := make([]int64, 0, len(dims))
	for _, d := range dims {
		dimIDs = append(dimIDs, d.ID)
	}

	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"scores": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"dimension_id": map[string]any{
							"type":        "integer",
							"description": "评价维度 ID",
						},
						"score": map[string]any{
							"type":        "number",
							"minimum":     0,
							"maximum":     100,
							"description": "该维度得分 (0-100)",
						},
						"rationale": map[string]any{
							"type":        "string",
							"description": "评分理由，简短说明为什么给这个分数",
						},
					},
					"required": []string{"dimension_id", "score", "rationale"},
				},
				"description": "各维度评分结果数组",
			},
		},
		"required": []string{"scores"},
	}

	paramsJSON, _ := json.Marshal(params)

	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "submit_scores",
			Description: "提交各评价维度的评分结果",
			Parameters:  paramsJSON,
		},
	}
}

// VerificationToolSchema returns the Function Calling tool for submit_verification.
func VerificationToolSchema() Tool {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"match_rate": map[string]any{
				"type":        "number",
				"minimum":     0,
				"maximum":     100,
				"description": "需求覆盖率 (0-100)",
			},
			"checkpoints": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "已完成的检查点列表",
			},
			"missing_items": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "缺失的内容列表",
			},
			"logic_issues": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "发现的逻辑问题列表",
			},
		},
		"required": []string{"match_rate", "checkpoints", "missing_items", "logic_issues"},
	}

	paramsJSON, _ := json.Marshal(params)

	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "submit_verification",
			Description: "提交实训要求核查结果",
			Parameters:  paramsJSON,
		},
	}
}
