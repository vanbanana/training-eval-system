package service

import (
	"context"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/repository"
)

// GradingAccess provides unified access control checks for grading endpoints.
type GradingAccess struct {
	taskRepo   repository.TaskRepo
	evalRepo   repository.EvaluationRepo
	uploadRepo repository.UploadRepo
	userRepo   repository.UserRepo
}

// NewGradingAccess creates a new access control helper.
func NewGradingAccess(
	taskRepo repository.TaskRepo,
	evalRepo repository.EvaluationRepo,
	uploadRepo repository.UploadRepo,
	userRepo repository.UserRepo,
) *GradingAccess {
	return &GradingAccess{
		taskRepo:   taskRepo,
		evalRepo:   evalRepo,
		uploadRepo: uploadRepo,
		userRepo:   userRepo,
	}
}

// CanAccessTask checks if the user can access the given task.
func (ga *GradingAccess) CanAccessTask(ctx context.Context, userID int64, role string, taskID int64) error {
	if role == "admin" {
		return nil
	}
	task, err := ga.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("access: task not found: %w", err)
	}
	if role == "teacher" && task.TeacherID != userID {
		return fmt.Errorf("access: teacher %d cannot access task %d", userID, taskID)
	}
	if role == "student" {
		return nil // students can read published tasks
	}
	return nil
}

// CanAccessEvaluation checks if the user can access the given evaluation.
func (ga *GradingAccess) CanAccessEvaluation(ctx context.Context, userID int64, role string, evalID int64) error {
	eval, err := ga.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("access: evaluation not found: %w", err)
	}
	if role == "admin" {
		return nil
	}
	if role == "teacher" {
		task, err := ga.taskRepo.GetByID(ctx, eval.TaskID)
		if err != nil {
			return fmt.Errorf("access: task not found: %w", err)
		}
		if task.TeacherID != userID {
			return fmt.Errorf("access: teacher %d cannot access evaluation %d", userID, evalID)
		}
		return nil
	}
	if role == "student" && eval.StudentID != userID {
		return fmt.Errorf("access: student %d cannot access evaluation %d", userID, evalID)
	}
	return nil
}

// CanAccessUpload checks if the user can access the given upload.
func (ga *GradingAccess) CanAccessUpload(ctx context.Context, userID int64, role string, uploadID int64) error {
	if role == "admin" {
		return nil
	}
	upload, err := ga.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("access: upload not found: %w", err)
	}
	if role == "teacher" {
		task, err := ga.taskRepo.GetByID(ctx, upload.TaskID)
		if err != nil {
			return fmt.Errorf("access: task not found: %w", err)
		}
		if task.TeacherID != userID {
			return fmt.Errorf("access: teacher %d cannot access upload %d", userID, uploadID)
		}
		return nil
	}
	if role == "student" && upload.StudentID != userID {
		return fmt.Errorf("access: student %d cannot access upload %d", userID, uploadID)
	}
	return nil
}
