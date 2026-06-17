package handler

import (
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
	"github.com/smartedu/training-eval-system/internal/sse"
)

type ChatHandler struct {
	svc          *service.ChatService
	broker       *sse.Broker
	llmClient    *llm.Client
	orchestrator *pipeline.ChatOrchestrator
	uploadRepo   repository.UploadRepo
	taskRepo     repository.TaskRepo
	evalRepo     repository.EvaluationRepo
}

func NewChatHandler(
	svc *service.ChatService,
	broker *sse.Broker,
	llmClient *llm.Client,
	orchestrator *pipeline.ChatOrchestrator,
	uploadRepo repository.UploadRepo,
	taskRepo repository.TaskRepo,
	evalRepo repository.EvaluationRepo,
) *ChatHandler {
	return &ChatHandler{
		svc:          svc,
		broker:       broker,
		llmClient:    llmClient,
		orchestrator: orchestrator,
		uploadRepo:   uploadRepo,
		taskRepo:     taskRepo,
		evalRepo:     evalRepo,
	}
}

// Sensitive keywords for basic content filtering.
var sensitiveKeywords = []string{
	"hack", "exploit", "inject", "绕过", "入侵", "攻击",
}

func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	sessions, err := h.svc.ListSessions(r.Context(), claims.Sub)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.ChatSessionResponse, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, dto.ChatSessionResponse{
			ID: s.ID, StudentID: s.StudentID, EvaluationID: s.EvaluationID,
			Title: s.Title, CreatedAt: s.CreatedAt.Format(time.RFC3339),
			LastActiveAt: s.LastActiveAt.Format(time.RFC3339),
		})
	}
	JSON(w, http.StatusOK, items)
}

func (h *ChatHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateSessionRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	claims := middleware.GetClaims(r.Context())
	session := &model.ChatSession{StudentID: claims.Sub, Title: req.Title, EvaluationID: req.EvaluationID}
	if err := h.svc.CreateSession(r.Context(), session); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusCreated, dto.ChatSessionResponse{
		ID: session.ID, StudentID: session.StudentID, Title: session.Title,
		CreatedAt: time.Now().Format(time.RFC3339), LastActiveAt: time.Now().Format(time.RFC3339),
	})
}

func (h *ChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid session ID")
		return
	}
	msgs, err := h.svc.GetMessages(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.ChatMessageResponse, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, dto.ChatMessageResponse{
			ID: m.ID, SessionID: m.SessionID, Role: m.Role, Content: m.Content,
			ToolCallID: m.ToolCallID, ToolName: m.ToolName,
			PromptTokens: m.PromptTokens, CompletionTokens: m.CompletionTokens,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	JSON(w, http.StatusOK, items)
}

func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid session ID")
		return
	}
	_ = h.svc.DeleteSession(r.Context(), id)
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Session deleted"})
}

