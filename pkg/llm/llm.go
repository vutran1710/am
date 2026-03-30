package llm

import "context"

// LLM is the abstraction for any language model backend.
// Callers don't know if it's an API, stdin pipe, or local model.
type LLM interface {
	// Complete sends a system prompt + user prompt and returns the response text.
	Complete(ctx context.Context, system, prompt string) (string, error)

	// Close releases resources (e.g. stdin process).
	Close() error
}
