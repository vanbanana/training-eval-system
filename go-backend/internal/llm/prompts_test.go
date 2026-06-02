package llm

import (
	"strings"
	"testing"

	"github.com/smartedu/training-eval-system/internal/model"
	"pgregory.net/rapid"
)

// Property 4: Scoring prompt contains all dimensions, weights, requirements, raw_text
func TestProperty_ScoringPromptCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		task := &model.TrainingTask{
			Name:         rapid.StringMatching(`[a-z]{5,20}`).Draw(t, "name"),
			Description:  rapid.StringMatching(`[a-z]{10,50}`).Draw(t, "desc"),
			Requirements: rapid.StringMatching(`[a-z]{10,50}`).Draw(t, "reqs"),
		}
		numDims := rapid.IntRange(1, 5).Draw(t, "numDims")
		dims := make([]model.Dimension, numDims)
		for i := range dims {
			dims[i] = model.Dimension{
				ID:     int64(i + 1),
				Name:   rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "dim_name"),
				Weight: rapid.IntRange(1, 50).Draw(t, "weight"),
			}
		}
		rawText := rapid.StringMatching(`[a-z]{20,100}`).Draw(t, "raw_text")

		messages := BuildScoringPrompt(task, dims, rawText)

		// Concatenate all message content
		var full strings.Builder
		for _, m := range messages {
			full.WriteString(m.Content)
		}
		combined := full.String()

		// Verify all required content is present
		if !strings.Contains(combined, task.Name) {
			t.Fatalf("prompt missing task name %q", task.Name)
		}
		if !strings.Contains(combined, task.Requirements) {
			t.Fatalf("prompt missing requirements")
		}
		if !strings.Contains(combined, rawText) {
			t.Fatalf("prompt missing raw_text")
		}
		for _, d := range dims {
			if !strings.Contains(combined, d.Name) {
				t.Fatalf("prompt missing dimension name %q", d.Name)
			}
		}
	})
}

// Property 11: Chat context truncation - raw_text portion ≤ 4000 chars
func TestProperty_ChatContextTruncation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate raw_text of varying length (0 to 10000)
		length := rapid.IntRange(0, 10000).Draw(t, "length")
		rawText := strings.Repeat("中", length) // Chinese chars, 3 bytes each but 1 rune

		task := &model.TrainingTask{Name: "test", Description: "d", Requirements: "r"}
		eval := &model.Evaluation{}

		prompt := BuildChatSystemPrompt(task, eval, rawText)

		// The raw_text in the prompt should be at most 4000 characters
		// Find the "学生提交内容" section
		idx := strings.Index(prompt, "学生提交内容")
		if idx < 0 && length > 0 {
			t.Fatalf("prompt should contain student content section for non-empty text")
		}
		if idx >= 0 {
			section := prompt[idx:]
			// Count characters in the section (rough check: total prompt - before section)
			if len([]rune(section)) > 4200 { // 4000 + header overhead
				t.Fatalf("raw_text section too long: %d runes (input was %d)", len([]rune(section)), length)
			}
		}
	})
}

// Unit test: BuildScoringPrompt structure
func TestBuildScoringPrompt_Structure(t *testing.T) {
	task := &model.TrainingTask{
		Name: "并发编程", Description: "实现生产者消费者", Requirements: "源代码+报告",
	}
	dims := []model.Dimension{
		{ID: 1, Name: "代码规范", Weight: 40},
		{ID: 2, Name: "功能完整", Weight: 60},
	}

	messages := BuildScoringPrompt(task, dims, "测试文本内容")

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("first message should be system, got %s", messages[0].Role)
	}
	if messages[1].Role != "user" {
		t.Errorf("second message should be user, got %s", messages[1].Role)
	}
	if !strings.Contains(messages[1].Content, "40%") {
		t.Error("user message should contain weight percentage")
	}
}

// Unit test: BuildChatSystemPrompt truncation
func TestBuildChatSystemPrompt_Truncation(t *testing.T) {
	longText := strings.Repeat("a", 8000)
	task := &model.TrainingTask{Name: "t", Description: "d", Requirements: "r"}

	prompt := BuildChatSystemPrompt(task, nil, longText)

	// The 8000 char text should be truncated to 4000
	count := strings.Count(prompt, "a")
	if count > 4000 {
		t.Errorf("expected max 4000 'a' chars, got %d", count)
	}
	if count < 3900 {
		t.Errorf("expected ~4000 'a' chars, got %d (too aggressive truncation)", count)
	}
}

// Unit test: ScoringToolSchema
func TestScoringToolSchema_Valid(t *testing.T) {
	dims := []model.Dimension{{ID: 1, Name: "A", Weight: 50}, {ID: 2, Name: "B", Weight: 50}}
	tool := ScoringToolSchema(dims)

	if tool.Type != "function" {
		t.Errorf("expected type 'function', got %q", tool.Type)
	}
	if tool.Function.Name != "submit_scores" {
		t.Errorf("expected name 'submit_scores', got %q", tool.Function.Name)
	}
	if len(tool.Function.Parameters) == 0 {
		t.Error("parameters should not be empty")
	}
}

// Unit test: VerificationToolSchema
func TestVerificationToolSchema_Valid(t *testing.T) {
	tool := VerificationToolSchema()
	if tool.Function.Name != "submit_verification" {
		t.Errorf("expected 'submit_verification', got %q", tool.Function.Name)
	}
}
