// Package ai provides the LLM client abstraction for the Quorant platform.
// It supports multiple providers (Anthropic, OpenAI-compatible) with a unified interface.
// This is the "platform/ai" package from the architecture doc (Section 2, line 232).
package ai

import (
	"context"
	"fmt"
)

// Provider identifies an LLM provider.
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai" // also compatible with Ollama, vLLM, Together, etc.
)

// CompletionRequest represents a text generation request.
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"` // system prompt (Anthropic-style)
}

// Message is a single message in a conversation.
type Message struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"`
}

// CompletionResponse is the result of a text generation request.
type CompletionResponse struct {
	Content    string `json:"content"`
	Model      string `json:"model"`
	TokensUsed int    `json:"tokens_used,omitempty"`
}

// EmbeddingRequest represents a text embedding request.
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbeddingResponse is the result of an embedding request.
type EmbeddingResponse struct {
	Embedding  []float32 `json:"embedding"`
	Model      string    `json:"model"`
	Dimensions int       `json:"dimensions"`
}

// Client is the unified interface for LLM operations.
// Both Anthropic and OpenAI-compatible providers implement this.
type Client interface {
	// Complete generates a text completion from the given messages.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Embed generates a vector embedding for the given text.
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)

	// Provider returns the name of the provider.
	Provider() Provider
}

// Config holds LLM client configuration.
type Config struct {
	Provider    Provider `json:"provider"`     // "anthropic" or "openai"
	APIKey      string   `json:"api_key"`      // API key (from env/secrets manager)
	BaseURL     string   `json:"base_url"`     // custom base URL (for OpenAI-compatible: Ollama, vLLM, etc.)
	Model       string   `json:"model"`        // default completion model
	EmbedModel  string   `json:"embed_model"`  // default embedding model
	MaxTokens   int      `json:"max_tokens"`   // default max tokens
	Temperature float64  `json:"temperature"`  // default temperature
}

// NewClient creates an LLM client based on the provider configuration.
func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case ProviderAnthropic:
		return NewAnthropicClient(cfg)
	case ProviderOpenAI:
		return NewOpenAIClient(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}
