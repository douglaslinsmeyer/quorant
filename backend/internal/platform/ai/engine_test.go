package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ai "github.com/quorant/quorant/internal/platform/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_Anthropic(t *testing.T) {
	client, err := ai.NewClient(ai.Config{Provider: ai.ProviderAnthropic, APIKey: "test-key"})
	require.NoError(t, err)
	assert.Equal(t, ai.ProviderAnthropic, client.Provider())
}

func TestNewClient_OpenAI(t *testing.T) {
	client, err := ai.NewClient(ai.Config{Provider: ai.ProviderOpenAI, APIKey: "test-key"})
	require.NoError(t, err)
	assert.Equal(t, ai.ProviderOpenAI, client.Provider())
}

func TestNewClient_Unknown(t *testing.T) {
	_, err := ai.NewClient(ai.Config{Provider: "unknown", APIKey: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown LLM provider")
}

func TestMultiClient_RoutesCorrectly(t *testing.T) {
	completionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Hello from completions"}},
			},
			"model": "test-model",
			"usage": map[string]int{"prompt_tokens": 7, "completion_tokens": 3},
		})
	}))
	defer completionServer.Close()

	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data":  []map[string]any{{"embedding": []float32{0.1, 0.2, 0.3}}},
			"model": "embed-model",
		})
	}))
	defer embeddingServer.Close()

	compClient, _ := ai.NewOpenAIClient(ai.Config{BaseURL: completionServer.URL, Model: "test"})
	embedClient, _ := ai.NewOpenAIClient(ai.Config{BaseURL: embeddingServer.URL, EmbedModel: "embed"})

	multi := ai.NewMultiClient(compClient, embedClient)

	// Completion goes to completion server
	resp, err := multi.Complete(context.Background(), ai.CompletionRequest{
		Messages: []ai.Message{{Role: "user", Content: "Hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello from completions", resp.Content)

	// Embedding goes to embedding server
	embedResp, err := multi.Embed(context.Background(), ai.EmbeddingRequest{Input: "test"})
	require.NoError(t, err)
	assert.Len(t, embedResp.Embedding, 3)
}

func TestStubClient_ReturnsError(t *testing.T) {
	client, err := ai.NewClientFromEnv(ai.Config{Provider: "none"}, nil)
	require.NoError(t, err)
	assert.Equal(t, ai.Provider("none"), client.Provider())

	_, err = client.Complete(context.Background(), ai.CompletionRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM not configured")

	_, err = client.Embed(context.Background(), ai.EmbeddingRequest{})
	assert.Error(t, err)
}

func TestAnthropicClient_EmbedReturnsError(t *testing.T) {
	client, _ := ai.NewAnthropicClient(ai.Config{APIKey: "test"})
	_, err := client.Embed(context.Background(), ai.EmbeddingRequest{Input: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not provide an embedding API")
}

func TestOpenAIClient_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "/chat/completions", r.URL.Path)

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "The answer is 42"}},
			},
			"model": "gpt-4o",
			"usage": map[string]int{"prompt_tokens": 15, "completion_tokens": 10},
		})
	}))
	defer server.Close()

	client, err := ai.NewOpenAIClient(ai.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "gpt-4o",
	})
	require.NoError(t, err)

	resp, err := client.Complete(context.Background(), ai.CompletionRequest{
		Messages: []ai.Message{{Role: "user", Content: "What is the meaning of life?"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "The answer is 42", resp.Content)
	assert.Equal(t, "gpt-4o", resp.Model)
	assert.Equal(t, 25, resp.InputTokens+resp.OutputTokens)
}

func TestOpenAIClient_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)

		json.NewEncoder(w).Encode(map[string]any{
			"data":  []map[string]any{{"embedding": make([]float32, 1536)}},
			"model": "text-embedding-3-small",
		})
	}))
	defer server.Close()

	client, err := ai.NewOpenAIClient(ai.Config{
		BaseURL:    server.URL,
		EmbedModel: "text-embedding-3-small",
	})
	require.NoError(t, err)

	resp, err := client.Embed(context.Background(), ai.EmbeddingRequest{Input: "Hello world"})
	require.NoError(t, err)
	assert.Len(t, resp.Embedding, 1536)
	assert.Equal(t, 1536, resp.Dimensions)
}

func TestAnthropicClient_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		assert.Equal(t, "/v1/messages", r.URL.Path)

		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Claude says hello"},
			},
			"model": "claude-sonnet-4-6",
			"usage": map[string]int{"input_tokens": 20, "output_tokens": 15},
		})
	}))
	defer server.Close()

	client, err := ai.NewAnthropicClient(ai.Config{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Model:   "claude-sonnet-4-6",
	})
	require.NoError(t, err)

	resp, err := client.Complete(context.Background(), ai.CompletionRequest{
		System:   "You are helpful.",
		Messages: []ai.Message{{Role: "user", Content: "Hello"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Claude says hello", resp.Content)
	assert.Equal(t, "claude-sonnet-4-6", resp.Model)
	assert.Equal(t, 20, resp.InputTokens)
	assert.Equal(t, 15, resp.OutputTokens)
}

func TestOpenAIClient_ErrorStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate limit exceeded"}}`))
	}))
	defer server.Close()

	client, _ := ai.NewOpenAIClient(ai.Config{BaseURL: server.URL, APIKey: "key"})
	_, err := client.Complete(context.Background(), ai.CompletionRequest{
		Messages: []ai.Message{{Role: "user", Content: "Hi"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestOpenAIClient_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client, _ := ai.NewOpenAIClient(ai.Config{BaseURL: server.URL, APIKey: "key"})
	_, err := client.Complete(context.Background(), ai.CompletionRequest{
		Messages: []ai.Message{{Role: "user", Content: "Hi"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}
