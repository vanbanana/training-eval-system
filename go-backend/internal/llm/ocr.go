package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// ExtractTextFromImage sends a base64-encoded image to the multimodal LLM endpoint
// for text extraction (OCR). Uses the proper multimodal API format with image_url content parts.
// Includes retry logic with backoff for rate-limited API calls.
// Uses ocrModel if set, otherwise falls back to the default model.
func (c *Client) ExtractTextFromImage(ctx context.Context, base64Image string, mimeType string) (string, error) {
	systemMsg := NewTextMessage("system",
		"你是一个 OCR 文字识别助手。请提取图片中的所有文字内容，保持原始排版结构。只输出提取的文字，不要添加任何解释。")

	userMsg := NewMultimodalUserMessage("请提取图片中的所有文字内容，保持原始排版结构。", base64Image, mimeType)

	messages := []ChatMessage{systemMsg, userMsg}

	// Retry with backoff for OCR calls (rate limiting is common during batch processing)
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := c.completeWithModel(ctx, messages)
		if err != nil {
			if attempt < maxRetries-1 {
				backoff := time.Duration(attempt+1) * 5 * time.Second
				slog.Warn("ocr: retrying after failure", "attempt", attempt, "backoff_ms", backoff.Milliseconds(), "error", err.Error())
				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return "", fmt.Errorf("ocr: context cancelled: %w", ctx.Err())
				}
			}
			slog.Error("ocr: multimodal LLM call failed", "error", err.Error(), "mime", mimeType)
			return "", fmt.Errorf("ocr: LLM call failed after %d attempts: %w", maxRetries, err)
		}

		if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
			return "", fmt.Errorf("ocr: empty response from LLM")
		}

		text := string(resp.Choices[0].Message.Content)
		slog.Info("ocr: text extracted", "chars", len(text))
		return text, nil
	}

	return "", fmt.Errorf("ocr: unexpected exit from retry loop")
}

// completeWithModel sends a completion request using the OCR model if available,
// otherwise the default model. Does NOT use thinking mode or tools.
// Uses a separate OCR semaphore so it doesn't starve scoring tasks.
func (c *Client) completeWithModel(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	model := c.model
	if c.ocrModel != "" {
		model = c.ocrModel
	}

	// Bypass circuit breaker for OCR calls - failures should not affect scoring
	// Use separate OCR semaphore to avoid starving scoring tasks
	select {
	case c.ocrSem <- struct{}{}:
		defer func() { <-c.ocrSem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("llm: context cancelled while waiting for OCR slot: %w", ctx.Err())
	}

	req := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Thinking: &ThinkingConfig{Type: "disabled"},
	}

	resp, err := c.doRequest(ctx, "/chat/completions", req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ExtractTextFromPDFPages sends multiple page images to the multimodal LLM for OCR.
// Used as fallback when local PDF text extraction yields insufficient text.
func (c *Client) ExtractTextFromPDFPages(ctx context.Context, base64Images []string) (string, error) {
	if len(base64Images) == 0 {
		return "", fmt.Errorf("ocr: no images provided")
	}

	systemMsg := NewTextMessage("system",
		"你是一个 OCR 文字识别助手。以下是一份多页文档的图片。请提取所有文字内容，保持原始排版结构。只输出提取的文字，不要添加任何解释。")

	// Build multimodal content parts with all page images
	var parts contentParts
	parts = append(parts, contentPart{
		Type: "text",
		Text: fmt.Sprintf("以下是一份 %d 页的文档图片。请按顺序提取所有页面的文字内容：", len(base64Images)),
	})
	for _, b64 := range base64Images {
		parts = append(parts, contentPart{
			Type: "image_url",
			ImageURL: &imageURLPart{
				URL:    fmt.Sprintf("data:image/png;base64,%s", b64),
				Detail: "auto",
			},
		})
	}

	partsJSON, _ := json.Marshal(parts)
	userMsg := ChatMessage{
		Role:    "user",
		Content: "MULTIMODAL:" + string(partsJSON),
	}

	messages := []ChatMessage{systemMsg, userMsg}

	resp, err := c.completeWithModel(ctx, messages)
	if err != nil {
		slog.Error("ocr: PDF multimodal LLM call failed", "error", err.Error(), "pages", len(base64Images))
		return "", fmt.Errorf("ocr: PDF LLM call failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("ocr: empty response from LLM for PDF")
	}

	text := string(resp.Choices[0].Message.Content)
	slog.Info("ocr: PDF text extracted", "chars", len(text), "pages", len(base64Images))
	return text, nil
}
