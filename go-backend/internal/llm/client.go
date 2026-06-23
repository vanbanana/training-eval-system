// Package llm provides an OpenAI-compatible HTTP client with circuit breaker.
// Supports Xiaomi MiMo V2.5 (api-key header, thinking parameter, max_completion_tokens).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// APIError represents a non-2xx response from the LLM provider, carrying the
// HTTP status code so callers can decide whether retrying is worthwhile.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("llm: API returned status %d: %s", e.StatusCode, e.Body)
}

// Retryable reports whether re-issuing the same request could plausibly succeed.
// Auth (401/403), bad-request (400/404/422) and quota-exhaustion (429) errors are
// permanent for the current request and should fail fast instead of hammering the
// provider; only transient server errors (5xx) and rate limits without quota
// exhaustion are worth retrying.
func (e *APIError) Retryable() bool {
	switch e.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden,
		http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity:
		return false
	case http.StatusTooManyRequests:
		// Distinguish transient rate limiting from hard quota exhaustion, which
		// no amount of retrying will fix within the same plan window.
		return !strings.Contains(e.Body, "使用上限") &&
			!strings.Contains(strings.ToLower(e.Body), "quota") &&
			!strings.Contains(strings.ToLower(e.Body), "insufficient")
	default:
		return e.StatusCode >= 500
	}
}

// IsRetryable returns true when err is nil-safe retryable. Non-APIError errors
// (e.g. network failures, timeouts) are treated as retryable.
func IsRetryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable()
	}
	return true
}

// Client is an OpenAI-compatible LLM HTTP client.
// Supports MiMo-specific features: api-key header, thinking parameter, max_completion_tokens.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
	embedModel string
	ocrModel   string // separate model for multimodal OCR (e.g. mimo-v2.5 supports vision, mimo-v2.5-pro does not)
	breaker    *CircuitBreaker
	// MiMo-specific: use api-key header instead of Authorization: Bearer (optional, defaults to Bearer)
	useAPIKeyHeader bool
	// Concurrency limiter: max concurrent LLM requests to avoid API rate limiting
	sem chan struct{}
	// Separate concurrency limiter for OCR calls so they don't starve scoring
	ocrSem chan struct{}
}

// NewClient creates a new LLM client.
func NewClient(baseURL, apiKey, model, embedModel string) *Client {
	maxConcurrent := 8 // concurrent LLM API calls; MiMo API supports higher concurrency
	maxOCR := 4        // separate OCR concurrency so it doesn't starve scoring
	return &Client{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		embedModel: embedModel,
		breaker:    NewCircuitBreaker(50, 30*time.Second), // higher threshold for batch workloads
		sem:        make(chan struct{}, maxConcurrent),
		ocrSem:     make(chan struct{}, maxOCR),
	}
}

// NewMiMoClient creates a client configured for Xiaomi MiMo API.
// Uses api-key header for authentication and supports thinking parameter.
func NewMiMoClient(apiKey, model string) *Client {
	c := NewClient("https://token-plan-cn.xiaomimimo.com/v1", apiKey, model, "")
	c.useAPIKeyHeader = true
	return c
}

// SetUseAPIKeyHeader configures whether to use the api-key header (MiMo style)
// instead of the standard Authorization: Bearer header.
func (c *Client) SetUseAPIKeyHeader(v bool) {
	c.useAPIKeyHeader = v
}

// SetOCRModel sets the model to use for multimodal OCR calls.
// If not set, falls back to the default model.
func (c *Client) SetOCRModel(model string) {
	c.ocrModel = model
}

// ChatMessage represents a message in the chat completion API.
// Supports both simple text content (Content as string) and multimodal content parts (Content as MULTIMODAL: prefixed JSON).
type ChatMessage struct {
	Role       string     `json:"-"`
	Content    string     `json:"-"`
	ToolCallID string     `json:"-"`
	Name       string     `json:"-"`
	ToolCalls  []ToolCall `json:"-"`
}

// contentParts represents the multimodal content array format.
type contentParts []contentPart

// contentPart is a single multimodal content element.
type contentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *imageURLPart `json:"image_url,omitempty"`
}

