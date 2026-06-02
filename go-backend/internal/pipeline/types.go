// Package pipeline orchestrates the evaluation processing stages.
package pipeline

import (
	"encoding/json"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/llm"
)

// ScoreToolResponse represents the parsed JSON from Function Calling submit_scores.
type ScoreToolResponse struct {
	Scores []ScoreItem `json:"scores"`
}

// ScoreItem represents a single dimension score from the LLM.
type ScoreItem struct {
	DimensionID int64   `json:"dimension_id"`
	Score       float64 `json:"score"`
	Rationale   string  `json:"rationale"`
}

// VerifyToolResponse represents the parsed JSON from Function Calling submit_verification.
type VerifyToolResponse struct {
	MatchRate    float64  `json:"match_rate"`
	Checkpoints  []string `json:"checkpoints"`
	MissingItems []string `json:"missing_items"`
	LogicIssues  []string `json:"logic_issues"`
}

// parseScoreToolCall extracts ScoreItems from a ChatResponse tool_call.
func parseScoreToolCall(resp *llm.ChatResponse) ([]ScoreItem, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	msg := resp.Choices[0].Message

	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == "submit_scores" {
				var toolResp ScoreToolResponse
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &toolResp); err != nil {
					return nil, fmt.Errorf("parse tool_call arguments: %w", err)
				}
				if len(toolResp.Scores) == 0 {
					return nil, fmt.Errorf("empty scores in tool_call")
				}
				return toolResp.Scores, nil
			}
		}
		return nil, fmt.Errorf("no submit_scores tool_call found in response")
	}

	if msg.Content == "" {
		return nil, fmt.Errorf("no content and no tool_calls in response")
	}

	var toolResp ScoreToolResponse
	if err := json.Unmarshal([]byte(msg.Content), &toolResp); err != nil {
		var items []ScoreItem
		if err2 := json.Unmarshal([]byte(msg.Content), &items); err2 != nil {
			return nil, fmt.Errorf("cannot parse response (no tool_calls, content is not JSON)")
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("empty scores array in content")
		}
		return items, nil
	}

	if len(toolResp.Scores) == 0 {
		return nil, fmt.Errorf("empty scores array in response")
	}

	return toolResp.Scores, nil
}

// parseVerifyToolCall extracts VerifyToolResponse from a ChatResponse tool_call.
func parseVerifyToolCall(resp *llm.ChatResponse) (*VerifyToolResponse, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	msg := resp.Choices[0].Message

	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == "submit_verification" {
				var result VerifyToolResponse
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &result); err != nil {
					return nil, fmt.Errorf("parse tool_call arguments: %w", err)
				}
				return &result, nil
			}
		}
		return nil, fmt.Errorf("no submit_verification tool_call found")
	}

	if msg.Content == "" {
		return nil, fmt.Errorf("no content and no tool_calls in response")
	}

	var result VerifyToolResponse
	if err := json.Unmarshal([]byte(msg.Content), &result); err != nil {
		return nil, fmt.Errorf("parse verification response: %w", err)
	}

	return &result, nil
}

func formatResponse(resp *llm.ChatResponse) string {
	if resp == nil {
		return "<nil>"
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func intPtr(v int64) *int64 {
	return &v
}
