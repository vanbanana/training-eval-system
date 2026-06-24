package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// Allowed file extensions
var allowedExtensions = map[string]bool{
	".pdf":  true,
	".docx": true,
	".doc":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
}

// UploadService handles file upload operations.
type UploadService struct {
	repo       repository.UploadRepo
	taskRepo   repository.TaskRepo
	uploadRoot string
	maxSizeMB  int
}

// NewUploadService creates a new upload service.
func NewUploadService(repo repository.UploadRepo, taskRepo repository.TaskRepo, uploadRoot string, maxSizeMB int) *UploadService {
	return &UploadService{repo: repo, taskRepo: taskRepo, uploadRoot: uploadRoot, maxSizeMB: maxSizeMB}
}

func (s *UploadService) GetByID(ctx context.Context, id int64) (*model.Upload, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UploadService) List(ctx context.Context, params repository.UploadListParams) ([]model.Upload, int64, error) {
	return s.repo.List(ctx, params)
}

// Upload validates and stores a file, creating an upload record.
func (s *UploadService) Upload(ctx context.Context, taskID, studentID int64, filename string, fileSize int64, reader io.Reader) (*model.Upload, error) {
	// Check task status
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("upload_service: task not found: %w", err)
	}
	if task.Status == "closed" {
		return nil, fmt.Errorf("upload_service: task is closed, uploads not allowed")
	}

	// Validate file size
	maxBytes := int64(s.maxSizeMB) * 1024 * 1024
	if fileSize > maxBytes {
		return nil, fmt.Errorf("upload_service: file too large (max %dMB)", s.maxSizeMB)
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExtensions[ext] {
		return nil, fmt.Errorf("upload_service: file type %q not allowed", ext)
	}

	// Validate filename (path traversal protection)
	if err := validateFilename(filename); err != nil {
		return nil, err
	}

	// Determine storage path: uploadRoot/task_id/student_id/filename
	dir := filepath.Join(s.uploadRoot, fmt.Sprintf("%d", taskID), fmt.Sprintf("%d", studentID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("upload_service: create dir: %w", err)
	}

	storagePath := filepath.Join(dir, filename)
	f, err := os.Create(storagePath)
	if err != nil {
		return nil, fmt.Errorf("upload_service: create file: %w", err)
	}
	defer f.Close()

// Write file and compute SHA-256
		hasher := sha256.New()
		// Use LimitReader to enforce actual byte cap (prevents declared-size bypass, M-2)
		limitedReader := io.LimitReader(reader, maxBytes+1)
		written, err := io.Copy(io.MultiWriter(f, hasher), limitedReader)
		if err != nil {
			os.Remove(storagePath)
			return nil, fmt.Errorf("upload_service: write file: %w", err)
		}
		if written > maxBytes {
			os.Remove(storagePath)
			return nil, fmt.Errorf("upload_service: file exceeds maximum size of %dMB", s.maxSizeMB)
		}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	upload := &model.Upload{
		TaskID:      taskID,
		StudentID:   studentID,
		Filename:    filename,
		FileType:    ext[1:], // remove leading dot
		FileSize:    written,
		StoragePath: storagePath,
		SHA256:      checksum,
		ParseStatus: "pending",
		Version:     1,
	}

	if err := s.repo.Create(ctx, upload); err != nil {
		os.Remove(storagePath)
		return nil, err
	}

	return upload, nil
}

func (s *UploadService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *UploadService) UpdateStatus(ctx context.Context, id int64, status string) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *UploadService) GetVerifyResult(ctx context.Context, uploadID int64) (*model.VerifyResult, error) {
	return s.repo.GetVerifyResult(ctx, uploadID)
}

// GetParseResult returns the parse result for an upload.
func (s *UploadService) GetParseResult(ctx context.Context, uploadID int64) (*model.ParseResult, error) {
	return s.repo.GetParseResult(ctx, uploadID)
}

// validateFilename rejects path traversal attempts.
func validateFilename(name string) error {
	if strings.Contains(name, "..") ||
		strings.Contains(name, "/") ||
		strings.Contains(name, "\\") ||
		strings.ContainsAny(name, "\x00") ||
		filepath.IsAbs(name) {
		return fmt.Errorf("upload_service: invalid filename (path traversal detected)")
	}
	return nil
}
