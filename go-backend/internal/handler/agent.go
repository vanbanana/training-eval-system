// Package handler — Agent API handler for the unified AI agent system.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/pipeline"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
)

// AgentHandler handles /api/agent/* endpoints.
type AgentHandler struct {
	agentSvc  *service.AgentService
	llmClient *llm.Client
	evalRepo  repository.EvaluationRepo
	uploadRepo repository.UploadRepo
	taskRepo  repository.TaskRepo
	classRepo repository.ClassRepo
	courseRepo repository.CourseRepo
	chatOrch  *pipeline.ChatOrchestrator
	roleOrch  *service.RoleAgentOrchestrator
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(
	agentSvc *service.AgentService,
	llmClient *llm.Client,
	evalRepo repository.EvaluationRepo,
	uploadRepo repository.UploadRepo,
	taskRepo repository.TaskRepo,
	classRepo repository.ClassRepo,
	courseRepo repository.CourseRepo,
	chatOrch *pipeline.ChatOrchestrator,
	roleOrch *service.RoleAgentOrchestrator,
) *AgentHandler {
	return &AgentHandler{
		agentSvc:   agentSvc,
		llmClient:  llmClient,
		evalRepo:   evalRepo,
		uploadRepo: uploadRepo,
		taskRepo:   taskRepo,
		classRepo:  classRepo,
		courseRepo: courseRepo,
		chatOrch:   chatOrch,
		roleOrch:   roleOrch,
	}
}

// agentError writes a unified agent error response.
func agentError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dto.AgentErrorResponse{Code: code, Message: message})
}

// ListSessions returns all sessions for the current user (including legacy).
func (h *AgentHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	sessions, err := h.agentSvc.ListSessions(r.Context(), claims.Sub)
	if err != nil {
		agentError(w, http.StatusInternalServerError, dto.AgentErrInternal, "Failed to list sessions")
		return
	}
	items := make([]dto.AgentSessionResponse, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, toAgentSessionResponse(s))
	}
	JSON(w, http.StatusOK, items)
}

// CreateSession creates a new agent session.
func (h *AgentHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req dto.CreateAgentSessionRequest
	if err := Decode(r, &req); err != nil {
		agentError(w, http.StatusBadRequest, dto.AgentErrInvalidRequest, "Invalid request body")
		return
	}

	// Role check: user role must match agent_role
	if req.AgentRole != claims.Role {
		agentError(w, http.StatusForbidden, dto.AgentErrRoleMismatch,
			fmt.Sprintf("User role %q cannot create %q agent sessions", claims.Role, req.AgentRole))
		return
	}

	sess := &model.AgentSession{
		OwnerID:   claims.Sub,
		OwnerRole: claims.Role,
		AgentRole: req.AgentRole,
		Title:     req.Title,
	}
	if req.Context != nil {
		ctxJSON, _ := json.Marshal(req.Context)
		sess.ContextJSON = string(ctxJSON)
	}

	if err := h.agentSvc.CreateSession(r.Context(), sess); err != nil {
		agentError(w, http.StatusInternalServerError, dto.AgentErrInternal, "Failed to create session")
		return
	}

	JSON(w, http.StatusCreated, toAgentSessionResponse(*sess))
}

// GetMessages returns messages for a session.
func (h *AgentHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	sessionID, err := PathInt64(r, "id")
	if err != nil {
		agentError(w, http.StatusBadRequest, dto.AgentErrInvalidRequest, "Invalid session ID")
		return
	}

	// Get session and verify ownership (anti-enumeration: 404 for non-owner)
	sess, err := h.agentSvc.GetSession(r.Context(), sessionID)
	if err != nil {
		agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
		return
	}
	if sess.OwnerID != claims.Sub {
		agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
		return
	}

	messages, err := h.agentSvc.GetMessages(r.Context(), sessionID, 50)
	if err != nil {
		agentError(w, http.StatusInternalServerError, dto.AgentErrInternal, "Failed to get messages")
		return
	}

	items := make([]dto.AgentMessageResponse, 0, len(messages))
	for _, m := range messages {
		items = append(items, toAgentMessageResponse(m))
	}
	JSON(w, http.StatusOK, items)
}

// DeleteSession deletes a session.
func (h *AgentHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	sessionID, err := PathInt64(r, "id")
	if err != nil {
		agentError(w, http.StatusBadRequest, dto.AgentErrInvalidRequest, "Invalid session ID")
		return
	}

	sess, err := h.agentSvc.GetSession(r.Context(), sessionID)
	if err != nil {
		agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
		return
	}
	if sess.OwnerID != claims.Sub {
		agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
		return
	}

	if err := h.agentSvc.DeleteSession(r.Context(), sessionID); err != nil {
		agentError(w, http.StatusInternalServerError, dto.AgentErrInternal, "Failed to delete session")
		return
	}
	JSON(w, http.StatusOK, dto.AgentSessionResponse{ID: sessionID})
}

