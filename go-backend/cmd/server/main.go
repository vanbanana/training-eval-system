package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smartedu/training-eval-system/internal/config"
	"github.com/smartedu/training-eval-system/internal/crypto"
	"github.com/smartedu/training-eval-system/internal/handler"
	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/pipeline"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
	"github.com/smartedu/training-eval-system/internal/sse"
	"github.com/smartedu/training-eval-system/internal/store"
	"github.com/smartedu/training-eval-system/internal/worker"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. Setup structured logger
	setupLogger(cfg.LogLevel)
	slog.Info("starting training-eval-system", "env", cfg.Env, "listen", cfg.ListenAddr, "db", cfg.DBPath)

	// 2b. Derive the AES master key used to encrypt stored LLM API keys.
	masterKey, err := crypto.DeriveMasterKey(cfg.LLMKeyMaster)
	if err != nil {
		slog.Error("failed to derive master key", "error", err)
		os.Exit(1)
	}

	// 3. Open database
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 4. Run migrations
	if err := db.Migrate(context.Background()); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// 5. Initialize infrastructure
	pool := worker.NewPool(cfg.WorkerCount, cfg.TaskBufferSize)
	broker := sse.NewBroker()
	lockout := middleware.NewAccountLockout(5, 15*time.Minute)

	// 6. Initialize repositories
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

	// 7. Initialize services
	authSvc := service.NewAuthService(userRepo, auditRepo, lockout, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	userSvc := service.NewUserService(userRepo)
	notifSvc := service.NewNotificationService(notifRepo, broker)
	taskSvc := service.NewTaskService(taskRepo, classRepo, notifSvc)
	uploadSvc := service.NewUploadService(uploadRepo, taskRepo, cfg.UploadRoot, cfg.MaxUploadSizeMB)
	evalSvc := service.NewEvaluationService(evalRepo, taskRepo)
	chatSvc := service.NewChatService(chatRepo)
	courseSvc := service.NewCourseService(courseRepo)
	classSvc := service.NewClassService(classRepo)
	templateSvc := service.NewTemplateService(templateRepo)
	profileSvc := service.NewProfileService(profileRepo)
	llmConfigSvc := service.NewLLMConfigService(llmConfigRepo)
	auditSvc := service.NewAuditService(auditRepo)

	slog.Info("services initialized", "worker_pool_size", cfg.WorkerCount)

	// 6b. Seed default admin user if no users exist
	if seedDefaultAdmin(userSvc) {
		slog.Info("seeded default admin user")
	}

	// 7b. Initialize LLM client if configured
	var llmClient *llm.Client
	if cfg.LLMAPIKey != "" {
		llmClient = llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, cfg.LLMEmbedModel)
		llmClient.SetUseAPIKeyHeader(cfg.LLMUseAPIKeyHeader)
		// Set OCR model: use mimo-v2.5 (multimodal) for OCR if using mimo-v2.5-pro (text-only)
		if cfg.LLMOCRModel != "" {
			llmClient.SetOCRModel(cfg.LLMOCRModel)
			slog.Info("OCR model configured", "ocr_model", cfg.LLMOCRModel)
		} else if cfg.LLMModel == "mimo-v2.5-pro" {
			llmClient.SetOCRModel("mimo-v2.5")
			slog.Info("Auto-detected OCR model", "ocr_model", "mimo-v2.5")
		}
		slog.Info("LLM client configured", "model", cfg.LLMModel, "base_url", cfg.LLMBaseURL)
	} else {
		slog.Warn("LLM API key not configured, scoring pipeline will not work")
	}

	// 7c. Initialize pipeline orchestrator
	profileComputer := service.NewProfileComputer(evalRepo, profileRepo, taskRepo, pool)
	if llmClient != nil {
		profileComputer.SetLLMClient(llmClient)
	}

	orch := pipeline.NewOrchestrator(pipeline.OrchestratorDeps{
		Pool:          pool,
		Broker:        broker,
		UploadRepo:    uploadRepo,
		EvalRepo:      evalRepo,
		SimRepo:       repository.NewSimilarityRepo(db),
		TaskRepo:      taskRepo,
		ProfileRepo:   profileRepo,
		SystemCfgRepo: repository.NewSystemConfigRepo(db),
		LLMClient:     llmClient,
		OnScored:      profileComputer.TriggerRecompute,
	})
	_ = orch // used by handlers below

	// Recover stuck pipeline tasks from previous run
	go func() {
		time.Sleep(2 * time.Second) // Wait for server to be ready
		orch.RecoverStuck(context.Background())
	}()

	// 8. Initialize handlers
	authHandler := handler.NewAuthHandler(authSvc)
	usersHandler := handler.NewUsersHandler(userSvc)
	tasksHandler := handler.NewTasksHandler(taskSvc)
	uploadsHandler := handler.NewUploadsHandler(uploadSvc, orch)
	evaluationsHandler := handler.NewEvaluationsHandler(evalSvc, taskSvc, uploadSvc)
	gradingHandler := handler.NewGradingHandler(evalSvc, uploadSvc, userSvc, db)
	coursesHandler := handler.NewCoursesHandler(courseSvc, classSvc)
	classesHandler := handler.NewClassesHandler(classSvc, userSvc)
	notificationsHandler := handler.NewNotificationsHandler(notifSvc)
	// Wire the AI chat orchestrator + LLM client so context-aware chat works
	// when an LLM is configured; otherwise chat falls back to the "not configured" message.
	var chatOrch *pipeline.ChatOrchestrator
	if llmClient != nil {
		chatOrch = pipeline.NewChatOrchestrator(llmClient, evalRepo, uploadRepo, taskRepo, profileRepo)
		chatOrch.SetClassRepo(classRepo)
		chatOrch.SetCourseRepo(courseRepo)
	}
	chatHandler := handler.NewChatHandler(chatSvc, broker, llmClient, chatOrch, uploadRepo, taskRepo, evalRepo)
	similarityHandler := handler.NewSimilarityHandler(repository.NewSimilarityRepo(db), uploadRepo)
	templatesHandler := handler.NewTemplatesHandler(templateSvc, taskSvc)
	importsHandler := handler.NewImportsHandler(service.NewImportService(repository.NewImportRepo(db), userRepo), userSvc, taskSvc)
	dashboardHandler := handler.NewDashboardHandler(db)
	reportsHandler := handler.NewReportsHandler(evalSvc, taskSvc, userSvc, db)
	profilesHandler := handler.NewProfilesHandler(profileSvc, db, llmClient)
	llmHandler := handler.NewLLMHandler(llmConfigSvc, masterKey)
	auditHandler := handler.NewAuditHandler(auditSvc)
	accountHandler := handler.NewAccountHandler(userSvc)
	parseHandler := handler.NewParseHandler(uploadSvc)
	sseHandler := handler.NewSSEHandler(broker, cfg.JWTSecret)
	healthHandler := handler.NewHealthHandler(db)
	staticHandler := handler.NewStaticHandler(cfg.DistDir)

	// 9. Build router
	router := handler.NewRouter(handler.RouterConfig{
		JWTSecret:            cfg.JWTSecret,
		CORSOrigins:          cfg.CORSOrigins,
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
		AccountHandler:       accountHandler,
		ParseHandler:         parseHandler,
		SSEHandler:           sseHandler,
		HealthHandler:        healthHandler,
		StaticHandler:        staticHandler,
	})

	// 10. Start HTTP server
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("HTTP server listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// 11. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP shutdown error", "error", err)
	}
	pool.Shutdown()
	broker.Shutdown()
	slog.Info("shutdown complete")
}

func setupLogger(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)
}

// seedDefaultAdmin creates the default admin user if no users exist.
func seedDefaultAdmin(userSvc *service.UserService) bool {
	ctx := context.Background()
	users, _, err := userSvc.List(ctx, repository.ListParams{Page: 1, PageSize: 1})
	if err != nil || len(users) > 0 {
		return false
	}
	err = userSvc.Create(ctx, &model.User{
		Username:    "admin",
		DisplayName: "系统管理员",
		Role:        "admin",
		IsActive:    true,
	}, "admin123")
	return err == nil
}