func (h *ChatHandler) Stream(w http.ResponseWriter, r *http.Request) {
	var req dto.ChatStreamRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	claims := middleware.GetClaims(r.Context())
	studentID := claims.Sub

	// --- Quota enforcement (requirement 22.8) ---
	if req.SessionID > 0 {
		msg := &model.ChatMessage{
			SessionID: req.SessionID,
			Role:      "user",
			Content:   req.Message,
		}
		if err := h.svc.SendMessage(r.Context(), studentID, msg); err != nil {
			if strings.Contains(err.Error(), "daily message limit") {
				Error(w, http.StatusTooManyRequests, "Daily message limit exceeded (50/day).")
				return
			}
			if strings.Contains(err.Error(), "session message limit") {
				Error(w, http.StatusTooManyRequests, "Session message limit reached (20 rounds). Please start a new session.")
				return
			}
			if strings.Contains(err.Error(), "message exceeds") {
				Error(w, http.StatusBadRequest, fmt.Sprintf("Message too long. Maximum %d characters.", 500))
				return
			}
		}
	}

	// --- Sensitive content check (requirement 22.6) ---
	for _, kw := range sensitiveKeywords {
		if strings.Contains(strings.ToLower(req.Message), kw) {
			Error(w, http.StatusBadRequest, "Your message contains content that cannot be processed. Please rephrase your question.")
			return
		}
	}

	// --- Load evaluation context for context-aware chat ---
	var tctx *pipeline.ChatToolContext
	if req.EvaluationID != nil && *req.EvaluationID > 0 {
		eval, err := h.evalRepo.GetByID(r.Context(), *req.EvaluationID)
		if err == nil && eval != nil && eval.StudentID == studentID {
			task, _ := h.taskRepo.GetByID(r.Context(), eval.TaskID)
			dims, _ := h.taskRepo.GetDimensions(r.Context(), eval.TaskID)
			upload, _ := h.uploadRepo.GetByID(r.Context(), eval.UploadID)
			var pr *model.ParseResult
			if upload != nil {
				pr, _ = h.uploadRepo.GetParseResult(r.Context(), upload.ID)
			}

			tctx = &pipeline.ChatToolContext{
				StudentID:   studentID,
				Evaluation:  eval,
				Task:        task,
				Upload:      upload,
				ParseResult: pr,
				Dimensions:  dims,
			}
		}
	}

	// --- Build conversation history ---
	var history []llm.ChatMessage
	if req.SessionID > 0 {
		msgs, _ := h.svc.GetMessages(r.Context(), req.SessionID)
		for _, m := range msgs {
			history = append(history, llm.NewTextMessage(m.Role, m.Content))
		}
	}

	// --- Stream LLM response ---
	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, hasFlusher := w.(http.Flusher)

	// If orchestrator is available, use it for tool-augmented response
	if h.orchestrator != nil && h.llmClient != nil && tctx != nil {
		// Set callback to emit tool_call progress events to frontend
		h.orchestrator.OnToolCall = func(toolName string) {
			evt, _ := json.Marshal(map[string]string{"type": "tool_call", "name": toolName})
			fmt.Fprintf(w, "data: %s\n\n", evt)
			if hasFlusher {
				flusher.Flush()
			}
		}

		resp, err := h.orchestrator.Run(r.Context(), history, req.Message, tctx)
		if err != nil {
			slog.Error("chat orchestrator failed", "error", err.Error())
			// Fall through to basic streaming
		} else if resp != nil && len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
			// Pseudo-stream: send content in small chunks for better UX
			content := resp.Choices[0].Message.Content
			chunkSize := 40
			for i := 0; i < len(content); i += chunkSize {
				end := i + chunkSize
				if end > len(content) {
					end = len(content)
				}
				chunk := content[i:end]
				tokenJSON, _ := json.Marshal(map[string]string{"type": "text", "content": chunk})
				fmt.Fprintf(w, "data: %s\n\n", tokenJSON)
				if hasFlusher {
					flusher.Flush()
				}
				time.Sleep(30 * time.Millisecond)
			}
			fmt.Fprintf(w, "data: {\"type\":\"done\"}\n\n")
			if hasFlusher {
				flusher.Flush()
			}

			// Save assistant message
			if req.SessionID > 0 {
				asstMsg := &model.ChatMessage{
					SessionID: req.SessionID,
					Role:      "assistant",
					Content:   content,
				}
				if resp.Usage != nil {
					asstMsg.PromptTokens = resp.Usage.PromptTokens
					asstMsg.CompletionTokens = resp.Usage.CompletionTokens
				}
				_ = h.svc.SendMessage(r.Context(), studentID, asstMsg)
			}
			return
		}
	}

	// --- Fallback: basic streaming without orchestrator ---
	if h.llmClient == nil {
		fmt.Fprintf(w, "data: {\"type\":\"text\",\"content\":\"AI 助手暂未配置，请联系管理员在 LLM 配置页面设置 API Key。\"}\n\n")
		if hasFlusher {
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: {\"type\":\"done\"}\n\n")
		if hasFlusher {
			flusher.Flush()
		}
		return
	}

	// Build messages with context
	messages := []llm.ChatMessage{}

	// Use context-aware system prompt if available
	if tctx != nil {
		sysPrompt := pipeline.BuildChatSystemPrompt(tctx.Task, tctx.Evaluation, tctx.ParseResult, tctx.Dimensions)
		messages = append(messages, llm.NewTextMessage("system", sysPrompt))
	} else {
		messages = append(messages, llm.NewTextMessage("system",
			"你是实训评价 AI 助手，帮助学生理解评价结果并提供改进建议。回答要简洁、具体、有建设性。"))
	}

	for _, h := range history {
		messages = append(messages, h)
	}
	messages = append(messages, llm.NewTextMessage("user", req.Message))

	result, err := h.llmClient.StreamChat(r.Context(), w, messages)
	if err != nil {
		return
	}

	// Save assistant response
	if req.SessionID > 0 && result != nil {
		asstMsg := &model.ChatMessage{
			SessionID:        req.SessionID,
			Role:             "assistant",
			Content:          result.Content,
			PromptTokens:     result.PromptTokens,
			CompletionTokens: result.CompletionTokens,
		}
		_ = h.svc.SendMessage(r.Context(), studentID, asstMsg)
	}
}
