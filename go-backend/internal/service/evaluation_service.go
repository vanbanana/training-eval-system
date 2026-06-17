package service

import (
	"context"
	"fmt"
	"math"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// ComputeFinalScore calculates total score with teacher overrides.
// Rule: dimension_final_score = teacher_score if teacher_score IS NOT NULL else ai_score
// total = Σ(dimension_final_score × weight / 100), rounded to 1 decimal.
func ComputeFinalScore(scores []model.DimensionScore, weightMap map[int64]int) float64 {
	var total float64
	for _, sc := range scores {
		weight := weightMap[sc.DimensionID]
		var dimScore float64
		if sc.TeacherScore != nil {
			dimScore = *sc.TeacherScore
		} else if sc.AIScore != nil {
			dimScore = *sc.AIScore
		}
		total += dimScore * float64(weight) / 100.0
	}
	return math.Round(total*10) / 10
}

// EvaluationService handles evaluation operations.
type EvaluationService struct {
	repo     repository.EvaluationRepo
	taskRepo repository.TaskRepo
}

// NewEvaluationService creates a new evaluation service.
func NewEvaluationService(repo repository.EvaluationRepo, taskRepo repository.TaskRepo) *EvaluationService {
	return &EvaluationService{repo: repo, taskRepo: taskRepo}
}

var validEvalTransitions = map[string][]string{
	"pending":   {"scored", "confirmed"},
	"scored":    {"confirmed", "rejected", "pending"},
	"rejected":  {"pending"},
	"confirmed": {},
}

func (s *EvaluationService) checkTransition(current, next string) error {
	if current == next {
		return nil
	}
	if allowed, ok := validEvalTransitions[current]; ok {
		for _, a := range allowed {
			if a == next {
				return nil
			}
		}
	}
	return fmt.Errorf("evaluation_service: invalid status transition from %q to %q", current, next)
}

func (s *EvaluationService) GetByID(ctx context.Context, id int64) (*model.Evaluation, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *EvaluationService) List(ctx context.Context, params repository.EvalListParams) ([]model.Evaluation, int64, error) {
	return s.repo.List(ctx, params)
}

func (s *EvaluationService) Create(ctx context.Context, e *model.Evaluation) error {
	if e.Status == "" {
		e.Status = "pending"
	}
	return s.repo.Create(ctx, e)
}

func (s *EvaluationService) Update(ctx context.Context, e *model.Evaluation) error {
	current, err := s.repo.GetByID(ctx, e.ID)
	if err != nil {
		return err
	}
	if err := s.checkTransition(current.Status, e.Status); err != nil {
		return err
	}
	return s.repo.Update(ctx, e)
}

// SaveScores saves dimension scores and computes total score.
func (s *EvaluationService) SaveScores(ctx context.Context, evalID int64, scores []model.DimensionScore) error {
	eval, err := s.repo.GetByID(ctx, evalID)
	if err != nil {
		return err
	}
	dims, err := s.taskRepo.GetDimensions(ctx, eval.TaskID)
	if err != nil {
		return fmt.Errorf("evaluation_service: get dimensions: %w", err)
	}
	weightMap := make(map[int64]int)
	for _, d := range dims {
		weightMap[d.ID] = d.Weight
	}
	totalScore := ComputeFinalScore(scores, weightMap)
	if err := s.repo.SaveScores(ctx, evalID, scores); err != nil {
		return err
	}
	eval.TotalScore = &totalScore
	eval.Status = "scored"
	return s.repo.Update(ctx, eval)
}

// BatchConfirm confirms multiple evaluations at once.
func (s *EvaluationService) BatchConfirm(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		eval, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if eval.Status != "scored" {
			return fmt.Errorf("evaluation_service: evaluation %d is %s, must be scored", id, eval.Status)
		}
	}
	return s.repo.BatchConfirm(ctx, ids)
}

// OverrideTeacherScore sets teacher_score for a specific dimension and recalculates total.
func (s *EvaluationService) OverrideTeacherScore(ctx context.Context, evalID, dimID int64, teacherScore *float64) error {
	eval, err := s.repo.GetByID(ctx, evalID)
	if err != nil {
		return err
	}
	if eval.Status == "rejected" {
		return fmt.Errorf("evaluation_service: cannot modify a rejected evaluation")
	}
	if err := s.repo.UpdateDimensionTeacherScore(ctx, evalID, dimID, teacherScore); err != nil {
		return err
	}
	dims, err := s.taskRepo.GetDimensions(ctx, eval.TaskID)
	if err != nil {
		return err
	}
	weightMap := make(map[int64]int)
	for _, d := range dims {
		weightMap[d.ID] = d.Weight
	}
	scores, err := s.repo.GetDimensionScores(ctx, evalID)
	if err != nil {
		return err
	}
	totalScore := ComputeFinalScore(scores, weightMap)
	eval.TotalScore = &totalScore
	if err := s.repo.Update(ctx, eval); err != nil {
		return err
	}
	return s.repo.AppendHistory(ctx, &model.EvaluationHistory{
		EvaluationID: evalID,
		OperatorID:   nil,
		Action:       "teacher_override",
	})
}

func (s *EvaluationService) AppendHistory(ctx context.Context, h *model.EvaluationHistory) error {
	return s.repo.AppendHistory(ctx, h)
}

func (s *EvaluationService) GetHistory(ctx context.Context, evalID int64) ([]model.EvaluationHistory, error) {
	return s.repo.GetHistory(ctx, evalID)
}
