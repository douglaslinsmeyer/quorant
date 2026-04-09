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

// OpenAIClient implements Client for OpenAI-compatible APIs.
// This works with OpenAI, Ollama, vLLM, Together AI, LM Studio,
// and any provider that implements the OpenAI chat completions API.
type OpenAIClient struct {
	cfg    Config
	client *http.Client
}

// NewOpenAIClient creates an OpenAI-compatible client.
func NewOpenAIClient(cfg Config) (*OpenAIClient, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}
	if cfg.EmbedModel == "" {
		cfg.EmbedModel = "text-embedding-3-small"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}

	return &OpenAIClient{
		cfg:    cfg,
		client: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (c *OpenAIClient) Provider() Provider { return ProviderOpenAI }

func (c *OpenAIClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.cfg.MaxTokens
	}

	messages := make([]map[string]string, 0, len(req.Messages)+1)
	if req.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}

	body := map[string]any{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": req.Temperature,
	}

	respBody, err := c.doRequest(ctx, "/chat/completions", body)
	if err != nil {
		return nil, fmt.Errorf("openai completion: %w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("openai parse response: %w", err)
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	return &CompletionResponse{
		Content:    content,
		Model:      result.Model,
		InputTokens: result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}, nil
}

func (c *OpenAIClient) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	model := req.Model
	if model == "" {
		model = c.cfg.EmbedModel
	}

	body := map[string]any{
		"model": model,
		"input": req.Input,
	}

	respBody, err := c.doRequest(ctx, "/embeddings", body)
	if err != nil {
		return nil, fmt.Errorf("openai embedding: %w", err)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("openai parse embedding: %w", err)
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai embedding: empty response")
	}

	return &EmbeddingResponse{
		Embedding:  result.Data[0].Embedding,
		Model:      result.Model,
		Dimensions: len(result.Data[0].Embedding),
	}, nil
}

func (c *OpenAIClient) doRequest(ctx context.Context, path string, body any) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