// Stream handles SSE streaming for agent messages.
func (h *AgentHandler) Stream(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	var req dto.AgentStreamRequest
	if err := Decode(r, &req); err != nil {
		agentError(w, http.StatusBadRequest, dto.AgentErrInvalidRequest, "Invalid request body")
		return
	}

	// Empty message check
	if strings.TrimSpace(req.Message) == "" {
		agentError(w, http.StatusBadRequest, dto.AgentErrInvalidRequest, "Message cannot be empty")
		return
	}

	// Message length check
	if len(req.Message) > 500 {
		agentError(w, http.StatusBadRequest, dto.AgentErrMessageTooLong, "Message exceeds maximum length of 500 characters")
		return
	}

	// Sensitive content check
	if service.CheckSensitiveContent(req.Message) {
		agentError(w, http.StatusBadRequest, dto.AgentErrSensitiveWord,
			"Your message contains content that cannot be processed. Please rephrase.")
		return
	}

	// Get session and verify ownership
	sess, err := h.agentSvc.GetSession(r.Context(), req.SessionID)
	if err != nil {
		agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
		return
	}
	if sess.OwnerID != claims.Sub {
		agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
		return
	}

	// Role check for stream
	if req.AgentRole != claims.Role {
		agentError(w, http.StatusForbidden, dto.AgentErrRoleMismatch, "Agent role does not match user role")
		return
	}

	// Context validation
	var evalCtx *model.Evaluation
	var taskCtx *model.TrainingTask
	var uploadCtx *model.Upload
	var parseResultCtx *model.ParseResult
	var classCtx *model.Class

	if req.Context != nil {
		// Cross-role context check: students cannot use teacher-only context fields
		if claims.Role == "student" {
			if req.Context.ClassID != nil || req.Context.CourseID != nil {
				agentError(w, http.StatusForbidden, dto.AgentErrCrossRoleContext,
					"Students cannot use class or course context")
				return
			}
		}

		// Context switch validation
		if req.Context != nil {
			newCtxJSON, _ := json.Marshal(req.Context)
			if err := service.ValidateContextSwitch(sess.ContextJSON, string(newCtxJSON), req.ForceContextSwitch); err != nil {
				agentError(w, http.StatusBadRequest, dto.AgentErrContextSwitch,
					"Context switch requires force_context_switch=true")
				return
			}
		}

		// Student evaluation context validation
		if claims.Role == "student" && req.Context.EvaluationID != nil {
			eval, err := h.evalRepo.GetByID(r.Context(), *req.Context.EvaluationID)
			if err != nil {
				agentError(w, http.StatusNotFound, dto.AgentErrContextNotFound, "Evaluation not found")
				return
			}
			if eval.StudentID != claims.Sub {
				agentError(w, http.StatusNotFound, dto.AgentErrContextNotFound, "Evaluation not found")
				return
			}
			// Check upload status
			upload, err := h.uploadRepo.GetByID(r.Context(), eval.UploadID)
			if err != nil || upload == nil {
				agentError(w, http.StatusNotFound, dto.AgentErrContextNotFound, "Upload not found")
				return
			}
			if upload.IsDeleted {
				agentError(w, http.StatusNotFound, dto.AgentErrContextNotFound, "Upload has been deleted")
				return
			}
			if upload.ParseStatus == "failed" {
				agentError(w, http.StatusBadRequest, dto.AgentErrContextNotFound,
					"Upload parsing failed, evaluation context unavailable")
				return
			}
			evalCtx = eval
			uploadCtx = upload
			taskCtx, _ = h.taskRepo.GetByID(r.Context(), eval.TaskID)
			if uploadCtx != nil {
				parseResultCtx, _ = h.uploadRepo.GetParseResult(r.Context(), uploadCtx.ID)
			}
		}

		// Teacher context validation
		if claims.Role == "teacher" {
			if req.Context.TaskID != nil {
				task, err := h.taskRepo.GetByID(r.Context(), *req.Context.TaskID)
				if err != nil {
					agentError(w, http.StatusNotFound, dto.AgentErrContextNotFound, "Task not found")
					return
				}
				if task.TeacherID != claims.Sub {
					agentError(w, http.StatusForbidden, dto.AgentErrContextForbidden,
						"You do not have permission to access this task")
					return
				}
				taskCtx = task
			}
			if req.Context.EvaluationID != nil {
				eval, err := h.evalRepo.GetByID(r.Context(), *req.Context.EvaluationID)
				if err != nil {
					agentError(w, http.StatusNotFound, dto.AgentErrContextNotFound, "Evaluation not found")
					return
				}
				task, err := h.taskRepo.GetByID(r.Context(), eval.TaskID)
				if err != nil {
					agentError(w, http.StatusNotFound, dto.AgentErrContextNotFound, "Task not found")
					return
				}
				if task.TeacherID != claims.Sub {
					agentError(w, http.StatusForbidden, dto.AgentErrContextForbidden,
						"You do not have permission to access this evaluation")
					return
				}
				evalCtx = eval
				taskCtx = task
			}
			if req.Context.ClassID != nil {
				cls, err := h.classRepo.GetByID(r.Context(), *req.Context.ClassID)
				if err != nil {
					agentError(w, http.StatusNotFound, dto.AgentErrContextNotFound, "Class not found")
					return
				}
				if cls.TeacherID != claims.Sub {
					agentError(w, http.StatusForbidden, dto.AgentErrContextForbidden,
						"You do not have permission to access this class")
					return
				}
				classCtx = cls
			}
		}
	}

	// Quota enforcement
	msg := &model.AgentMessage{
		SessionID: req.SessionID,
		Role:      "user",
		Content:   req.Message,
	}
	if err := h.agentSvc.SaveUserMessage(r.Context(), msg); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "message exceeds max length") {
			agentError(w, http.StatusBadRequest, dto.AgentErrMessageTooLong, "Message too long")
			return
		}
		if strings.Contains(errMsg, "session message limit") {
			agentError(w, http.StatusTooManyRequests, dto.AgentErrSessionLimit, "Session message limit reached")
			return
		}
		if strings.Contains(errMsg, "daily message limit") {
			agentError(w, http.StatusTooManyRequests, dto.AgentErrDailyLimit, "Daily message limit reached")
			return
		}
		agentError(w, http.StatusInternalServerError, dto.AgentErrInternal, "Failed to save message")
		return
	}

	// Update session context if provided
	if req.Context != nil {
		newCtxJSON, _ := json.Marshal(req.Context)
		_ = h.agentSvc.(*service.AgentService).UpdateContext(r.Context(), req.SessionID, string(newCtxJSON))
	}

	// Build conversation history
	messages, _ := h.agentSvc.GetMessages(r.Context(), req.SessionID, 20)
	history := buildHistory(messages)

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	// Route to appropriate orchestrator
	var content string
	switch {
	case claims.Role == "student" && evalCtx != nil && h.chatOrch != nil:
		// Tool-augmented student path
		dims, _ := h.taskRepo.GetDimensions(r.Context(), evalCtx.TaskID)
		tctx := &pipeline.ChatToolContext{
			StudentID:   claims.Sub,
			Evaluation:  evalCtx,
			Task:        taskCtx,
			Upload:      uploadCtx,
			ParseResult: parseResultCtx,
			Dimensions:  dims,
		}
		if h.chatOrch.OnToolCall == nil {
			h.chatOrch.OnToolCall = func(name string) {
				writeSSE(w, flusher, map[string]any{"type": "tool_start", "name": name})
			}
		}
		resp, err := h.chatOrch.Run(r.Context(), history, req.Message, tctx)
		if err != nil {
			slog.Error("agent stream: student tool path failed", "error", err.Error())
			content = "抱歉，系统暂时遇到了问题，请稍后再试。"
		} else if resp != nil && len(resp.Choices) > 0 {
			content = resp.Choices[0].Message.Content
		}

	case claims.Role == "teacher" && hasTeacherContext(req.Context) && h.chatOrch != nil:
		// Teacher tool-augmented path
		ttctx := &pipeline.TeacherToolContext{
			TeacherID: claims.Sub,
		}
		if req.Context != nil {
			ttctx.TaskID = req.Context.TaskID
			ttctx.ClassID = req.Context.ClassID
			ttctx.CourseID = req.Context.CourseID
		}
		_ = classCtx // classCtx used for validation only
		systemPrompt := service.BuildTeacherPrompt(service.AgentContext{
			UserID:    claims.Sub,
			UserRole:  claims.Role,
			AgentRole: "teacher",
			TaskID:    req.Context.TaskID,
			ClassID:   req.Context.ClassID,
			CourseID:  req.Context.CourseID,
		})
		if h.chatOrch.OnToolCall == nil {
			h.chatOrch.OnToolCall = func(name string) {
				writeSSE(w, flusher, map[string]any{"type": "tool_start", "name": name})
			}
		}
		resp, err := h.chatOrch.RunTeacher(r.Context(), history, req.Message, ttctx, systemPrompt)
		if err != nil {
			slog.Error("agent stream: teacher tool path failed", "error", err.Error())
			content = "抱歉，系统暂时遇到了问题，请稍后再试。"
		} else if resp != nil && len(resp.Choices) > 0 {
			content = resp.Choices[0].Message.Content
		}

	default:
		// Basic RoleAgentOrchestrator path (no tools)
		if h.roleOrch != nil {
			agCtx := service.AgentContext{
				UserID:    claims.Sub,
				UserRole:  claims.Role,
				AgentRole: req.AgentRole,
				SessionID: req.SessionID,
			}
			if req.Context != nil {
				agCtx.EvaluationID = req.Context.EvaluationID
				agCtx.TaskID = req.Context.TaskID
				agCtx.ClassID = req.Context.ClassID
				agCtx.CourseID = req.Context.CourseID
			}
			resp, err := h.roleOrch.Stream(r.Context(), agCtx, req.Message, history, w)
			if err != nil {
				slog.Error("agent stream: basic path failed", "error", err.Error())
				content = "抱歉，系统暂时遇到了问题，请稍后再试。"
			} else if resp != nil {
				content = resp.Content
			}
			// roleOrch.Stream writes SSE directly, so skip the pseudo-stream below
			// Save assistant response asynchronously
			go func() {
				ctx := context.Background()
				truncated := content
				if len(truncated) > 500 {
					truncated = truncated[:500]
				}
				if truncated != "" {
					h.agentSvc.SaveAssistantMessage(ctx, &model.AgentMessage{
						SessionID: req.SessionID,
						Content:   truncated,
					})
				}
			}()
			return
		}
		content = "AI 助手暂未配置，请联系管理员。"
	}

	// Pseudo-stream the content (for student tool path and teacher tool path)
	if content != "" {
		chunks := chunkString(content, 40)
		for _, chunk := range chunks {
			writeSSE(w, flusher, map[string]any{"type": "text", "content": chunk})
			time.Sleep(30 * time.Millisecond)
		}
	}
	writeSSE(w, flusher, map[string]any{"type": "done"})
	if flusher != nil {
		flusher.Flush()
	}

	// Save assistant response asynchronously
	go func() {
		ctx := context.Background()
		truncated := content
		if len(truncated) > 500 {
			truncated = truncated[:500]
		}
		if truncated != "" {
			h.agentSvc.SaveAssistantMessage(ctx, &model.AgentMessage{
				SessionID: req.SessionID,
				Content:   truncated,
			})
		}
	}()
}

