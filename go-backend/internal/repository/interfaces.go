// Package repository defines data access interfaces and their SQLite implementations.
package repository

import (
	"context"
	"time"

	"github.com/smartedu/training-eval-system/internal/model"
)

// ListParams holds common pagination/filter parameters.
type ListParams struct {
	Page     int
	PageSize int
	Search   string
	SortBy   string
	SortDir  string // "asc" or "desc"
}

// Offset calculates the SQL OFFSET from page and page_size.
func (p ListParams) Offset() int {
	if p.Page < 1 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

// allowedSortColumns is the allow-list of column names for ORDER BY.
// Any SortBy value not in this list is rejected to prevent SQL injection.
var allowedSortColumns = map[string]bool{
	"id":              true,
	"name":            true,
	"username":        true,
	"display_name":    true,
	"role":            true,
	"status":          true,
	"created_at":      true,
	"updated_at":      true,
	"deadline":        true,
	"teacher_id":      true,
	"course_id":       true,
	"total_score":     true,
	"student_count":   true,
	"last_login_at":   true,
}

// isValidSortColumn checks that sortBy is a known, safe column name.
func isValidSortColumn(sortBy string) bool {
	return allowedSortColumns[sortBy]
}

// TaskListParams extends ListParams with task-specific filters.
type TaskListParams struct {
	ListParams
	TeacherID *int64
	CourseID  *int64
	Status    *string
}

// UploadListParams extends ListParams with upload-specific filters.
type UploadListParams struct {
	ListParams
	TaskID      *int64
	StudentID   *int64
	ParseStatus *string
}

// EvalListParams extends ListParams with evaluation-specific filters.
type EvalListParams struct {
	ListParams
	TaskID    *int64
	StudentID *int64
	UploadID  *int64
	Status    *string
}

// UserRepo defines data access for users.
type UserRepo interface {
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, params ListParams) ([]model.User, int64, error)
	Create(ctx context.Context, u *model.User) error
	Update(ctx context.Context, u *model.User) error
	Delete(ctx context.Context, id int64) error
	UpdateLoginState(ctx context.Context, id int64, failed int, lockedUntil *time.Time) error
	UpdateLastLogin(ctx context.Context, id int64) error
	ToggleActive(ctx context.Context, id int64, active bool) error
}

// TaskRepo defines data access for training tasks.
type TaskRepo interface {
	GetByID(ctx context.Context, id int64) (*model.TrainingTask, error)
	List(ctx context.Context, params TaskListParams) ([]model.TrainingTask, int64, error)
	Create(ctx context.Context, t *model.TrainingTask) error
	Update(ctx context.Context, t *model.TrainingTask) error
	Delete(ctx context.Context, id int64) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	SetClasses(ctx context.Context, taskID int64, classIDs []int64) error
	GetDimensions(ctx context.Context, taskID int64) ([]model.Dimension, error)
	SetDimensions(ctx context.Context, taskID int64, dims []model.Dimension) error
	EnsureTaskHasClasses(ctx context.Context, taskID int64) error
}

// UploadRepo defines data access for file uploads.
type UploadRepo interface {
	GetByID(ctx context.Context, id int64) (*model.Upload, error)
	List(ctx context.Context, params UploadListParams) ([]model.Upload, int64, error)
	Create(ctx context.Context, u *model.Upload) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
	GetParseResult(ctx context.Context, uploadID int64) (*model.ParseResult, error)
	SaveParseResult(ctx context.Context, pr *model.ParseResult) error
	SaveVerifyResult(ctx context.Context, vr *model.VerifyResult) error
	GetVerifyResult(ctx context.Context, uploadID int64) (*model.VerifyResult, error)
}

// EvaluationRepo defines data access for evaluations.
type EvaluationRepo interface {
	GetByID(ctx context.Context, id int64) (*model.Evaluation, error)
	List(ctx context.Context, params EvalListParams) ([]model.Evaluation, int64, error)
	Create(ctx context.Context, e *model.Evaluation) error
	Update(ctx context.Context, e *model.Evaluation) error
	Delete(ctx context.Context, id int64) error
	BatchConfirm(ctx context.Context, ids []int64) error
	SaveScores(ctx context.Context, evalID int64, scores []model.DimensionScore) error
	AppendHistory(ctx context.Context, h *model.EvaluationHistory) error
	GetHistory(ctx context.Context, evalID int64) ([]model.EvaluationHistory, error)
	GetDimensionScores(ctx context.Context, evalID int64) ([]model.DimensionScore, error)
	UpdateDimensionTeacherScore(ctx context.Context, evalID, dimID int64, teacherScore *float64) error
}

// CourseRepo defines data access for courses.
type CourseRepo interface {
	GetByID(ctx context.Context, id int64) (*model.Course, error)
	List(ctx context.Context, params ListParams) ([]model.Course, int64, error)
	Create(ctx context.Context, c *model.Course) error
	Update(ctx context.Context, c *model.Course) error
	Delete(ctx context.Context, id int64) error
}

// ClassRepo defines data access for classes.
type ClassRepo interface {
	GetByID(ctx context.Context, id int64) (*model.Class, error)
	List(ctx context.Context, courseID *int64, teacherID *int64) ([]model.Class, error)
	Create(ctx context.Context, c *model.Class) error
	Update(ctx context.Context, c *model.Class) error
	Delete(ctx context.Context, id int64) error
	AddMember(ctx context.Context, classID, studentID int64) error
	RemoveMember(ctx context.Context, classID, studentID int64) error
	GetMembers(ctx context.Context, classID int64) ([]model.ClassMembership, error)
}

