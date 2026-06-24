// Package handler — Agent API handler for the unified AI agent system.
package handler

import (
	"context"
	"encoding/json"
	"errors"
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
	agentSvc      *service.AgentService
llmClient     llm.LLMClient
		evalRepo      repository.EvaluationRepo
	uploadRepo    repository.UploadRepo
	taskRepo      repository.TaskRepo
	classRepo     repository.ClassRepo
	courseRepo    repository.CourseRepo
	auditRepo     repository.AuditRepo
	usageSvc      *service.UsageService
	chatOrch      *pipeline.ChatOrchestrator
	roleOrch      *service.RoleAgentOrchestrator
	streamTracker *StreamTracker
}

// SetStreamTracker sets the concurrent stream tracker (optional, nil-safe).
func (h *AgentHandler) SetStreamTracker(t *StreamTracker) {
	h.streamTracker = t
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(
	agentSvc *service.AgentService,
	llmClient llm.LLMClient,
	evalRepo repository.EvaluationRepo,
	uploadRepo repository.UploadRepo,
	taskRepo repository.TaskRepo,
	classRepo repository.ClassRepo,
	courseRepo repository.CourseRepo,
	chatOrch *pipeline.ChatOrchestrator,
	roleOrch *service.RoleAgentOrchestrator,
	auditRepo repository.AuditRepo,
	usageSvc *service.UsageService,
) *AgentHandler {
	return &AgentHandler{
		agentSvc:   agentSvc,
		llmClient:  llmClient,
		evalRepo:   evalRepo,
		uploadRepo: uploadRepo,
		taskRepo:   taskRepo,
		classRepo:  classRepo,
		courseRepo: courseRepo,
		auditRepo:  auditRepo,
		usageSvc:   usageSvc,
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
	streamStart := time.Now()
	claims := middleware.GetClaims(r.Context())

	var req dto.AgentStreamRequest
	if err := Decode(r, &req); err != nil {
		agentError(w, http.StatusBadRequest, dto.AgentErrInvalidRequest, "Invalid request body")
		return
	}

	// Audit tracking state
	var toolNames []string
	var promptTokens, completionTokens int
	var streamSuccess bool
	var streamErr string
	var streamErrCode string

	// Defer audit log entry
	defer func() {
		if h.auditRepo == nil || claims == nil {
			return
		}
		latencyMs := time.Since(streamStart).Milliseconds()
		result := "success"
		if !streamSuccess {
			result = "failure"
		}
		payload := map[string]any{
			"agent_role":        req.AgentRole,
			"session_id":        req.SessionID,
			"tool_names":        toolNames,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"latency_ms":        latencyMs,
		}
		errDetail := streamErr
		if streamErrCode != "" {
			errDetail = streamErrCode
			payload["error_code"] = streamErrCode
		}
		if errDetail != "" {
			payload["error"] = errDetail
		}
		userID := claims.Sub
		go func() {
			_ = h.auditRepo.Create(context.Background(), &model.AuditLog{
				UserID:     &userID,
				Username:   claims.Username,
				Role:       claims.Role,
				Action:     "agent.chat." + result,
				TargetType: "agent_session",
				TargetID:   fmt.Sprintf("%d", req.SessionID),
				Result:     result,
				Detail: fmt.Sprintf("agent_role=%s tools=%d tokens=%d/%d latency=%dms",
					req.AgentRole, len(toolNames), promptTokens, completionTokens, latencyMs),
				Payload: payload,
			})
		}()
	}()

	// Defer token usage recording (fire-and-forget, T8.3)
	defer func() {
		if h.usageSvc == nil || claims == nil {
			return
		}
		h.usageSvc.RecordUsage(context.Background(), &model.TokenUsage{
			UserID:           claims.Sub,
			UserRole:         claims.Role,
			AgentRole:        req.AgentRole,
			SessionID:        req.SessionID,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			ToolCallCount:    len(toolNames),
			Success:          streamSuccess,
			LatencyMs:        time.Since(streamStart).Milliseconds(),
			ErrorCode:        streamErr,
		})
	}()

	// Empty message check
	if strings.TrimSpace(req.Message) == "" {
		agentError(w, http.StatusBadRequest, dto.AgentErrInvalidRequest, "Message cannot be empty")
		return
	}

// Message length check (measured in runes to handle multi-byte characters correctly)
		if len([]rune(req.Message)) > 500 {
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
					// Anti-enumeration: return 404 (same as "not found") to prevent ID probing
					agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
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
					// Anti-enumeration: return 404
					agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
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
						// Anti-enumeration: return 404
						agentError(w, http.StatusNotFound, dto.AgentErrSessionNotFound, "Session not found")
						return
					}
					_ = cls // class validated for ownership, stored for later use if needed
				}
		}
	}

	// Concurrency tracking: acquire slot before SSE, release on stream end
	if h.streamTracker != nil {
		if err := h.streamTracker.Acquire(claims.Sub); err != nil {
			agentError(w, http.StatusTooManyRequests, dto.AgentErrConcurrentLimit, err.Error())
			return
		}
		defer h.streamTracker.Release(claims.Sub)
	}

	// T8.2: Circuit breaker pre-check — short-circuit if LLM is unavailable
	if h.llmClient != nil && h.llmClient.IsBreakerOpen() {
		agentError(w, http.StatusServiceUnavailable, dto.AgentErrLLMUnavailable,
			"AI 服务暂时不可用，请稍后再试")
		return
	}

	// Quota enforcement (role-based)
	msg := &model.AgentMessage{
		SessionID: req.SessionID,
		Role:      "user",
		Content:   req.Message,
	}
	if err := h.agentSvc.SaveUserMessageWithRole(r.Context(), msg, claims.Role); err != nil {
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
		_ = h.agentSvc.UpdateContext(r.Context(), req.SessionID, string(newCtxJSON))
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
			OnToolCall: func(name string) {
				toolNames = append(toolNames, name)
				writeSSE(w, flusher, map[string]any{"type": "tool_start", "name": name})
			},
		}
		resp, err := h.chatOrch.Run(r.Context(), history, req.Message, tctx)
		if err != nil {
			slog.Error("agent stream: student tool path failed", "error", err.Error())
			streamErr = err.Error()
			_, content = ClassifyLLMError(err)
		} else if resp != nil && len(resp.Choices) > 0 {
			content = resp.Choices[0].Message.Content
			if resp.Usage != nil {
				promptTokens = resp.Usage.PromptTokens
				completionTokens = resp.Usage.CompletionTokens
			}
			streamSuccess = true
		}

	case claims.Role == "teacher" && hasTeacherContext(req.Context) && h.chatOrch != nil:
		// Teacher tool-augmented path
		ttctx := &pipeline.TeacherToolContext{
			TeacherID: claims.Sub,
			OnToolCall: func(name string) {
				toolNames = append(toolNames, name)
				writeSSE(w, flusher, map[string]any{"type": "tool_start", "name": name})
			},
		}
		if req.Context != nil {
ttctx.TaskID = req.Context.TaskID
				ttctx.ClassID = req.Context.ClassID
				ttctx.CourseID = req.Context.CourseID
			}
			systemPrompt := service.BuildTeacherPrompt(service.AgentContext{
			UserID:    claims.Sub,
			UserRole:  claims.Role,
			AgentRole: "teacher",
			TaskID:    req.Context.TaskID,
			ClassID:   req.Context.ClassID,
			CourseID:  req.Context.CourseID,
		})
		resp, err := h.chatOrch.RunTeacher(r.Context(), history, req.Message, ttctx, systemPrompt)
		if err != nil {
			slog.Error("agent stream: teacher tool path failed", "error", err.Error())
			streamErr = err.Error()
			_, content = ClassifyLLMError(err)
		} else if resp != nil && len(resp.Choices) > 0 {
			content = resp.Choices[0].Message.Content
			if resp.Usage != nil {
				promptTokens = resp.Usage.PromptTokens
				completionTokens = resp.Usage.CompletionTokens
			}
			streamSuccess = true
		}

	case claims.Role == "admin" && h.chatOrch != nil:
		// Admin tool-augmented path
		actx := &pipeline.AdminToolContext{
			AdminID: claims.Sub,
			OnToolCall: func(name string) {
				toolNames = append(toolNames, name)
				writeSSE(w, flusher, map[string]any{"type": "tool_start", "name": name})
			},
		}
		systemPrompt := service.BuildAdminPrompt(service.AgentContext{
			UserID:    claims.Sub,
			UserRole:  claims.Role,
			AgentRole: "admin",
		})
		resp, err := h.chatOrch.RunAdmin(r.Context(), history, req.Message, actx, systemPrompt)
		if err != nil {
			slog.Error("agent stream: admin tool path failed", "error", err.Error())
			streamErr = err.Error()
			_, content = ClassifyLLMError(err)
		} else if resp != nil && len(resp.Choices) > 0 {
			content = resp.Choices[0].Message.Content
			if resp.Usage != nil {
				promptTokens = resp.Usage.PromptTokens
				completionTokens = resp.Usage.CompletionTokens
			}
			streamSuccess = true
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
				streamErr = err.Error()
				_, content = ClassifyLLMError(err)
			} else if resp != nil {
				content = resp.Content
				promptTokens = resp.PromptTokens
				completionTokens = resp.CompletionTokens
				streamSuccess = true
			}
			// roleOrch.Stream writes SSE directly, so skip the pseudo-stream below
			// But if it errored, emit a terminal done event so the client doesn't hang
			if err != nil {
				writeSSE(w, flusher, map[string]any{"type": "error", "code": "AGENT_STREAM_INTERRUPTED", "message": content})
				writeSSE(w, flusher, map[string]any{"type": "done"})
				if flusher != nil {
					flusher.Flush()
				}
				// Save assistant response asynchronously (the error message, not the failed content)
				go func() {
					truncated := truncateContent(content, 500)
					if truncated != "" {
						h.agentSvc.SaveAssistantMessage(context.Background(), &model.AgentMessage{
							SessionID: req.SessionID,
							Content:   truncated,
						})
					}
				}()
				return
			}
			// Success path: roleOrch.Stream emitted its own SSE, save response
			go func() {
				truncated := truncateContent(content, 500)
				if truncated != "" {
					h.agentSvc.SaveAssistantMessage(context.Background(), &model.AgentMessage{
						SessionID: req.SessionID,
						Content:   truncated,
					})
				}
			}()
			return
		}
		content = "AI 助手暂未配置，请联系管理员。"
	}

	// Pseudo-stream the content (for student/tool paths), with context cancellation
	if content != "" {
		chunks := chunkString(content, 40)
		for _, chunk := range chunks {
			select {
			case <-r.Context().Done():
				slog.Warn("agent stream: client disconnected during pseudo-stream", "session_id", req.SessionID)
				// Write done anyway so parser doesn't hang
				writeSSE(w, flusher, map[string]any{"type": "done"})
				if flusher != nil {
					flusher.Flush()
				}
				return
			default:
			}
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
		truncated := truncateContent(content, 500)
		if truncated != "" {
			h.agentSvc.SaveAssistantMessage(context.Background(), &model.AgentMessage{
				SessionID: req.SessionID,
				Content:   truncated,
			})
		}
	}()
}

