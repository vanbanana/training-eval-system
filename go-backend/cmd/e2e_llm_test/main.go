// E2E test: calls real MiMo V2.5 API to verify the scoring & verification pipelines work end-to-end.
// Uses token-plan based MiMo API: https://platform.xiaomimimo.com/docs/zh-CN/welcome
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
		log.Fatal("MIMO_API_KEY environment variable not set. Get your API key from https://platform.xiaomimimo.com")
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
	client.SetUseAPIKeyHeader(true) // MiMo uses api-key header

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// ======== TEST 1: Simple chat completion ========
	fmt.Println("=== TEST 1: Basic chat completion ===")
	fmt.Printf("Model: %s\nBase URL: %s\n", modelName, baseURL)

	resp, err := client.Complete(ctx, []llm.ChatMessage{
		llm.NewTextMessage("user", "Say 'hello' in Chinese, nothing else."),
	}, nil)
	if err != nil {
		log.Fatalf("TEST 1 FAILED: %v", err)
	}
	fmt.Printf("Response: %q\n", resp.Choices[0].Message.Content)
	if resp.Usage != nil {
		fmt.Printf("Tokens: prompt=%d completion=%d total=%d\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}
	fmt.Println()

	// ======== TEST 2: Function Calling scoring (MiMo supports tool_calls) ========
	fmt.Println("=== TEST 2: Function Calling scoring ===")

	task := &model.TrainingTask{
		Name:         "并发编程实训",
		Description:  "使用 Java 实现生产者-消费者模型",
		Requirements: "1. 源代码 zip\n2. 实验报告 PDF\n3. 测试用例截图",
	}
	dims := []model.Dimension{
		{ID: 1, Name: "代码规范性", Weight: 25, Description: "代码命名、注释、结构"},
		{ID: 2, Name: "功能完整性", Weight: 35, Description: "是否完整实现要求的功能"},
		{ID: 3, Name: "并发正确性", Weight: 25, Description: "线程安全、无死锁"},
		{ID: 4, Name: "文档质量", Weight: 15, Description: "报告完整性、图表"},
	}

	rawText := `本实验实现了生产者-消费者模型。
使用Java的synchronized关键字实现线程同步。
生产者线程负责生产数据放入缓冲区，消费者线程从缓冲区取出数据。
当缓冲区满时生产者等待，当缓冲区空时消费者等待。
测试了5个生产者和3个消费者并发运行的场景，运行稳定无死锁。
报告包含UML类图和时序图。`

	messages := llm.BuildScoringPrompt(task, dims, rawText)
	tool := llm.ScoringToolSchema(dims)

	fmt.Printf("Calling %s with Function Calling...\n", modelName)
	start := time.Now()
	scoreResp, err := client.Complete(ctx, messages, []llm.Tool{tool})
	elapsed := time.Since(start)
	if err != nil {
		log.Fatalf("TEST 2 FAILED (LLM call): %v", err)
	}

	fmt.Printf("LLM response in %v\n", elapsed)
	if scoreResp.Usage != nil {
		fmt.Printf("Tokens: prompt=%d completion=%d total=%d\n",
			scoreResp.Usage.PromptTokens, scoreResp.Usage.CompletionTokens, scoreResp.Usage.TotalTokens)
	}

	content := scoreResp.Choices[0].Message.Content
	toolCalls := scoreResp.Choices[0].Message.ToolCalls
	fmt.Printf("Raw content: %q\n", content)
	fmt.Printf("Tool calls: %d\n", len(toolCalls))

	var toolResp pipeline.ScoreToolResponse

	if len(toolCalls) > 0 {
		for _, tc := range toolCalls {
			fmt.Printf("  tool_call: name=%s, args=%s\n", tc.Function.Name, tc.Function.Arguments)
			if tc.Function.Name == "submit_scores" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &toolResp); err != nil {
					log.Fatalf("TEST 2 FAILED (parse tool_call args): %v", err)
				}
				break
			}
		}
	} else if content != "" {
		// Fallback: try content as JSON
		if err := json.Unmarshal([]byte(content), &toolResp); err != nil {
			var items []pipeline.ScoreItem
			if err2 := json.Unmarshal([]byte(content), &items); err2 != nil {
				log.Fatalf("TEST 2 FAILED (parse): no tool_calls and content is not JSON:\n  content: %s", content)
			}
			toolResp.Scores = items
		}
	} else {
		log.Fatalf("TEST 2 FAILED: no content and no tool_calls")
	}

	if len(toolResp.Scores) == 0 {
		log.Fatalf("TEST 2 FAILED: empty scores array")
	}

	fmt.Printf("Got %d dimension scores:\n", len(toolResp.Scores))
	weightMap := map[int64]int{1: 25, 2: 35, 3: 25, 4: 15}
	for _, s := range toolResp.Scores {
		fmt.Printf("  - Dim %d: score=%.0f rationale=%q\n", s.DimensionID, s.Score, s.Rationale)
		if s.Score < 0 || s.Score > 100 {
			log.Fatalf("TEST 2 FAILED: score %.0f out of range [0,100]", s.Score)
		}
	}

	total := pipeline.ComputeTotalScore(toolResp.Scores, weightMap)
	fmt.Printf("Total score: %.1f\n\n", total)

	// ======== TEST 3: Verification Function Calling ========
	fmt.Println("=== TEST 3: Verification Function Calling ===")
	verifyMessages := llm.BuildVerificationPrompt(task.Requirements, rawText)
	verifyTool := llm.VerificationToolSchema()

	start = time.Now()
	verifyResp, err := client.Complete(ctx, verifyMessages, []llm.Tool{verifyTool})
	elapsed = time.Since(start)
	if err != nil {
		log.Fatalf("TEST 3 FAILED (LLM call): %v", err)
	}

	fmt.Printf("Verification response in %v\n", elapsed)
	verifyContent := verifyResp.Choices[0].Message.Content
	verifyToolCalls := verifyResp.Choices[0].Message.ToolCalls
	fmt.Printf("Content: %q\n", verifyContent)
	fmt.Printf("Tool calls: %d\n", len(verifyToolCalls))

	var verifyResult pipeline.VerifyToolResponse
	if len(verifyToolCalls) > 0 {
		for _, tc := range verifyToolCalls {
			fmt.Printf("  tool_call: name=%s\n", tc.Function.Name)
			if tc.Function.Name == "submit_verification" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &verifyResult); err != nil {
					log.Fatalf("TEST 3 FAILED (parse tool_call): %v\nargs: %s", err, tc.Function.Arguments)
				}
				break
			}
		}
	} else if verifyContent != "" {
		if err := json.Unmarshal([]byte(verifyContent), &verifyResult); err != nil {
			log.Fatalf("TEST 3 FAILED (parse content): %v\ncontent: %s", err, verifyContent)
		}
	} else {
		log.Fatalf("TEST 3 FAILED: no content and no tool_calls")
	}

	fmt.Printf("Match rate: %.0f%%\n", verifyResult.MatchRate)
	fmt.Printf("  Checkpoints: %v\n", verifyResult.Checkpoints)
	fmt.Printf("  Missing: %v\n", verifyResult.MissingItems)
	fmt.Printf("  Logic issues: %v\n\n", verifyResult.LogicIssues)

	// ======== SUMMARY ========
	fmt.Println("========================================")
	fmt.Println("ALL E2E TESTS PASSED!")
	fmt.Println("========================================")
	fmt.Printf("Model: %s\n", modelName)
	fmt.Printf("Scoring: %d dimensions, total=%.1f\n", len(toolResp.Scores), total)
	fmt.Printf("Verification: match_rate=%.0f%%\n", verifyResult.MatchRate)
}
