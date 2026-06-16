package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/smartedu/training-eval-system/internal/crypto"
	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/service"
)

type LLMHandler struct {
	svc       *service.LLMConfigService
	masterKey []byte
}

func NewLLMHandler(svc *service.LLMConfigService, masterKey []byte) *LLMHandler {
	return &LLMHandler{svc: svc, masterKey: masterKey}
}

func (h *LLMHandler) List(w http.ResponseWriter, r *http.Request) {
	configs, err := h.svc.List(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]dto.LLMConfigResponse, 0, len(configs))
	for _, c := range configs {
		items = append(items, dto.LLMConfigResponse{
			ID: c.ID, Provider: c.Provider, BaseURL: c.BaseURL,
			ChatModel: c.ChatModel, EmbedModel: c.EmbedModel, IsActive: c.IsActive,
			CreatedAt: c.CreatedAt.Format(time.RFC3339), UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
		})
	}
	JSON(w, http.StatusOK, items)
}

func (h *LLMHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateLLMConfigRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	// Encrypt the API key with AES-256-GCM before persisting (never store plaintext).
	encryptedKey, err := crypto.Encrypt(h.masterKey, []byte(req.APIKey))
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to encrypt API key")
		return
	}
	c := &model.LLMConfig{
		Provider: req.Provider, BaseURL: req.BaseURL,
		APIKeyEncrypted: encryptedKey,
		ChatModel: req.ChatModel, EmbedModel: req.EmbedModel, IsActive: true,
	}
	if err := h.svc.Create(r.Context(), c); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusCreated, dto.LLMConfigResponse{
		ID: c.ID, Provider: c.Provider, BaseURL: c.BaseURL,
		ChatModel: c.ChatModel, EmbedModel: c.EmbedModel, IsActive: c.IsActive,
	})
}

func (h *LLMHandler) Test(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateLLMConfigRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.BaseURL == "" || req.APIKey == "" || req.ChatModel == "" {
		JSON(w, http.StatusOK, dto.LLMTestResponse{Success: false, Message: "缺少 base_url / api_key / chat_model"})
		return
	}

	client := llm.NewClient(req.BaseURL, req.APIKey, req.ChatModel, req.EmbedModel)
	client.SetUseAPIKeyHeader(req.UseAPIKeyHeader)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	start := time.Now()
	_, err := client.Complete(ctx, []llm.ChatMessage{llm.NewTextMessage("user", "ping")}, nil)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		JSON(w, http.StatusOK, dto.LLMTestResponse{Success: false, Message: "连接失败: " + err.Error(), Latency: latency})
		return
	}
	JSON(w, http.StatusOK, dto.LLMTestResponse{Success: true, Message: "连接正常", Latency: latency})
}
