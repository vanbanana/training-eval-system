// Package service — ImportService provides business logic for bulk user/student imports.
package service

import (
	"context"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// ImportService orchestrates bulk import jobs.
type ImportService struct {
	importRepo repository.ImportRepo
	userRepo   repository.UserRepo
}

// NewImportService creates a new ImportService.
func NewImportService(importRepo repository.ImportRepo, userRepo repository.UserRepo) *ImportService {
	return &ImportService{importRepo: importRepo, userRepo: userRepo}
}

// GetJob retrieves an import job by ID.
func (s *ImportService) GetJob(ctx context.Context, id int64) (*model.ImportJob, error) {
	return s.importRepo.GetByID(ctx, id)
}

// ListJobs returns paginated import jobs, optionally filtered by operator.
func (s *ImportService) ListJobs(ctx context.Context, operatorID *int64, params repository.ListParams) ([]model.ImportJob, int64, error) {
	return s.importRepo.List(ctx, operatorID, params)
}

// CreateJob creates a new import job record.
func (s *ImportService) CreateJob(ctx context.Context, j *model.ImportJob) error {
	if j.Status == "" {
		j.Status = "pending"
	}
	return s.importRepo.Create(ctx, j)
}

// UpdateJob updates an existing import job.
func (s *ImportService) UpdateJob(ctx context.Context, j *model.ImportJob) error {
	return s.importRepo.Update(ctx, j)
}

// CreateRecord saves a single import record row.
func (s *ImportService) CreateRecord(ctx context.Context, rec *model.ImportRecord) error {
	return s.importRepo.CreateRecord(ctx, rec)
}

// suppress unused import warning
var _ = fmt.Sprintf
