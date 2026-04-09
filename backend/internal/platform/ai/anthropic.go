package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnthropicClient implements Client for the Anthropic Messages API.
type AnthropicClient struct {
	cfg    Config
	client *http.Client
}

// NewAnthropicClient creates an Anthropic client.
func NewAnthropicClient(cfg Config) (*AnthropicClient, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-6"
	}
	if cfg.EmbedModel == "" {
		// Anthropic doesn't have an embedding API — fall back to OpenAI-compatible
		cfg.EmbedModel = "text-embedding-3-small"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}

	return &AnthropicClient{
		cfg:    cfg,
		client: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (c *AnthropicClient) Provider() Provider { return ProviderAnthropic }

func (c *AnthropicClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.cfg.MaxTokens
	}

	// Convert to Anthropic Messages API format
	messages := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue // system goes in the top-level "system" field
		}
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}

	system := req.System
	if system == "" {
		// Check if any messages have role "system"
		for _, m := range req.Messages {
			if m.Role == "system" {
				system = m.Content
				break
			}
		}
	}

	body := map[string]any{
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
	}
	if system != "" {
		body["system"] = system
	}
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("anthropic parse response: %w", err)
	}

	content := ""
	for _, block := range result.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &CompletionResponse{
		Content:    content,
		Model:      result.Model,
		InputTokens: result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
	}, nil
}

// Embed generates embeddings. Since Anthropic doesn't have a native embedding API,
// this returns an error directing callers to use an OpenAI-compatible embedding provider.
// In practice, the platform should configure a separate embedding client (e.g., OpenAI)
// for embeddings while using Anthropic for completions.
func (c *AnthropicClient) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	return nil, fmt.Errorf("anthropic does not provide an embedding API; configure an OpenAI-compatible provider for embeddings")
}