// --- Helpers ---

func toAgentSessionResponse(s model.AgentSession) dto.AgentSessionResponse {
	return dto.AgentSessionResponse{
		ID:           s.ID,
		OwnerID:      s.OwnerID,
		OwnerRole:    s.OwnerRole,
		AgentRole:    s.AgentRole,
		Title:        s.Title,
		ContextJSON:  s.ContextJSON,
		CreatedAt:    s.CreatedAt,
		LastActiveAt: s.LastActiveAt,
	}
}

func toAgentMessageResponse(m model.AgentMessage) dto.AgentMessageResponse {
	return dto.AgentMessageResponse{
		ID:               m.ID,
		SessionID:        m.SessionID,
		Role:             m.Role,
		Content:          m.Content,
		ToolCallID:       m.ToolCallID,
		ToolName:         m.ToolName,
		PromptTokens:     m.PromptTokens,
		CompletionTokens: m.CompletionTokens,
		CreatedAt:        m.CreatedAt,
	}
}

func buildHistory(messages []model.AgentMessage) []llm.ChatMessage {
	var history []llm.ChatMessage
	for _, m := range messages {
		if m.Role == "user" || m.Role == "assistant" {
			history = append(history, llm.NewTextMessage(m.Role, m.Content))
		}
	}
	return history
}

func writeSSE(w http.ResponseWriter, f http.Flusher, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", string(b))
	if f != nil {
		f.Flush()
	}
}

func chunkString(s string, size int) []string {
	if len(s) <= size {
		return []string{s}
	}
	var chunks []string
	runes := []rune(s)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func hasTeacherContext(ctx *dto.AgentContextReq) bool {
	if ctx == nil {
		return false
	}
	return ctx.TaskID != nil || ctx.ClassID != nil || ctx.CourseID != nil || ctx.EvaluationID != nil
}
