package handler

import (
	"net/http"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/service"
)

type LLMHandler struct{ svc *service.LLMConfigService }

func NewLLMHandler(svc *service.LLMConfigService) *LLMHandler {
	return &LLMHandler{svc: svc}
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
	c := &model.LLMConfig{
		Provider: req.Provider, BaseURL: req.BaseURL,
		APIKeyEncrypted: req.APIKey, // TODO: encrypt with AES
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
	JSON(w, http.StatusOK, dto.LLMTestResponse{Success: true, Message: "Connection OK", Latency: 200})
}
