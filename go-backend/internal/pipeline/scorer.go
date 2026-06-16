package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/sse"
)

// DefaultObjectiveRatio is the default weight for AI objective score (0.6 = 60%).
const DefaultObjectiveRatio = 0.6

// Scorer handles LLM-based evaluation scoring via Function Calling.
type Scorer struct {
	client        *llm.Client
	evalRepo      repository.EvaluationRepo
	taskRepo      repository.TaskRepo
	systemCfgRepo repository.SystemConfigRepo
	broker        *sse.Broker
}

// SetSystemConfigRepo sets the system config repository for reading runtime config.
func (s *Scorer) SetSystemConfigRepo(repo repository.SystemConfigRepo) {
	s.systemCfgRepo = repo
}

// Score executes the scoring pipeline for a single evaluation.
func (s *Scorer) Score(ctx context.Context, evalID int64, rawText string) error {
	eval, err := s.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("scorer: load evaluation: %w", err)
	}

	task, err := s.taskRepo.GetByID(ctx, eval.TaskID)
	if err != nil {
		return fmt.Errorf("scorer: load task: %w", err)
	}

	dims, err := s.taskRepo.GetDimensions(ctx, eval.TaskID)
	if err != nil {
		return fmt.Errorf("scorer: load dimensions: %w", err)
	}

	if len(dims) == 0 {
		return fmt.Errorf("scorer: task has no dimensions")
	}

	// Read objective ratio from system_config or use default
	objRatio := s.getObjectiveRatio(ctx)

	// Build prompt and tool schema
	messages := llm.BuildScoringPrompt(task, dims, rawText)
	tool := llm.ScoringToolSchema(dims)

	// Attempt scoring with retries for missing tool_call
	var scores []ScoreItem
	maxAttempts := 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := s.client.Complete(ctx, messages, []llm.Tool{tool})
		if err != nil {
			slog.Error("scorer: LLM call failed", "eval_id", evalID, "attempt", attempt, "error", err.Error())
			if attempt == maxAttempts-1 {
				// Final attempt failed — set evaluation to manual mode
				return s.markManualRequired(ctx, eval, fmt.Sprintf("LLM scoring failed after %d attempts: %v", maxAttempts, err))
			}
			continue
		}

		scores, err = parseScoreToolCall(resp)
		if err != nil {
			slog.Warn("scorer: parse tool_call failed, retrying", "eval_id", evalID, "attempt", attempt, "error", err.Error())
			if attempt == maxAttempts-1 {
				slog.Error("scorer: no valid tool_call after retries", "eval_id", evalID, "raw_response", formatResponse(resp))
				return s.markManualRequired(ctx, eval, fmt.Sprintf("No valid tool_call after %d attempts", maxAttempts))
			}
			continue
		}
		break
	}

	// Validate scores
	for _, sc := range scores {
		if sc.Score < 0 || sc.Score > 100 {
			return fmt.Errorf("scorer: invalid score %f for dimension %d (must be 0-100)", sc.Score, sc.DimensionID)
		}
	}

	// Build weight map
	weightMap := make(map[int64]int)
	for _, d := range dims {
		weightMap[d.ID] = d.Weight
	}

	// Save dimension scores (store both AI and initial subjective scores)
	modelScores := make([]model.DimensionScore, 0, len(scores))
	for _, sc := range scores {
		aiScore := sc.Score
		// Initialize subjective score as nil (teacher can fill in later)
		modelScores = append(modelScores, model.DimensionScore{
			EvaluationID: evalID,
			DimensionID:  sc.DimensionID,
			AIScore:      &aiScore,
			Rationale:    sc.Rationale,
		})
	}

	if err := s.evalRepo.SaveScores(ctx, evalID, modelScores); err != nil {
		return fmt.Errorf("scorer: save scores: %w", err)
	}

	// Compute total score using α-weighted formula: when no subjective scores yet, uses 100% AI.
	totalScore := ComputeTotalScoreWithRatio(scores, nil, weightMap, objRatio)
	eval.TotalScore = &totalScore
	eval.ObjectiveRatio = &objRatio
	eval.Status = "scored"

	if err := s.evalRepo.Update(ctx, eval); err != nil {
		return fmt.Errorf("scorer: update evaluation: %w", err)
	}

	// Append history
	_ = s.evalRepo.AppendHistory(ctx, &model.EvaluationHistory{
		EvaluationID: evalID,
		OperatorID:   intPtr(0), // system
		Action:       "ai_scored",
		AfterValue:   scores,
	})

	// Publish SSE
	data, _ := json.Marshal(map[string]any{
		"evaluation_id":   evalID,
		"upload_id":       eval.UploadID,
		"total_score":     totalScore,
		"objective_ratio": objRatio,
	})
	s.broker.Publish(sse.Event{
		UserID: task.TeacherID,
		Type:   "score_complete",
		Data:   string(data),
	})

	slog.Info("scorer: evaluation scored", "eval_id", evalID, "total_score", totalScore, "obj_ratio", objRatio)
	return nil
}

