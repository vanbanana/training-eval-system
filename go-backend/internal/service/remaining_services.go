package service

import (
	"context"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
)

// --- ProfileService ---

type ProfileService struct {
	repo repository.ProfileRepo
}

func NewProfileService(repo repository.ProfileRepo) *ProfileService {
	return &ProfileService{repo: repo}
}

func (s *ProfileService) GetByStudentID(ctx context.Context, studentID int64) (*model.StudentProfile, error) {
	return s.repo.GetByStudentID(ctx, studentID)
}

func (s *ProfileService) Upsert(ctx context.Context, p *model.StudentProfile) error {
	return s.repo.Upsert(ctx, p)
}

// --- DashboardService ---

type DashboardService struct {
	userRepo repository.UserRepo
	taskRepo repository.TaskRepo
	evalRepo repository.EvaluationRepo
}

func NewDashboardService(userRepo repository.UserRepo, taskRepo repository.TaskRepo, evalRepo repository.EvaluationRepo) *DashboardService {
	return &DashboardService{userRepo: userRepo, taskRepo: taskRepo, evalRepo: evalRepo}
}

// --- ImportService ---

type ImportService struct {
	repo     repository.ImportRepo
	userRepo repository.UserRepo
}

func NewImportService(repo repository.ImportRepo, userRepo repository.UserRepo) *ImportService {
	return &ImportService{repo: repo, userRepo: userRepo}
}

// --- TemplateService ---

type TemplateService struct {
	repo repository.TemplateRepo
}

func NewTemplateService(repo repository.TemplateRepo) *TemplateService {
	return &TemplateService{repo: repo}
}

func (s *TemplateService) GetByID(ctx context.Context, id int64) (*model.EvalTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TemplateService) List(ctx context.Context, ownerID *int64, courseID *int64, visibility *string) ([]model.EvalTemplate, error) {
	return s.repo.List(ctx, ownerID, courseID, visibility)
}

func (s *TemplateService) Create(ctx context.Context, t *model.EvalTemplate) error {
	return s.repo.Create(ctx, t)
}

func (s *TemplateService) Update(ctx context.Context, t *model.EvalTemplate) error {
	return s.repo.Update(ctx, t)
}

func (s *TemplateService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// --- CourseService ---

type CourseService struct {
	repo repository.CourseRepo
}

func NewCourseService(repo repository.CourseRepo) *CourseService {
	return &CourseService{repo: repo}
}

func (s *CourseService) GetByID(ctx context.Context, id int64) (*model.Course, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CourseService) List(ctx context.Context, params repository.ListParams) ([]model.Course, int64, error) {
	return s.repo.List(ctx, params)
}

func (s *CourseService) Create(ctx context.Context, c *model.Course) error {
	return s.repo.Create(ctx, c)
}

func (s *CourseService) Update(ctx context.Context, c *model.Course) error {
	return s.repo.Update(ctx, c)
}

func (s *CourseService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// --- ClassService ---

type ClassService struct {
	repo repository.ClassRepo
}

func NewClassService(repo repository.ClassRepo) *ClassService {
	return &ClassService{repo: repo}
}

func (s *ClassService) GetByID(ctx context.Context, id int64) (*model.Class, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ClassService) List(ctx context.Context, courseID *int64, teacherID *int64) ([]model.Class, error) {
	return s.repo.List(ctx, courseID, teacherID)
}

func (s *ClassService) Create(ctx context.Context, c *model.Class) error {
	return s.repo.Create(ctx, c)
}

func (s *ClassService) AddMember(ctx context.Context, classID, studentID int64) error {
	return s.repo.AddMember(ctx, classID, studentID)
}

func (s *ClassService) RemoveMember(ctx context.Context, classID, studentID int64) error {
	return s.repo.RemoveMember(ctx, classID, studentID)
}

func (s *ClassService) GetMembers(ctx context.Context, classID int64) ([]model.ClassMembership, error) {
	return s.repo.GetMembers(ctx, classID)
}

// --- AuditService ---

type AuditService struct {
	repo repository.AuditRepo
}

func NewAuditService(repo repository.AuditRepo) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) Create(ctx context.Context, log *model.AuditLog) error {
	return s.repo.Create(ctx, log)
}

func (s *AuditService) List(ctx context.Context, params repository.ListParams, userID *int64, action *string) ([]model.AuditLog, int64, error) {
	return s.repo.List(ctx, params, userID, action)
}

// --- AccountService ---

type AccountService struct {
	userRepo repository.UserRepo
}

func NewAccountService(userRepo repository.UserRepo) *AccountService {
	return &AccountService{userRepo: userRepo}
}

// --- SystemConfigService ---

type SystemConfigService struct {
	repo repository.SystemConfigRepo
}

func NewSystemConfigService(repo repository.SystemConfigRepo) *SystemConfigService {
	return &SystemConfigService{repo: repo}
}

func (s *SystemConfigService) GetByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	return s.repo.GetByKey(ctx, key)
}

func (s *SystemConfigService) List(ctx context.Context, category *string) ([]model.SystemConfig, error) {
	return s.repo.List(ctx, category)
}

func (s *SystemConfigService) Upsert(ctx context.Context, cfg *model.SystemConfig) error {
	return s.repo.Upsert(ctx, cfg)
}

// --- LLMConfigService ---

type LLMConfigService struct {
	repo repository.LLMConfigRepo
}

func NewLLMConfigService(repo repository.LLMConfigRepo) *LLMConfigService {
	return &LLMConfigService{repo: repo}
}

func (s *LLMConfigService) GetActive(ctx context.Context) (*model.LLMConfig, error) {
	return s.repo.GetActive(ctx)
}

func (s *LLMConfigService) List(ctx context.Context) ([]model.LLMConfig, error) {
	return s.repo.List(ctx)
}

func (s *LLMConfigService) Create(ctx context.Context, c *model.LLMConfig) error {
	return s.repo.Create(ctx, c)
}

func (s *LLMConfigService) Update(ctx context.Context, c *model.LLMConfig) error {
	return s.repo.Update(ctx, c)
}

func (s *LLMConfigService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
