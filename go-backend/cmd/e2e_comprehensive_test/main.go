package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/pipeline"
)

func main() {
	apiKey := os.Getenv("MIMO_API_KEY")
	if apiKey == "" {
		log.Fatal("MIMO_API_KEY not set")
	}
	modelName := os.Getenv("MIMO_MODEL")
	if modelName == "" {
		modelName = "mimo-v2.5-pro"
	}
	baseURL := os.Getenv("MIMO_BASE_URL")
	if baseURL == "" {
		baseURL = "https://token-plan-cn.xiaomimimo.com/v1"
	}

	client := llm.NewClient(baseURL, apiKey, modelName, "")
	client.SetUseAPIKeyHeader(true)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	fmt.Printf("🔍 MiMo E2E 综合测试 | model=%s\n\n", modelName)

	// === TEST 1: Chat Function Calling 工具编排 ===
	fmt.Println("=== [TEST 1] Chat 工具编排 + 上下文注入 ===")

	eval := &model.Evaluation{ID: 1, Status: "scored", TotalScore: floatPtr(79.0)}
	eval.Scores = []model.DimensionScore{
		{DimensionID: 1, AIScore: floatPtr(75), Rationale: "代码结构基本清晰，命名规范"},
		{DimensionID: 2, AIScore: floatPtr(85), Rationale: "完整实现了核心功能"},
		{DimensionID: 3, AIScore: floatPtr(80), Rationale: "线程同步正确"},
		{DimensionID: 4, AIScore: floatPtr(70), Rationale: "文档质量一般"},
	}
	task := &model.TrainingTask{ID: 1, Name: "并发编程实训", Requirements: "1. 源代码\n2. 实验报告\n3. 测试截图"}
	dims := []model.Dimension{
		{ID: 1, Name: "代码规范", Weight: 25},
		{ID: 2, Name: "功能完整", Weight: 35},
		{ID: 3, Name: "并发正确", Weight: 25},
		{ID: 4, Name: "文档质量", Weight: 15},
	}
	pr := &model.ParseResult{RawText: "本实验使用Java实现了生产者-消费者模型，使用synchronized同步，经测试5生产者3消费者无死锁。"}

	sysPrompt := pipeline.BuildChatSystemPrompt(task, eval, pr, dims)
	fmt.Printf("  System prompt (%d chars): %s...\n\n", len(sysPrompt), truncate(sysPrompt, 200))

	messages := []llm.ChatMessage{
		llm.NewTextMessage("system", sysPrompt),
		llm.NewTextMessage("user", "我的代码质量得分为什么只有75分？我该怎么改进？"),
	}
	resp, err := client.Complete(ctx, messages, pipeline.ChatToolSchemas())
	if err != nil {
		fmt.Printf("  ❌ TEST 1 FAILED: %v\n\n", err)
	} else {
		choice := resp.Choices[0]
		content := choice.Message.Content
		toolCalls := choice.Message.ToolCalls
		if len(toolCalls) > 0 {
			fmt.Printf("  ✅ 模型返回 tool_calls=%d 个\n", len(toolCalls))
			for _, tc := range toolCalls {
				fmt.Printf("     - %s\n", tc.Function.Name)
			}
			fmt.Println()
		} else if content != "" {
			fmt.Printf("  ✅ 直接回答 (%d chars):\n     %q\n\n", len([]rune(content)), truncate(content, 500))
		} else {
			fmt.Printf("  ⚠️  空响应\n\n")
		}
	}

	// === TEST 2: 学生薄弱点分析 ===
	fmt.Println("=== [TEST 2] 薄弱点 LLM 分析 ===")
	profilePrompt := `你是一位实训教学分析专家。以下学生的历次评价数据：

## 各维度得分汇总
- 代码规范: 平均分=75.0
- 功能完整: 平均分=85.0
- 并发正确: 平均分=80.0
- 文档质量: 平均分=55.0 (薄弱!)

## 历次任务
- 任务「并发编程」: 总分=79.0
- 任务「数据库设计」: 总分=72.0

## 已识别的薄弱维度（平均分<60）
- 文档质量 (当前掌握度: 55)

请为每个薄弱维度生成以下内容（JSON格式）：
1. 具体的薄弱点描述
2. 个性化学习建议（不少于100字）

输出格式: {"suggestions": [{"dimension":"名称", "description":"薄弱点描述", "advice":"学习建议(≥100字)"}]}`

	resp2, err := client.Complete(ctx, []llm.ChatMessage{
		llm.NewTextMessage("system", "你是一位实训教学分析专家。请基于学生历次评价数据分析薄弱点并生成学习建议。严格以JSON格式输出。"),
		llm.NewTextMessage("user", profilePrompt),
	}, nil)
	if err != nil {
		fmt.Printf("  ❌ TEST 2 FAILED: %v\n\n", err)
	} else if resp2.Choices[0].Message.Content != "" {
		content := resp2.Choices[0].Message.Content
		fmt.Printf("  LLM 回复截取: %s...\n\n", truncate(content, 500))
		var result struct {
			Suggestions []struct {
				Dimension   string `json:"dimension"`
				Description string `json:"description"`
				Advice      string `json:"advice"`
			} `json:"suggestions"`
		}
		if err := json.Unmarshal([]byte(content), &result); err != nil {
			fmt.Printf("  ⚠️  JSON解析失败（可能是markdown包裹）\n\n")
		} else {
			for _, s := range result.Suggestions {
				runeLen := len([]rune(s.Advice))
				fmt.Printf("  ✅ %s: 描述=%s | 建议=%d字\n", s.Dimension, truncate(s.Description, 100), runeLen)
				if runeLen < 100 {
					fmt.Printf("     ❌ 不足100字\n")
				}
				fmt.Println()
			}
		}
	}

	// === TEST 3: 教学画像总结 ===
	fmt.Println("=== [TEST 3] 教学画像 LLM 总结 ===")
	teachingPrompt := `你是教学质量管理分析专家。请为以下教学质量画像生成中文总结（不少于150字）：

分析范围：学校
平均得分：76.3
分数分布：[12, 45, 78, 52, 18]
共性薄弱维度：文档质量、代码规范
建议加强教学：文档质量、代码规范`

	resp3, err := client.Complete(ctx, []llm.ChatMessage{
		llm.NewTextMessage("system", "你是教学质量管理分析专家。请基于数据生成简洁、有洞察力的中文教学分析总结。直接输出总结内容，不要附加JSON格式。"),
		llm.NewTextMessage("user", teachingPrompt),
	}, nil)
	if err != nil {
		fmt.Printf("  ❌ TEST 3 FAILED: %v\n\n", err)
	} else if resp3.Choices[0].Message.Content != "" {
		content := resp3.Choices[0].Message.Content
		runeLen := len([]rune(content))
		fmt.Printf("  ✅ 教学总结 (%d字):\n", runeLen)
		fmt.Printf("  %s\n\n", truncate(content, 500))
	}

	fmt.Println("========================================")
	fmt.Println("✅ 全部测试完成")
	fmt.Println("========================================")
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

func floatPtr(v float64) *float64 { return &v }
