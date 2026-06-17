package service

import (
	"context"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// TaskService handles training task operations.
type TaskService struct {
	repo      repository.TaskRepo
	classRepo repository.ClassRepo
	notifSvc  *NotificationService
}

// NewTaskService creates a new task service.
func NewTaskService(repo repository.TaskRepo, classRepo repository.ClassRepo, notifSvc *NotificationService) *TaskService {
	return &TaskService{repo: repo, classRepo: classRepo, notifSvc: notifSvc}
}

// ValidateTaskClassesBelongToCourse checks that all classIDs belong to the given course.
func (s *TaskService) ValidateTaskClassesBelongToCourse(ctx context.Context, courseID int64, classIDs []int64) error {
	for _, cid := range classIDs {
		cls, err := s.classRepo.GetByID(ctx, cid)
		if err != nil {
			return fmt.Errorf("task_service: class %d: %w", cid, err)
		}
		if cls.IsArchived {
			return fmt.Errorf("task_service: class %d is archived", cid)
		}
		if cls.CourseID != courseID {
			return fmt.Errorf("task_service: class %d belongs to course %d, not course %d", cid, cls.CourseID, courseID)
		}
	}
	return nil
}

// validTaskTransitions defines legal state machine transitions.
var validTaskTransitions = map[string][]string{
	"":          {"draft"},
	"draft":     {"published"},
	"published": {"closed"},
}

func (s *TaskService) checkTransition(current, next string) error {
	if allowed, ok := validTaskTransitions[current]; ok {
		for _, a := range allowed {
			if a == next {
				return nil
			}
		}
	}
	return fmt.Errorf("task_service: invalid status transition from %q to %q", current, next)
}

func (s *TaskService) GetByID(ctx context.Context, id int64) (*model.TrainingTask, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TaskService) List(ctx context.Context, params repository.TaskListParams) ([]model.TrainingTask, int64, error) {
	return s.repo.List(ctx, params)
}

func (s *TaskService) Create(ctx context.Context, t *model.TrainingTask) error {
	if t.Status == "" {
		t.Status = "draft"
	}
	return s.repo.Create(ctx, t)
}

func (s *TaskService) Update(ctx context.Context, t *model.TrainingTask) error {
	return s.repo.Update(ctx, t)
}

func (s *TaskService) Delete(ctx context.Context, id int64) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if task.Status != "draft" {
		return fmt.Errorf("task_service: can only delete draft tasks")
	}
	return s.repo.Delete(ctx, id)
}

// SetClasses validates class-course ownership then sets task classes.
func (s *TaskService) SetClasses(ctx context.Context, taskID int64, classIDs []int64) error {
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if len(classIDs) > 0 {
		if err := s.ValidateTaskClassesBelongToCourse(ctx, task.CourseID, classIDs); err != nil {
			return err
		}
	} else {
		if task.Status == "published" || task.Status == "closed" {
			return fmt.Errorf("task_service: cannot clear classes on a %s task", task.Status)
		}
	}
	return s.repo.SetClasses(ctx, taskID, classIDs)
}

// Publish transitions a task from draft to published.
func (s *TaskService) Publish(ctx context.Context, id int64) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.checkTransition(task.Status, "published"); err != nil {
		return err
	}
	if err := s.repo.EnsureTaskHasClasses(ctx, id); err != nil {
		return fmt.Errorf("task_service: %w", err)
	}
	return s.repo.UpdateStatus(ctx, id, "published")
}

// Close transitions a task from published to closed.
func (s *TaskService) Close(ctx context.Context, id int64) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.checkTransition(task.Status, "closed"); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, "closed")
}

func (s *TaskService) SetDimensions(ctx context.Context, taskID int64, dims []model.Dimension) error {
	return s.repo.SetDimensions(ctx, taskID, dims)
}

func (s *TaskService) GetDimensions(ctx context.Context, taskID int64) ([]model.Dimension, error) {
	return s.repo.GetDimensions(ctx, taskID)
}
