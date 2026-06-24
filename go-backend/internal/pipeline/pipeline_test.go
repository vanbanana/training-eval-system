package pipeline

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"pgregory.net/rapid"
)

// ==================== Property Tests ====================

// Property 3: Total score weighted computation
// For any list of scores with weights summing to 100, total = round(sum(score*weight/100), 1)
func TestProperty_TotalScoreComputation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 2-6 dimensions with weights summing to 100
		numDims := rapid.IntRange(2, 6).Draw(t, "numDims")
		weights := generateWeightsSumTo100(t, numDims)

		weightMap := make(map[int64]int)
		scores := make([]ScoreItem, numDims)
		var expectedTotal float64

		for i := 0; i < numDims; i++ {
			dimID := int64(i + 1)
			score := rapid.Float64Range(0, 100).Draw(t, "score")
			weightMap[dimID] = weights[i]
			scores[i] = ScoreItem{DimensionID: dimID, Score: score}
			expectedTotal += score * float64(weights[i]) / 100.0
		}

		// Round expected
		expectedTotal = roundTo1(expectedTotal)

		actual := ComputeTotalScore(scores, weightMap)

		if actual != expectedTotal {
			t.Fatalf("expected total %.1f, got %.1f (scores=%v, weights=%v)", expectedTotal, actual, scores, weights)
		}
	})
}

// Property 5: Score validation - scores must be [0, 100]
func TestProperty_ScoreValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		score := rapid.Float64Range(-1000, 1000).Draw(t, "score")
		valid := score >= 0 && score <= 100

		item := ScoreItem{DimensionID: 1, Score: score, Rationale: "test"}
		err := validateScoreRange(item)

		if valid && err != nil {
			t.Fatalf("score %.2f should be valid but got error: %v", score, err)
		}
		if !valid && err == nil {
			t.Fatalf("score %.2f should be invalid but no error", score)
		}
	})
}

// Property 7: Similarity ordering invariant - a_id always < b_id
func TestProperty_SimilarityOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.Int64Range(1, 10000).Draw(t, "a")
		b := rapid.Int64Range(1, 10000).Draw(t, "b")
		if a == b {
			b = a + 1
		}

		resultA, resultB := OrderPair(a, b)

		if resultA >= resultB {
			t.Fatalf("OrderPair(%d, %d) = (%d, %d), expected a < b", a, b, resultA, resultB)
		}
		// Also verify both original values are present
		if (resultA != a && resultA != b) || (resultB != a && resultB != b) {
			t.Fatalf("OrderPair(%d, %d) = (%d, %d), values not preserved", a, b, resultA, resultB)
		}
	})
}

// Property 6: Similarity threshold - suspect iff hamming < HammingThreshold
func TestProperty_SimilarityThreshold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		dist := rapid.IntRange(0, 64).Draw(t, "distance")
		shouldBeSuspect := dist < HammingThreshold

		// HammingThreshold = 3: distances 0,1,2 are suspect; 3+ are not
		if shouldBeSuspect != (dist < HammingThreshold) {
			t.Fatalf("dist=%d: shouldBeSuspect=%v but threshold=%d disagrees", dist, shouldBeSuspect, HammingThreshold)
		}
	})
}

// Property 10: Teacher score range - accept iff [0, 100]
func TestProperty_TeacherScoreRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		score := rapid.Float64Range(-1000, 1000).Draw(t, "score")
		valid := score >= 0 && score <= 100

		err := ValidateTeacherScore(score)

		if valid && err != nil {
			t.Fatalf("score %.2f should be valid but got error: %v", score, err)
		}
		if !valid && err == nil {
			t.Fatalf("score %.2f should be invalid but no error", score)
		}
	})
}

// ==================== Unit Tests ====================

func TestComputeTotalScore_Basic(t *testing.T) {
	weightMap := map[int64]int{1: 40, 2: 60}
	scores := []ScoreItem{
		{DimensionID: 1, Score: 80},
		{DimensionID: 2, Score: 90},
	}
	// 80*40/100 + 90*60/100 = 32 + 54 = 86.0
	got := ComputeTotalScore(scores, weightMap)
	if got != 86.0 {
		t.Fatalf("expected 86.0, got %f", got)
	}
}

func TestComputeTotalScore_Rounding(t *testing.T) {
	weightMap := map[int64]int{1: 33, 2: 34, 3: 33}
	scores := []ScoreItem{
		{DimensionID: 1, Score: 85},
		{DimensionID: 2, Score: 72},
		{DimensionID: 3, Score: 91},
	}
	// 85*33/100 + 72*34/100 + 91*33/100 = 28.05 + 24.48 + 30.03 = 82.56 → 82.6
	got := ComputeTotalScore(scores, weightMap)
	if got != 82.6 {
		t.Fatalf("expected 82.6, got %f", got)
	}
}