// RecomputeWithSubjective recalculates total score when teacher provides subjective scores.
// Uses the α-weighted formula: final = Σ(weight × (ai × α + subj × (1-α))) / 100
func (s *Scorer) RecomputeWithSubjective(ctx context.Context, evalID int64) error {
	eval, err := s.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("scorer: load evaluation: %w", err)
	}

	dims, err := s.taskRepo.GetDimensions(ctx, eval.TaskID)
	if err != nil {
		return fmt.Errorf("scorer: load dimensions: %w", err)
	}

	weightMap := make(map[int64]int)
	for _, d := range dims {
		weightMap[d.ID] = d.Weight
	}

	objRatio := DefaultObjectiveRatio
	if eval.ObjectiveRatio != nil {
		objRatio = *eval.ObjectiveRatio
	}

	// Read current scores from DB (includes teacher subjective scores)
	fullEval, err := s.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return err
	}

	totalScore := ComputeTotalScoreFromModelWithRatio(fullEval.Scores, weightMap, objRatio)
	eval.TotalScore = &totalScore

	if err := s.evalRepo.Update(ctx, eval); err != nil {
		return fmt.Errorf("scorer: update recomputed evaluation: %w", err)
	}

	// Append history
	_ = s.evalRepo.AppendHistory(ctx, &model.EvaluationHistory{
		EvaluationID: evalID,
		OperatorID:   intPtr(0), // system recompute
		Action:       "score_recomputed",
	})

	slog.Info("scorer: scores recomputed with subjective", "eval_id", evalID, "total", totalScore, "obj_ratio", objRatio)
	return nil
}

// UpdateObjectiveRatio changes the objective-to-subjective weighting and recomputes.
func (s *Scorer) UpdateObjectiveRatio(ctx context.Context, evalID int64, newRatio float64) error {
	if newRatio < 0 || newRatio > 1 {
		return fmt.Errorf("scorer: objective ratio must be 0-1, got %f", newRatio)
	}

	eval, err := s.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("scorer: load evaluation: %w", err)
	}

	eval.ObjectiveRatio = &newRatio
	if err := s.evalRepo.Update(ctx, eval); err != nil {
		return fmt.Errorf("scorer: update ratio: %w", err)
	}

	return s.RecomputeWithSubjective(ctx, evalID)
}

func (s *Scorer) getObjectiveRatio(ctx context.Context) float64 {
	if s.systemCfgRepo != nil {
		cfg, err := s.systemCfgRepo.GetByKey(ctx, "evaluation.objective_ratio")
		if err == nil && cfg != nil {
			// cfg.Value is stored as any (float64 from JSON number), handle both types
			switch v := cfg.Value.(type) {
			case float64:
				if v >= 0 && v <= 1 {
					return v
				}
			case json.Number:
				if f, err := v.Float64(); err == nil && f >= 0 && f <= 1 {
					return f
				}
			}
		}
	}
	return DefaultObjectiveRatio
}

