// Package llm provides LLM client interfaces and implementations.
package llm

import (
	"context"
	"net/http"
)

// LLMClient is the interface for LLM completion calls.
// Both the production *Client and testutil.FakeLLM implement this.
type LLMClient interface {
	Complete(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error)
	StreamChat(ctx context.Context, w http.ResponseWriter, messages []ChatMessage) (*StreamResult, error)
	ExtractTextFromImage(ctx context.Context, base64Image string, mimeType string) (string, error)
	IsBreakerOpen() bool
	Model() string
}