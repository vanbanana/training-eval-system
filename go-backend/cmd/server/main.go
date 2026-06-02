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
	"github.com/smartedu/training-eval-system/internal/handler"
	"github.com/smartedu/training-eval-system/internal/middleware"
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
	taskSvc := service.NewTaskService(taskRepo, notifSvc)
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

	// 7b. Initialize pipeline orchestrator
	orch := pipeline.NewOrchestrator(pipeline.OrchestratorDeps{
		Pool:        pool,
		Broker:      broker,
		UploadRepo:  uploadRepo,
		EvalRepo:    evalRepo,
		SimRepo:     repository.NewSimilarityRepo(db),
		TaskRepo:    taskRepo,
		ProfileRepo: profileRepo,
		LLMClient:   nil, // Will be initialized when LLM config is active
	})
	_ = orch // used by handlers below

	// 8. Initialize handlers
	authHandler := handler.NewAuthHandler(authSvc)
	usersHandler := handler.NewUsersHandler(userSvc)
	tasksHandler := handler.NewTasksHandler(taskSvc)
	uploadsHandler := handler.NewUploadsHandler(uploadSvc)
	evaluationsHandler := handler.NewEvaluationsHandler(evalSvc, taskSvc)
	gradingHandler := handler.NewGradingHandler(evalSvc, uploadSvc, userSvc, db)
	coursesHandler := handler.NewCoursesHandler(courseSvc, classSvc)
	classesHandler := handler.NewClassesHandler(classSvc, userSvc)
	notificationsHandler := handler.NewNotificationsHandler(notifSvc)
	chatHandler := handler.NewChatHandler(chatSvc, broker, nil, nil, uploadRepo, taskRepo, evalRepo) // LLM client + orchestrator set below if configured
	similarityHandler := handler.NewSimilarityHandler()
	templatesHandler := handler.NewTemplatesHandler(templateSvc, taskSvc)
	importsHandler := handler.NewImportsHandler()
	dashboardHandler := handler.NewDashboardHandler(db)
	reportsHandler := handler.NewReportsHandler(evalSvc, taskSvc, userSvc, db)
	profilesHandler := handler.NewProfilesHandler(profileSvc, db, nil)
	llmHandler := handler.NewLLMHandler(llmConfigSvc)
	auditHandler := handler.NewAuditHandler(auditSvc)
	accountHandler := handler.NewAccountHandler(userSvc)
	parseHandler := handler.NewParseHandler(uploadSvc)

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
	})

	// 10. Start HTTP server
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
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
