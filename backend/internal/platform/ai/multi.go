package ai

import (
	"context"
	"fmt"
)

// MultiClient routes completion and embedding requests to different providers.
// This is the recommended setup: use Anthropic for completions, OpenAI for embeddings.
type MultiClient struct {
	completion Client
	embedding  Client
}

// NewMultiClient creates a client that routes to different providers.
// completionClient handles Complete calls, embeddingClient handles Embed calls.
func NewMultiClient(completionClient, embeddingClient Client) *MultiClient {
	return &MultiClient{
		completion: completionClient,
		embedding:  embeddingClient,
	}
}

func (c *MultiClient) Provider() Provider { return "multi" }

func (c *MultiClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if c.completion == nil {
		return nil, fmt.Errorf("no completion provider configured")
	}
	return c.completion.Complete(ctx, req)
}

func (c *MultiClient) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if c.embedding == nil {
		return nil, fmt.Errorf("no embedding provider configured")
	}
	return c.embedding.Embed(ctx, req)
}

// NewClientFromEnv creates an LLM client from environment configuration.
// Supports three modes:
//   - Single provider: set LLM_PROVIDER + LLM_API_KEY
//   - Multi provider: set LLM_COMPLETION_PROVIDER + LLM_EMBED_PROVIDER with separate keys
//   - Disabled: set LLM_PROVIDER=none (returns a stub that errors on every call)
func NewClientFromEnv(cfg Config, embedCfg *Config) (Client, error) {
	if cfg.Provider == "none" || cfg.Provider == "" {
		return &stubClient{}, nil
	}

	completionClient, err := NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating completion client: %w", err)
	}

	if embedCfg == nil {
		// Single provider for both
		return completionClient, nil
	}

	embeddingClient, err := NewClient(*embedCfg)
	if err != nil {
		return nil, fmt.Errorf("creating embedding client: %w", err)
	}

	return NewMultiClient(completionClient, embeddingClient), nil
}

// stubClient returns errors for all operations.
// Used when LLM is disabled (e.g., in dev without API keys).
type stubClient struct{}

func (c *stubClient) Provider() Provider { return "none" }
func (c *stubClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	return nil, fmt.Errorf("LLM not configured: set LLM_PROVIDER and LLM_API_KEY environment variables")
}
func (c *stubClient) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	return nil, fmt.Errorf("LLM not configured: set LLM_PROVIDER and LLM_API_KEY environment variables")
}