// NotificationRepo defines data access for notifications.
type NotificationRepo interface {
	GetByID(ctx context.Context, id int64) (*model.Notification, error)
	List(ctx context.Context, userID int64, unreadOnly bool, params ListParams) ([]model.Notification, int64, error)
	Create(ctx context.Context, n *model.Notification) error
	MarkRead(ctx context.Context, id int64) error
	MarkAllRead(ctx context.Context, userID int64) error
	UnreadCount(ctx context.Context, userID int64) (int64, error)
	// Preferences
	GetPreferencesByUserID(ctx context.Context, userID int64) ([]model.NotificationPref, error)
	UpsertPreference(ctx context.Context, pref *model.NotificationPref) error
}

// ChatRepo defines data access for chat sessions and messages.
type ChatRepo interface {
	GetSession(ctx context.Context, id int64) (*model.ChatSession, error)
	ListSessions(ctx context.Context, studentID int64) ([]model.ChatSession, error)
	CreateSession(ctx context.Context, s *model.ChatSession) error
	DeleteSession(ctx context.Context, id int64) error
	GetMessages(ctx context.Context, sessionID int64, limit int) ([]model.ChatMessage, error)
	CreateMessage(ctx context.Context, m *model.ChatMessage) error
	CountTodayMessages(ctx context.Context, studentID int64) (int, error)
	CountSessionMessages(ctx context.Context, sessionID int64) (int, error)
}

// SimilarityRepo defines data access for similarity records.
type SimilarityRepo interface {
	GetByID(ctx context.Context, id int64) (*model.SimilarityRecord, error)
	List(ctx context.Context, taskID int64, state *string) ([]model.SimilarityRecord, error)
	Create(ctx context.Context, r *model.SimilarityRecord) error
	UpdateState(ctx context.Context, id int64, state string, reviewedBy int64) error
	GetByTaskPair(ctx context.Context, taskID, uploadAID, uploadBID int64) (*model.SimilarityRecord, error)
}

// TemplateRepo defines data access for evaluation templates.
type TemplateRepo interface {
	GetByID(ctx context.Context, id int64) (*model.EvalTemplate, error)
	List(ctx context.Context, ownerID *int64, courseID *int64, visibility *string) ([]model.EvalTemplate, error)
	Create(ctx context.Context, t *model.EvalTemplate) error
	Update(ctx context.Context, t *model.EvalTemplate) error
	Delete(ctx context.Context, id int64) error
	SetItems(ctx context.Context, templateID int64, items []model.TemplateDimension) error
}

// ImportRepo defines data access for import jobs.
type ImportRepo interface {
	GetByID(ctx context.Context, id int64) (*model.ImportJob, error)
	List(ctx context.Context, operatorID *int64, params ListParams) ([]model.ImportJob, int64, error)
	Create(ctx context.Context, j *model.ImportJob) error
	Update(ctx context.Context, j *model.ImportJob) error
	CreateRecord(ctx context.Context, r *model.ImportRecord) error
}

// AuditRepo defines data access for audit logs (append-only).
type AuditRepo interface {
	Create(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, params ListParams, userID *int64, action *string) ([]model.AuditLog, int64, error)
}

// ProfileRepo defines data access for student profiles.
type ProfileRepo interface {
	GetByStudentID(ctx context.Context, studentID int64) (*model.StudentProfile, error)
	Upsert(ctx context.Context, p *model.StudentProfile) error
}

// SystemConfigRepo defines data access for system configuration.
type SystemConfigRepo interface {
	GetByKey(ctx context.Context, key string) (*model.SystemConfig, error)
	List(ctx context.Context, category *string) ([]model.SystemConfig, error)
	Upsert(ctx context.Context, cfg *model.SystemConfig) error
}

// LLMConfigRepo defines data access for LLM configurations.
type LLMConfigRepo interface {
	GetByID(ctx context.Context, id int64) (*model.LLMConfig, error)
	GetActive(ctx context.Context) (*model.LLMConfig, error)
	List(ctx context.Context) ([]model.LLMConfig, error)
	Create(ctx context.Context, c *model.LLMConfig) error
	Update(ctx context.Context, c *model.LLMConfig) error
	Delete(ctx context.Context, id int64) error
}

// AgentRepo defines data access for role-aware agent sessions and messages.
type AgentRepo interface {
	GetSession(ctx context.Context, id int64) (*model.AgentSession, error)
	ListSessions(ctx context.Context, ownerID int64) ([]model.AgentSession, error)
	CreateSession(ctx context.Context, s *model.AgentSession) error
	DeleteSession(ctx context.Context, id int64) error
	GetMessages(ctx context.Context, sessionID int64, limit int) ([]model.AgentMessage, error)
	CreateMessage(ctx context.Context, m *model.AgentMessage) error
	CountTodayMessages(ctx context.Context, ownerID int64) (int, error)
	CountSessionMessages(ctx context.Context, sessionID int64) (int, error)
	UpdateSessionContext(ctx context.Context, sessionID int64, contextJSON string) error
	// Legacy chat_sessions backward compatibility
	ListLegacySessions(ctx context.Context, ownerID int64) ([]model.AgentSession, error)
	GetLegacySession(ctx context.Context, id int64) (*model.AgentSession, error)
	GetLegacyMessages(ctx context.Context, sessionID int64, limit int) ([]model.AgentMessage, error)
}
