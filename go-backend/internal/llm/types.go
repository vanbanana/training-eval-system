// Package llm provides LLM client interfaces and implementations.
package llm

import "context"

// LLMClient is the interface for LLM completion calls.
// Both the production *Client and testutil.FakeLLM implement this.
type LLMClient interface {
	Complete(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error)
	Model() string
}