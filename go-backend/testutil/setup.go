package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smartedu/training-eval-system/internal/handler"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/pipeline"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
	"github.com/smartedu/training-eval-system/internal/sse"
	"github.com/smartedu/training-eval-system/internal/store"
)

// TestApp holds all test dependencies.
type TestApp struct {
	Server *httptest.Server
	DB     *store.DB
	Router http.Handler
}

// SetupTestApp creates a fully wired test application with SQLite :memory:.
func SetupTestApp(t *testing.T) *TestApp {
	t.Helper()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	// Ensure chat migration infrastructure exists (column + mapping table).
	// No data is migrated here — tests seed data and call migration explicitly.
	if err := db.MigrateChatSessions(context.Background()); err != nil {
		t.Fatalf("failed to initialize chat migration: %v", err)
	}

	// Repositories
	userRepo := repository.NewUserRepo(db)
	auditRepo := repository.NewAuditRepo(db)
	taskRepo := repository.NewTaskRepo(db)
	uploadRepo := repository.NewUploadRepo(db)
	evalRepo := repository.NewEvaluationRepo(db)
	courseRepo := repository.NewCourseRepo(db)
	classRepo := repository.NewClassRepo(db)
	notifRepo := repository.NewNotificationRepo(db)
	chatRepo := repository.NewChatRepo(db)
	templateRepo := repository.NewTemplateRepo(db)
	profileRepo := repository.NewProfileRepo(db)
	llmConfigRepo := repository.NewLLMConfigRepo(db)
	usageRepo := repository.NewUsageRepo(db)

	// Infrastructure
	broker := sse.NewBroker()
	lockout := middleware.NewAccountLockout(5, 15*60*1e9) // 15 min

	// Services
	authSvc := service.NewAuthService(userRepo, auditRepo, lockout, TestJWTSecret, 60*60*1e9, 7*24*60*60*1e9)
	userSvc := service.NewUserService(userRepo)
	notifSvc := service.NewNotificationService(notifRepo, broker)
	taskSvc := service.NewTaskService(taskRepo, classRepo, notifSvc)
	uploadSvc := service.NewUploadService(uploadRepo, taskRepo, t.TempDir(), 50)
	evalSvc := service.NewEvaluationService(evalRepo, taskRepo)
	chatSvc := service.NewChatService(chatRepo)
	courseSvc := service.NewCourseService(courseRepo)
	classSvc := service.NewClassService(classRepo)
	templateSvc := service.NewTemplateService(templateRepo)
	profileSvc := service.NewProfileService(profileRepo)
	llmConfigSvc := service.NewLLMConfigService(llmConfigRepo)
	auditSvc := service.NewAuditService(auditRepo)
	usageSvc := service.NewUsageService(usageRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc)
	usersHandler := handler.NewUsersHandler(userSvc)
	tasksHandler := handler.NewTasksHandler(taskSvc)
	uploadsHandler := handler.NewUploadsHandler(uploadSvc, nil)
	evaluationsHandler := handler.NewEvaluationsHandler(evalSvc, taskSvc, uploadSvc)
	gradingHandler := handler.NewGradingHandler(evalSvc, uploadSvc, userSvc, db, nil, nil)
	coursesHandler := handler.NewCoursesHandler(courseSvc, classSvc)
	classesHandler := handler.NewClassesHandler(classSvc, userSvc)
	notificationsHandler := handler.NewNotificationsHandler(notifSvc)
	chatHandler := handler.NewChatHandler(chatSvc, broker, nil, nil, uploadRepo, taskRepo, evalRepo)
	templatesHandler := handler.NewTemplatesHandler(templateSvc, taskSvc)
	dashboardHandler := handler.NewDashboardHandler(db)
	reportsHandler := handler.NewReportsHandler(evalSvc, taskSvc, userSvc, db)
	profilesHandler := handler.NewProfilesHandler(profileSvc, db, nil)
	llmHandler := handler.NewLLMHandler(llmConfigSvc, testMasterKey())
	auditHandler := handler.NewAuditHandler(auditSvc)
	usageHandler := handler.NewUsageHandler(usageSvc)
	accountHandler := handler.NewAccountHandler(userSvc)
	parseHandler := handler.NewParseHandler(uploadSvc)
	similarityHandler := handler.NewSimilarityHandler(repository.NewSimilarityRepo(db), uploadRepo)
	importsHandler := handler.NewImportsHandler(service.NewImportService(repository.NewImportRepo(db), userRepo), userSvc, taskSvc)
	sseHandler := handler.NewSSEHandler(broker, TestJWTSecret)

	// Agent
	agentRepo := repository.NewAgentRepo(db)
	agentSvc := service.NewAgentServiceWithQuotas(agentRepo, nil) // nil → uses safe defaults
	chatOrch := pipeline.NewChatOrchestrator(nil, evalRepo, uploadRepo, taskRepo, profileRepo)
	roleOrch := service.NewRoleAgentOrchestrator(nil)
	agentHandler := handler.NewAgentHandler(agentSvc, nil, evalRepo, uploadRepo, taskRepo, classRepo, courseRepo, chatOrch, roleOrch, auditRepo, usageSvc)
	streamTracker := handler.NewStreamTracker(2, 50)
	agentHandler.SetStreamTracker(streamTracker)

	// Router
	router := handler.NewRouter(handler.RouterConfig{
		JWTSecret:   TestJWTSecret,
		CORSOrigins: []string{"http://localhost:5173"},
		FeatureFlags: middleware.FeatureFlags{
			AgentV2Enabled:         true,
			StudentAgentV2Enabled:  true,
			TeacherAgentEnabled:    true,
			AdminAgentEnabled:      true,
			AgentToolEventsEnabled: true,
		},
		AuthHandler:          authHandler,
		UsersHandler:         usersHandler,
		TasksHandler:         tasksHandler,
		UploadsHandler:       uploadsHandler,
		EvaluationsHandler:   evaluationsHandler,
		GradingHandler:       gradingHandler,
		CoursesHandler:       coursesHandler,
		ClassesHandler:       classesHandler,
		NotificationsHandler: notificationsHandler,
		ChatHandler:          chatHandler,
		SimilarityHandler:    similarityHandler,
		TemplatesHandler:     templatesHandler,
		ImportsHandler:       importsHandler,
		DashboardHandler:     dashboardHandler,
		ReportsHandler:       reportsHandler,
		ProfilesHandler:      profilesHandler,
		LLMHandler:           llmHandler,
		AuditHandler:         auditHandler,
		UsageHandler:         usageHandler,
		AccountHandler:       accountHandler,
		ParseHandler:         parseHandler,
		SSEHandler:           sseHandler,
		AgentHandler:         agentHandler,
		CapabilitiesHandler: handler.NewCapabilitiesHandler(middleware.FeatureFlags{
			AgentV2Enabled:         true,
			StudentAgentV2Enabled:  true,
			TeacherAgentEnabled:    true,
			AdminAgentEnabled:      true,
			AgentToolEventsEnabled: true,
		}),
	})

	srv := httptest.NewServer(router)

	t.Cleanup(func() {
		srv.Close()
		broker.Shutdown()
		db.Close()
	})

	return &TestApp{
		Server: srv,
		DB:     db,
		Router: router,
	}
}