// imageURLPart holds the image URL for multimodal input.
type imageURLPart struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for ChatMessage.
// When Content starts with "MULTIMODAL:", it renders as an array of content parts.
// Otherwise it renders as a plain string.
func (m ChatMessage) MarshalJSON() ([]byte, error) {
	type Alias ChatMessage
	// Build a map for flexible marshaling
	mp := map[string]any{
		"role": m.Role,
	}
	if m.ToolCallID != "" {
		mp["tool_call_id"] = m.ToolCallID
	}
	if m.Name != "" {
		mp["name"] = m.Name
	}
	if len(m.ToolCalls) > 0 {
		mp["tool_calls"] = m.ToolCalls
	}

	if len(m.Content) >= 11 && m.Content[:11] == "MULTIMODAL:" {
		var parts contentParts
		if err := json.Unmarshal([]byte(m.Content[11:]), &parts); err != nil {
			return nil, fmt.Errorf("unmarshal multimodal content: %w", err)
		}
		mp["content"] = parts
	} else {
		mp["content"] = m.Content
	}

	return json.Marshal(mp)
}

// UnmarshalJSON implements custom JSON unmarshaling for ChatMessage.
func (m *ChatMessage) UnmarshalJSON(data []byte) error {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["role"]; ok {
		json.Unmarshal(v, &m.Role)
	}
	if v, ok := raw["tool_call_id"]; ok {
		json.Unmarshal(v, &m.ToolCallID)
	}
	if v, ok := raw["name"]; ok {
		json.Unmarshal(v, &m.Name)
	}
	if v, ok := raw["tool_calls"]; ok {
		json.Unmarshal(v, &m.ToolCalls)
	}
	if v, ok := raw["content"]; ok {
		// Try string first, then array (multimodal)
		var s string
		if json.Unmarshal(v, &s) == nil {
			m.Content = s
		} else {
			// It's a multimodal array — store as MULTIMODAL: prefixed JSON
			var parts contentParts
			if err := json.Unmarshal(v, &parts); err != nil {
				return fmt.Errorf("unmarshal content: %w", err)
			}
			partsJSON, _ := json.Marshal(parts)
			m.Content = "MULTIMODAL:" + string(partsJSON)
		}
	}
	return nil
}

// NewMultimodalUserMessage creates a user message with text + image for multimodal models.
func NewMultimodalUserMessage(text, base64Image, mimeType string) ChatMessage {
	parts := contentParts{
		{Type: "text", Text: text},
		{Type: "image_url", ImageURL: &imageURLPart{
			URL:    fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image),
			Detail: "auto",
		}},
	}
	partsJSON, _ := json.Marshal(parts)
	return ChatMessage{
		Role:    "user",
		Content: "MULTIMODAL:" + string(partsJSON),
	}
}

// NewTextMessage creates a simple text chat message.
func NewTextMessage(role, content string) ChatMessage {
	return ChatMessage{Role: role, Content: content}
}

// ToolCall represents a function call made by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the function name and arguments JSON string.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatRequest is the request body for chat completions.
// Supports MiMo-specific fields: max_completion_tokens, thinking.
type ChatRequest struct {
	Model               string          `json:"model"`
	Messages            []ChatMessage   `json:"messages"`
	Temperature         float64         `json:"temperature,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`            // standard OpenAI
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"` // MiMo preferred
	Stream              bool            `json:"stream,omitempty"`
	Tools               []Tool          `json:"tools,omitempty"`
	TopP                float64         `json:"top_p,omitempty"`
	Stop                any             `json:"stop,omitempty"`
	FrequencyPenalty    float64         `json:"frequency_penalty,omitempty"`
	PresencePenalty     float64         `json:"presence_penalty,omitempty"`
	Thinking            *ThinkingConfig `json:"thinking,omitempty"` // MiMo thinking mode
}

// ThinkingConfig controls the thinking/reasoning mode.
// MiniMax uses "adaptive" (enabled) or "disabled"; MiMo uses "enabled" or "disabled".
type ThinkingConfig struct {
	Type string `json:"type"` // "disabled" or "adaptive"
}

// Tool represents a function calling tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction defines a callable function.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ChatResponse is the response from chat completions.
type ChatResponse struct {
	ID      string       `json:"id"`
	Choices []ChatChoice `json:"choices"`
	Usage   *UsageInfo   `json:"usage"`
}

// ChatChoice represents a single completion choice.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// UsageInfo holds token usage information.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbeddingRequest is the request for embeddings API.
type EmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
}

// EmbeddingResponse is the response from embeddings API.
type EmbeddingResponse struct {
	Data  []EmbeddingData `json:"data"`
	Model string          `json:"model"`
	Usage *UsageInfo      `json:"usage"`
}

