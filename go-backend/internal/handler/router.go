// Package handler implements HTTP route handlers using chi v5.
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smartedu/training-eval-system/internal/middleware"
)

// RouterConfig holds all dependencies needed to build the router.
type RouterConfig struct {
	JWTSecret            string
	CORSOrigins          []string
	AuthHandler          *AuthHandler
	UsersHandler         *UsersHandler
	TasksHandler         *TasksHandler
	UploadsHandler       *UploadsHandler
	EvaluationsHandler   *EvaluationsHandler
	GradingHandler       *GradingHandler
	CoursesHandler       *CoursesHandler
	ClassesHandler       *ClassesHandler
	NotificationsHandler *NotificationsHandler
	ChatHandler          *ChatHandler
	SimilarityHandler    *SimilarityHandler
	TemplatesHandler     *TemplatesHandler
	ImportsHandler       *ImportsHandler
	DashboardHandler     *DashboardHandler
	ReportsHandler       *ReportsHandler
	ProfilesHandler      *ProfilesHandler
	LLMHandler           *LLMHandler
	AuditHandler         *AuditHandler
	AccountHandler       *AccountHandler
	SSEHandler           *SSEHandler
	ParseHandler         *ParseHandler
}

// NewRouter creates the chi router with all routes and middleware.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Global middleware chain
	r.Use(middleware.TraceMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.CORS(cfg.CORSOrigins))
	r.Use(middleware.SecurityHeaders)

	// Health check (public)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public auth routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", cfg.AuthHandler.Login)
			r.Post("/refresh", cfg.AuthHandler.Refresh)
			r.Group(func(r chi.Router) {
				r.Use(middleware.AuthMiddleware(cfg.JWTSecret))
				r.Get("/me", cfg.AuthHandler.Me)
				r.Post("/logout", cfg.AuthHandler.Logout)
			})
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(cfg.JWTSecret))

			// Admin-only routes
			r.Route("/users", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/", cfg.UsersHandler.List)
				r.Post("/", cfg.UsersHandler.Create)
				r.Put("/{id}", cfg.UsersHandler.Update)
				r.Patch("/{id}", cfg.UsersHandler.Update)
				r.Delete("/{id}", cfg.UsersHandler.Delete)
				r.Patch("/{id}/toggle-status", cfg.UsersHandler.ToggleStatus)
				r.Patch("/{id}/toggle-active", cfg.UsersHandler.ToggleStatus)
				r.Post("/{id}/reset-password", cfg.UsersHandler.ResetPassword)
			})

			// Allow teachers to look up individual students for profile display
			r.Get("/users/{id}", cfg.UsersHandler.Get)

			r.Route("/llm", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/configs", cfg.LLMHandler.List)
				r.Post("/configs", cfg.LLMHandler.Create)
				r.Post("/test", cfg.LLMHandler.Test)
			})

			r.Route("/audit", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/", cfg.AuditHandler.List)
				r.Get("/export", cfg.AuditHandler.Export)
			})

			r.Route("/imports", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Post("/users", cfg.ImportsHandler.ImportUsers)
				r.Post("/students", cfg.ImportsHandler.ImportStudents)
				r.Get("/template/user.xlsx", cfg.ImportsHandler.DownloadTemplate)
			})

			// Teacher + Admin routes (write operations)
			r.Route("/tasks", func(r chi.Router) {
				// GET is accessible to all authenticated users (students see published tasks)
				r.Get("/", cfg.TasksHandler.List)
				r.Get("/{id}", cfg.TasksHandler.Get)

				// Write operations require teacher or admin
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin", "teacher"))
					r.Post("/", cfg.TasksHandler.Create)
					r.Patch("/{id}", cfg.TasksHandler.Update)
					r.Delete("/{id}", cfg.TasksHandler.Delete)
					r.Patch("/{id}/publish", cfg.TasksHandler.Publish)
					r.Post("/{id}/publish", cfg.TasksHandler.Publish)
					r.Patch("/{id}/close", cfg.TasksHandler.Close)
					r.Put("/{id}/dimensions", cfg.TasksHandler.ReplaceDimensions)
				})
			})

			r.Route("/grading", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin", "teacher"))
				r.Get("/tasks/{id}/submissions", cfg.GradingHandler.GetSubmissions)
				r.Get("/tasks/{id}/summary", cfg.GradingHandler.GetSummary)
				r.Post("/evaluations/{id}/confirm", cfg.GradingHandler.Confirm)
				r.Post("/evaluations/{id}/reject", cfg.GradingHandler.Reject)
			})

			r.Route("/similarity", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin", "teacher"))
				r.Get("/task/{taskId}", cfg.SimilarityHandler.GetByTask)
				r.Get("/{id}/segments", cfg.SimilarityHandler.GetSegments)
				r.Post("/{id}/decision", cfg.SimilarityHandler.UpdateDecision)
			})

			r.Route("/templates", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin", "teacher"))
				r.Get("/", cfg.TemplatesHandler.List)
				r.Post("/", cfg.TemplatesHandler.Create)
				r.Delete("/{id}", cfg.TemplatesHandler.Delete)
				r.Post("/from-task", cfg.TemplatesHandler.CreateFromTask)
			})

			r.Route("/reports", func(r chi.Router) {
				// Personal report is accessible to all authenticated users (students download their own)
				r.Get("/personal/{evalId}", cfg.ReportsHandler.GetPersonal)

				// Statistics and CSV export require teacher or admin
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin", "teacher"))
					r.Get("/task/{taskId}/csv", cfg.ReportsHandler.ExportCSV)
					r.Get("/statistics/{taskId}", cfg.ReportsHandler.GetStatistics)
					r.Get("/statistics/{taskId}/xlsx", cfg.ReportsHandler.ExportStatisticsXLSX)
				})
			})

			// All authenticated users
			r.Route("/uploads", func(r chi.Router) {
				r.Get("/{taskId}", cfg.UploadsHandler.ListByTask)
				r.Post("/{taskId}", cfg.UploadsHandler.Upload)
				r.Get("/{id}/verify-result", cfg.UploadsHandler.VerifyResult)
				r.Post("/{id}/retry", cfg.UploadsHandler.Retry)
			})

			r.Route("/evaluations", func(r chi.Router) {
				r.Get("/my", cfg.EvaluationsHandler.GetMy)
				r.Get("/{id}", cfg.EvaluationsHandler.GetByID)
				r.Get("/{id}/history", cfg.EvaluationsHandler.GetHistory)
				r.Post("/trigger/{uploadId}", cfg.EvaluationsHandler.Trigger)
				r.Post("/bulk-action", cfg.EvaluationsHandler.BulkAction)
				r.Patch("/{id}/dimensions/{dimId}", cfg.EvaluationsHandler.UpdateDimensionScore)
			})

			r.Route("/courses", func(r chi.Router) {
				r.Get("/", cfg.CoursesHandler.List)
				r.Post("/", cfg.CoursesHandler.Create)
				r.Patch("/{id}/archive", cfg.CoursesHandler.ToggleArchive)
				r.Get("/{id}/classes", cfg.CoursesHandler.GetClasses)
			})

			r.Route("/classes", func(r chi.Router) {
				r.Get("/", cfg.ClassesHandler.List)
				r.Post("/", cfg.ClassesHandler.Create)
				r.Get("/{id}/students", cfg.ClassesHandler.GetStudents)
				r.Patch("/{id}/archive", cfg.ClassesHandler.ToggleArchive)
				r.Post("/{id}/students/bulk", cfg.ClassesHandler.BulkAddStudents)
				r.Delete("/{id}/students/{studentId}", cfg.ClassesHandler.RemoveStudent)
			})

			r.Route("/notifications", func(r chi.Router) {
				r.Get("/", cfg.NotificationsHandler.List)
				r.Post("/{id}/read", cfg.NotificationsHandler.MarkRead)
				r.Post("/read-all", cfg.NotificationsHandler.MarkAllRead)
				r.Get("/preferences", cfg.NotificationsHandler.GetPreferences)
				r.Put("/preferences", cfg.NotificationsHandler.UpdatePreferences)
			})

			r.Route("/chat", func(r chi.Router) {
				r.Get("/sessions", cfg.ChatHandler.ListSessions)
				r.Post("/sessions", cfg.ChatHandler.CreateSession)
				r.Get("/sessions/{id}/messages", cfg.ChatHandler.GetMessages)
				r.Delete("/sessions/{id}", cfg.ChatHandler.DeleteSession)
				r.Post("/stream", cfg.ChatHandler.Stream)
			})

			r.Get("/dashboard", cfg.DashboardHandler.Get)

			r.Route("/profiles", func(r chi.Router) {
				r.Get("/student/{userId}", cfg.ProfilesHandler.GetStudent)
				r.Get("/school", cfg.ProfilesHandler.GetSchool)
				r.Get("/course/{courseId}", cfg.ProfilesHandler.GetCourse)
			})

			r.Route("/account", func(r chi.Router) {
				r.Get("/me", cfg.AccountHandler.GetMe)
				r.Patch("/profile", cfg.AccountHandler.UpdateProfile)
				r.Post("/change-password", cfg.AccountHandler.ChangePassword)
			})

			r.Get("/parse/{uploadId}/result", cfg.ParseHandler.GetResult)

			// SSE endpoint for real-time events (replaces WebSocket)
			r.Get("/sse/events", cfg.SSEHandler.Events)
		})
	})

	return r
}
