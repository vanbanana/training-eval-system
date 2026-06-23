package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// StreamResult holds the final result of a streaming chat completion.
type StreamResult struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
}

// StreamDelta represents a single chunk from the streaming response.
type StreamDelta struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *UsageInfo `json:"usage,omitempty"`
}

// StreamChat sends a streaming chat completion and writes SSE events to the writer.
// Returns the accumulated full response, token counts, and any error.
func (c *Client) StreamChat(ctx context.Context, w http.ResponseWriter, messages []ChatMessage) (*StreamResult, error) {
	if err := c.breaker.Allow(); err != nil {
		return nil, fmt.Errorf("llm: circuit breaker open: %w", err)
	}

	req := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
		Thinking: &ThinkingConfig{Type: "disabled"},
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal stream request: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("llm: create stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// MiMo uses the api-key header; standard OpenAI uses Authorization: Bearer.
	if c.useAPIKeyHeader {
		httpReq.Header.Set("api-key", c.apiKey)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("llm: stream http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("llm: stream API returned status %d: %s", resp.StatusCode, string(body))
	}

	c.breaker.RecordSuccess()

	// Set SSE headers on the response writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, hasFlusher := w.(http.Flusher)

	// Read SSE stream from LLM and forward to client
	var content strings.Builder
	result := &StreamResult{}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {json}" or "data: [DONE]"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var delta StreamDelta
		if err := json.Unmarshal([]byte(data), &delta); err != nil {
			continue
		}

		if len(delta.Choices) > 0 && delta.Choices[0].Delta.Content != "" {
			token := delta.Choices[0].Delta.Content
			content.WriteString(token)

			// Forward token to client as SSE (unnamed event; type is in JSON body)
			tokenJSON, _ := json.Marshal(map[string]string{"type": "text", "content": token})
			fmt.Fprintf(w, "data: %s\n\n", tokenJSON)
			if hasFlusher {
				flusher.Flush()
			}
		}

		// Capture usage info if present (some providers send it in the last chunk)
		if delta.Usage != nil {
			result.PromptTokens = delta.Usage.PromptTokens
			result.CompletionTokens = delta.Usage.CompletionTokens
		}
	}

	if err := scanner.Err(); err != nil {
		// Send error event to client (unnamed event; type is in JSON body)
		errJSON, _ := json.Marshal(map[string]string{"type": "error", "code": "STREAM_ERROR", "message": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", errJSON)
		if hasFlusher {
			flusher.Flush()
		}
		return nil, fmt.Errorf("llm: stream read error: %w", err)
	}

	// Send done event (unnamed event; type is in JSON body)
	fmt.Fprintf(w, "data: {\"type\":\"done\"}\n\n")
	if hasFlusher {
		flusher.Flush()
	}

	result.Content = content.String()
	duration := time.Since(start)
	slog.Info("llm stream completed", "duration_ms", duration.Milliseconds(), "tokens", result.CompletionTokens)

	return result, nil
}