// --- Helpers ---

// truncateContent truncates content to maxLen runes without breaking UTF-8.
func truncateContent(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

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

// ClassifyLLMError maps an LLM/orchestrator error to the appropriate agent error code and user message (T8.2).
func ClassifyLLMError(err error) (code string, userMsg string) {
	if err == nil {
		return "", ""
	}
	if errors.Is(err, llm.ErrFirstTokenTimeout) || errors.Is(err, llm.ErrTotalTimeout) {
		return dto.AgentErrLLMTimeout, "AI 响应超时，请稍后再试或简化问题后重试。"
	}
	if errors.Is(err, llm.ErrCircuitOpen) {
		return dto.AgentErrLLMUnavailable, "AI 服务暂时不可用，请稍后再试。"
	}
	// Check for generic timeout strings from context or HTTP client
	errMsg := err.Error()
	if strings.Contains(errMsg, "context deadline exceeded") || strings.Contains(errMsg, "timeout") {
		return dto.AgentErrLLMTimeout, "AI 响应超时，请稍后再试或简化问题后重试。"
	}
	if strings.Contains(errMsg, "circuit breaker") {
		return dto.AgentErrLLMUnavailable, "AI 服务暂时不可用，请稍后再试。"
	}
	return dto.AgentErrInternal, "抱歉，系统暂时遇到了问题，请稍后再试。"
}