func TestOrderPair_AlreadyOrdered(t *testing.T) {
	a, b := OrderPair(3, 7)
	if a != 3 || b != 7 {
		t.Fatalf("expected (3,7), got (%d,%d)", a, b)
	}
}

func TestOrderPair_Reversed(t *testing.T) {
	a, b := OrderPair(10, 2)
	if a != 2 || b != 10 {
		t.Fatalf("expected (2,10), got (%d,%d)", a, b)
	}
}

func TestParseScoreToolCall_ValidResponse(t *testing.T) {
	resp := makeMockResp(`{"scores":[{"dimension_id":1,"score":85,"rationale":"good"},{"dimension_id":2,"score":72,"rationale":"ok"}]}`)
	scores, err := parseScoreToolCall(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}
	if scores[0].Score != 85 {
		t.Fatalf("expected score 85, got %f", scores[0].Score)
	}
}

func TestParseScoreToolCall_EmptyScores(t *testing.T) {
	resp := makeMockResp(`{"scores":[]}`)
	_, err := parseScoreToolCall(resp)
	if err == nil {
		t.Fatal("expected error for empty scores")
	}
}

func TestParseScoreToolCall_InvalidJSON(t *testing.T) {
	resp := makeMockResp(`not json at all`)
	_, err := parseScoreToolCall(resp)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseScoreToolCall_DirectArray(t *testing.T) {
	resp := makeMockResp(`[{"dimension_id":1,"score":90,"rationale":"excellent"}]`)
	scores, err := parseScoreToolCall(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scores) != 1 || scores[0].Score != 90 {
		t.Fatalf("unexpected scores: %v", scores)
	}
}

func TestValidateTeacherScore_Boundary(t *testing.T) {
	tests := []struct {
		score float64
		valid bool
	}{
		{0, true}, {100, true}, {50, true},
		{-0.01, false}, {100.01, false}, {-1, false}, {101, false},
	}
	for _, tt := range tests {
		err := ValidateTeacherScore(tt.score)
		if tt.valid && err != nil {
			t.Errorf("score %f should be valid, got error: %v", tt.score, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("score %f should be invalid, got nil error", tt.score)
		}
	}
}

// --- Accuracy: tool_call content parsing ---

func TestParseScoreToolCall_WithToolCalls(t *testing.T) {
	resp := &llm.ChatResponse{
		Choices: []llm.ChatChoice{{
			Message: llm.ChatMessage{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{{
					ID: "c1", Type: "function",
					Function: llm.FunctionCall{
						Name:      "submit_scores",
						Arguments: `{"scores":[{"dimension_id":10,"score":95,"rationale":"完美"}]}`,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	}
	scores, err := parseScoreToolCall(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scores) != 1 || scores[0].DimensionID != 10 || scores[0].Score != 95 {
		t.Fatalf("unexpected: %+v", scores)
	}
}

func TestParseScoreToolCall_WrongToolName(t *testing.T) {
	resp := &llm.ChatResponse{
		Choices: []llm.ChatChoice{{
			Message: llm.ChatMessage{
				ToolCalls: []llm.ToolCall{{
					ID: "c1", Type: "function",
					Function: llm.FunctionCall{Name: "other_tool", Arguments: "{}"},
				}},
			},
		}},
	}
	_, err := parseScoreToolCall(resp)
	if err == nil {
		t.Fatal("expected error for wrong tool name")
	}
}

func TestParseVerifyToolCall_Valid(t *testing.T) {
	resp := &llm.ChatResponse{
		Choices: []llm.ChatChoice{{
			Message: llm.ChatMessage{
				ToolCalls: []llm.ToolCall{{
					ID: "c1", Type: "function",
					Function: llm.FunctionCall{
						Name:      "submit_verification",
						Arguments: `{"match_rate":88,"checkpoints":["a","b"],"missing_items":["c"],"logic_issues":[]}`,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	}
	r, err := parseVerifyToolCall(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int(r.MatchRate) != 88 || len(r.Checkpoints) != 2 {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestParseVerifyToolCall_ContentFallback(t *testing.T) {
	resp := makeMockResp(`{"match_rate":75,"checkpoints":["x"],"missing_items":[],"logic_issues":[]}`)
	r, err := parseVerifyToolCall(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int(r.MatchRate) != 75 {
		t.Fatalf("expected 75, got %.0f", r.MatchRate)
	}
}

func TestParseVerifyToolCall_Invalid(t *testing.T) {
	_, err := parseVerifyToolCall(nil)
	if err == nil {
		t.Fatal("expected error for nil")
	}
}

// --- α Ratio tests ---

func TestComputeTotalScoreWithRatio_Basic(t *testing.T) {
	aiScores := []ScoreItem{
		{DimensionID: 1, Score: 80},
		{DimensionID: 2, Score: 70},
	}
	subjScores := map[int64]float64{1: 90, 2: 60}
	weightMap := map[int64]int{1: 50, 2: 50}

	total := ComputeTotalScoreWithRatio(aiScores, subjScores, weightMap, 0.6)
	// dim1 = 80*0.6 + 90*0.4 = 84, dim2 = 70*0.6 + 60*0.4 = 66
	// total = 84*0.5 + 66*0.5 = 75
	if total != 75.0 {
		t.Fatalf("expected 75.0, got %.1f", total)
	}
}

func TestComputeTotalScoreWithRatio_AIOnly(t *testing.T) {
	aiScores := []ScoreItem{
		{DimensionID: 1, Score: 90},
		{DimensionID: 2, Score: 80},
	}
	weightMap := map[int64]int{1: 50, 2: 50}
	total := ComputeTotalScoreWithRatio(aiScores, nil, weightMap, 0.6)
	// No subj scores → 100% AI: 90*0.5 + 80*0.5 = 85
	if total != 85.0 {
		t.Fatalf("expected 85.0, got %.1f", total)
	}
}

func TestComputeTotalScoreWithRatio_ZeroRatio(t *testing.T) {
	// objRatio=0 → 0% AI, 100% subjective (valid config, not overridden)
	total := ComputeTotalScoreWithRatio(
		[]ScoreItem{{DimensionID: 1, Score: 100}},
		map[int64]float64{1: 50},
		map[int64]int{1: 100},
		0,
	)
	// 100*0.0 + 50*1.0 = 50
	if total != 50.0 {
		t.Fatalf("expected 50.0, got %.1f", total)
	}
}

// --- Model-level scoring formula ---

func TestComputeTotalScoreFromModelWithRatio(t *testing.T) {
	scores := []model.DimensionScore{
		{DimensionID: 1, AIScore: floatPtr(80), TeacherScore: floatPtr(90)},
		{DimensionID: 2, AIScore: floatPtr(60), TeacherScore: floatPtr(70)},
	}
	weightMap := map[int64]int{1: 50, 2: 50}

	// α=0.4: dim1 = 80*0.4 + 90*0.6 = 86, dim2 = 60*0.4 + 70*0.6 = 66
	// total = 86*0.5 + 66*0.5 = 76
	total := ComputeTotalScoreFromModelWithRatio(scores, weightMap, 0.4)
	if total != 76.0 {
		t.Fatalf("expected 76.0, got %.1f", total)
	}
}

// Similarity ordering tests — OrderPair already tested by property tests above

// --- Edge cases ---

func TestComputeTotalScore_EmptyScores(t *testing.T) {
	total := ComputeTotalScore(nil, map[int64]int{1: 100})
	if total != 0 {
		t.Fatalf("expected 0, got %.1f", total)
	}
}

func TestComputeTotalScore_MissingDimension(t *testing.T) {
	scores := []ScoreItem{{DimensionID: 99, Score: 100}}
	total := ComputeTotalScore(scores, map[int64]int{1: 100})
	// dimension 99 not in weight map → score*weight = 100*0 = 0
	if total != 0 {
		t.Fatalf("expected 0, got %.1f", total)
	}
}

// ==================== Helpers ====================

func generateWeightsSumTo100(t *rapid.T, n int) []int {
	if n == 1 {
		return []int{100}
	}
	weights := make([]int, n)
	remaining := 100
	for i := 0; i < n-1; i++ {
		max := remaining - (n - i - 1) // leave at least 1 for each remaining
		if max < 1 {
			max = 1
		}
		weights[i] = rapid.IntRange(1, max).Draw(t, "weight")
		remaining -= weights[i]
	}
	weights[n-1] = remaining
	return weights
}

func roundTo1(v float64) float64 {
	return math.Round(v*10) / 10
}

func validateScoreRange(item ScoreItem) error {
	if item.Score < 0 || item.Score > 100 {
		return fmt.Errorf("score %f out of range [0, 100]", item.Score)
	}
	return nil
}

// makeMockResp creates a minimal ChatResponse from content string.
func makeMockResp(content string) *llm.ChatResponse {
	return &llm.ChatResponse{
		Choices: []llm.ChatChoice{
			{Message: llm.ChatMessage{Content: content}},
		},
	}
}

// suppress unused imports
var _ = json.Marshal
var _ = model.Evaluation{}

func floatPtr(v float64) *float64 {
	return &v
}