// EmbeddingData holds a single embedding vector.
type EmbeddingData struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// Complete sends a non-streaming chat completion request.
func (c *Client) Complete(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error) {
	return c.completeWithOpts(ctx, messages, tools, false, 0, false)
}

// CompleteWithThinking sends a non-streaming request with thinking mode enabled (MiMo specific).
func (c *Client) CompleteWithThinking(ctx context.Context, messages []ChatMessage, tools []Tool, maxTokens int) (*ChatResponse, error) {
	return c.completeWithOpts(ctx, messages, tools, true, maxTokens, false)
}

func (c *Client) completeWithOpts(ctx context.Context, messages []ChatMessage, tools []Tool, thinking bool, maxTokens int, stream bool) (*ChatResponse, error) {
	if err := c.breaker.Allow(); err != nil {
		return nil, fmt.Errorf("llm: circuit breaker open: %w", err)
	}

	// Acquire concurrency slot
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("llm: context cancelled while waiting for concurrency slot: %w", ctx.Err())
	}

	req := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
		Stream:   stream,
	}

	if maxTokens > 0 {
		req.MaxCompletionTokens = maxTokens
	}

	if thinking {
		req.Thinking = &ThinkingConfig{Type: "adaptive"}
	} else {
		req.Thinking = &ThinkingConfig{Type: "disabled"}
	}

	// For non-thinking scoring/verification tasks, set reasonable temperature
	if len(tools) > 0 {
		req.Temperature = 0.1
	}

	resp, err := c.doRequest(ctx, "/chat/completions", req)
	if err != nil {
		c.breaker.RecordFailure()
		return nil, err
	}
	c.breaker.RecordSuccess()
	return resp, nil
}

func (c *Client) doRequest(ctx context.Context, path string, body any) (*ChatResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}

	url := c.baseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("llm: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// MiMo uses api-key header; standard OpenAI uses Authorization: Bearer
	if c.useAPIKeyHeader {
		httpReq.Header.Set("api-key", c.apiKey)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	duration := time.Since(start)

	if err != nil {
		slog.Error("llm request failed", "url", url, "duration_ms", duration.Milliseconds(), "error", err.Error())
		return nil, fmt.Errorf("llm: http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Log the actual model used from the request body
	var reqModel string
	if chatReq, ok := body.(ChatRequest); ok && chatReq.Model != "" {
		reqModel = chatReq.Model
	} else {
		reqModel = c.model
	}

	slog.Info("llm request completed",
		"model", reqModel,
		"endpoint", path,
		"status", resp.StatusCode,
		"duration_ms", duration.Milliseconds(),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("llm: unmarshal response: %w", err)
	}

	return &chatResp, nil
}

// Embed generates embedding vectors for the given texts.
// Falls back to a simple TF-based approach if no embedding model is configured.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if c.embedModel == "" {
		return nil, fmt.Errorf("llm: no embedding model configured")
	}

	if err := c.breaker.Allow(); err != nil {
		return nil, fmt.Errorf("llm: circuit breaker open: %w", err)
	}

	// Acquire concurrency slot
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("llm: context cancelled while waiting for concurrency slot: %w", ctx.Err())
	}

	req := EmbeddingRequest{
		Model:          c.embedModel,
		Input:          texts,
		EncodingFormat: "float",
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal embed request: %w", err)
	}

	url := c.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("llm: create embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if c.useAPIKeyHeader {
		httpReq.Header.Set("api-key", c.apiKey)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	duration := time.Since(start)

	if err != nil {
		c.breaker.RecordFailure()
		slog.Error("embed request failed", "duration_ms", duration.Milliseconds(), "error", err.Error())
		return nil, fmt.Errorf("llm: embed request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("llm: embed API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	c.breaker.RecordSuccess()

	var embedResp EmbeddingResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("llm: unmarshal embed response: %w", err)
	}

	slog.Info("embed request completed", "model", c.embedModel, "texts", len(texts), "duration_ms", duration.Milliseconds())

	result := make([][]float64, len(embedResp.Data))
	for _, d := range embedResp.Data {
		result[d.Index] = d.Embedding
	}
	return result, nil
}

// Model returns the configured chat model name.
func (c *Client) Model() string {
	return c.model
}

// EmbedModel returns the configured embedding model name.
func (c *Client) EmbedModel() string {
	return c.embedModel
}