func (s *Scorer) markManualRequired(ctx context.Context, eval *model.Evaluation, reason string) error {
	// evaluations.status only accepts pending/scored/confirmed/rejected. When AI
	// scoring fails, the evaluation stays "pending" so a teacher can score it
	// manually; the reason is preserved in the comment and history.
	eval.Status = "pending"
	eval.OverallComment = reason
	if err := s.evalRepo.Update(ctx, eval); err != nil {
		return fmt.Errorf("scorer: mark manual_required: %w", err)
	}

	// Append history
	_ = s.evalRepo.AppendHistory(ctx, &model.EvaluationHistory{
		EvaluationID: eval.ID,
		OperatorID:   intPtr(0),
		Action:       "ai_failed_manual_required",
		AfterValue:   reason,
	})

	slog.Warn("scorer: AI scoring failed, evaluation left pending for manual scoring", "eval_id", eval.ID, "reason", reason)
	return nil
}

// ComputeTotalScore calculates weighted total from scores and weight map.
// Uses ai_score only (legacy, no subjective scores).
func ComputeTotalScore(scores []ScoreItem, weightMap map[int64]int) float64 {
	return ComputeTotalScoreWithRatio(scores, nil, weightMap, 1.0)
}

// ComputeTotalScoreWithRatio computes composite score using α-weighted formula.
// total = Σ(weight_i × (ai_i × α + subj_i × (1-α))) / 100
// If subjScores is nil or missing, uses 100% AI score.
func ComputeTotalScoreWithRatio(aiScores []ScoreItem, subjScores map[int64]float64, weightMap map[int64]int, objRatio float64) float64 {
	if objRatio <= 0 {
		objRatio = DefaultObjectiveRatio
	}

	// Build lookup for fast access
	subjMap := subjScores
	if subjMap == nil {
		subjMap = make(map[int64]float64)
	}

	var total float64
	for _, sc := range aiScores {
		weight := weightMap[sc.DimensionID]
		aiScore := sc.Score
		subjScore, hasSubj := subjMap[sc.DimensionID]

		var finalDimScore float64
		if hasSubj {
			finalDimScore = aiScore*objRatio + subjScore*(1-objRatio)
		} else {
			finalDimScore = aiScore // 100% AI when no subjective score
		}

		total += finalDimScore * float64(weight) / 100.0
	}
	return math.Round(total*10) / 10
}

// ComputeTotalScoreFromModel calculates weighted total from model scores.
// Prefers teacher_score over ai_score.
func ComputeTotalScoreFromModel(scores []model.DimensionScore, weightMap map[int64]int) float64 {
	return ComputeTotalScoreFromModelWithRatio(scores, weightMap, DefaultObjectiveRatio)
}

// ComputeTotalScoreFromModelWithRatio uses α-weighted formula on model scores.
func ComputeTotalScoreFromModelWithRatio(scores []model.DimensionScore, weightMap map[int64]int, objRatio float64) float64 {
	if objRatio <= 0 {
		objRatio = DefaultObjectiveRatio
	}

	var total float64
	for _, sc := range scores {
		weight := weightMap[sc.DimensionID]
		aiScore := 0.0
		if sc.AIScore != nil {
			aiScore = *sc.AIScore
		}
		subjScore := 0.0
		hasSubj := false
		if sc.TeacherScore != nil {
			subjScore = *sc.TeacherScore
			hasSubj = true
		}

		var finalDimScore float64
		if hasSubj {
			finalDimScore = aiScore*objRatio + subjScore*(1-objRatio)
		} else {
			finalDimScore = aiScore
		}

		total += finalDimScore * float64(weight) / 100.0
	}
	return math.Round(total*10) / 10
}
