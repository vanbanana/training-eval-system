package service

import (
	"context"
	"fmt"
	"math"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// EvaluationService handles evaluation operations.
type EvaluationService struct {
	repo     repository.EvaluationRepo
	taskRepo repository.TaskRepo
}

// NewEvaluationService creates a new evaluation service.
func NewEvaluationService(repo repository.EvaluationRepo, taskRepo repository.TaskRepo) *EvaluationService {
	return &EvaluationService{repo: repo, taskRepo: taskRepo}
}

// validEvalTransitions defines legal evaluation state machine transitions (requirement 13.4).
// Only DB-legal statuses are used here (evaluations.status CHECK: pending/scored/confirmed/rejected).
// Flow: pending → scored → confirmed | rejected; rejected/scored → pending (resubmit/manual fallback).
var validEvalTransitions = map[string][]string{
	"pending":  {"scored", "confirmed"},
	"scored":   {"confirmed", "rejected", "pending"}, // pending = resubmit / manual fallback
	"rejected": {"pending"},                          // resubmission re-enters pipeline
	"confirmed": {},                                   // terminal
}

func (s *EvaluationService) checkTransition(current, next string) error {
	if current == next {
		// No status change (e.g. updating comment/score on the same status).
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
	// Look up the persisted status to validate the real transition (current -> new).
	current, err := s.repo.GetByID(ctx, e.ID)
	if err != nil {
		return err
	}
	if err := s.checkTransition(current.Status, e.Status); err != nil {
		return err
	}
	return s.repo.Update(ctx, e)
}

// BatchConfirm confirms multiple evaluations at once. Valid from scored or manual_required.
func (s *EvaluationService) BatchConfirm(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		eval, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if err := s.checkTransition(eval.Status, "confirmed"); err != nil {
			return err
		}
	}
	return s.repo.BatchConfirm(ctx, ids)
}

// SaveScores saves dimension scores and computes total score.
func (s *EvaluationService) SaveScores(ctx context.Context, evalID int64, scores []model.DimensionScore) error {
	eval, err := s.repo.GetByID(ctx, evalID)
	if err != nil {
		return err
	}

	// Get task dimensions for weights
	dims, err := s.taskRepo.GetDimensions(ctx, eval.TaskID)
	if err != nil {
		return fmt.Errorf("evaluation_service: get dimensions: %w", err)
	}

	// Build weight map
	weightMap := make(map[int64]int)
	for _, d := range dims {
		weightMap[d.ID] = d.Weight
	}

	// Compute weighted total: sum(score * weight / 100)
	var totalScore float64
	for _, sc := range scores {
		weight := weightMap[sc.DimensionID]
		score := 0.0
		if sc.TeacherScore != nil {
			score = *sc.TeacherScore
		} else if sc.AIScore != nil {
			score = *sc.AIScore
		}
		totalScore += score * float64(weight) / 100.0
	}

	// Round to 1 decimal place
	totalScore = math.Round(totalScore*10) / 10

	if err := s.repo.SaveScores(ctx, evalID, scores); err != nil {
		return err
	}

	eval.TotalScore = &totalScore
	eval.Status = "scored"
	return s.repo.Update(ctx, eval)
}

// AppendHistory records a change to an evaluation.
func (s *EvaluationService) AppendHistory(ctx context.Context, h *model.EvaluationHistory) error {
	return s.repo.AppendHistory(ctx, h)
}

// GetHistory returns the history entries for an evaluation.
func (s *EvaluationService) GetHistory(ctx context.Context, evalID int64) ([]model.EvaluationHistory, error) {
	return s.repo.GetHistory(ctx, evalID)
}
