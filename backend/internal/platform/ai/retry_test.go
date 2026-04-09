package ai_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	ai "github.com/quorant/quorant/internal/platform/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failNTimesClient struct {
	failCount int32
	current   atomic.Int32
}

func (c *failNTimesClient) Provider() ai.Provider { return "test" }

func (c *failNTimesClient) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	if c.current.Add(1) <= c.failCount {
		return nil, fmt.Errorf("API error 429: rate limited")
	}
	return &ai.CompletionResponse{Content: "success"}, nil
}

func (c *failNTimesClient) Embed(ctx context.Context, req ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	if c.current.Add(1) <= c.failCount {
		return nil, fmt.Errorf("API error 503: service unavailable")
	}
	return &ai.EmbeddingResponse{Embedding: []float32{0.1}, Dimensions: 1}, nil
}

func TestRetryClient_SucceedsOnFirstTry(t *testing.T) {
	inner := &failNTimesClient{failCount: 0}
	client := ai.NewRetryClient(inner, 3)

	resp, err := client.Complete(context.Background(), ai.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "success", resp.Content)
}

func TestRetryClient_RetriesAndSucceeds(t *testing.T) {
	inner := &failNTimesClient{failCount: 2} // fail twice, succeed third
	client := ai.NewRetryClient(inner, 3)

	resp, err := client.Complete(context.Background(), ai.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "success", resp.Content)
}

func TestRetryClient_ExhaustsRetries(t *testing.T) {
	inner := &failNTimesClient{failCount: 10} // always fail
	client := ai.NewRetryClient(inner, 2)

	_, err := client.Complete(context.Background(), ai.CompletionRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 2 retries")
}

func TestRetryClient_DoesNotRetryNonTransient(t *testing.T) {
	calls := atomic.Int32{}
	inner := &countingCompleteClient{
		err:   fmt.Errorf("API error 400: bad request"),
		calls: &calls,
	}
	client := ai.NewRetryClient(inner, 3)

	_, err := client.Complete(context.Background(), ai.CompletionRequest{})
	assert.Error(t, err)
	assert.Equal(t, int32(1), calls.Load()) // only called once, no retry
}

type countingCompleteClient struct {
	err   error
	calls *atomic.Int32
}

func (c *countingCompleteClient) Provider() ai.Provider { return "test" }
func (c *countingCompleteClient) Complete(_ context.Context, _ ai.CompletionRequest) (*ai.CompletionResponse, error) {
	c.calls.Add(1)
	return nil, c.err
}
func (c *countingCompleteClient) Embed(_ context.Context, _ ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	c.calls.Add(1)
	return nil, c.err
}

func TestRetryClient_EmbedRetries(t *testing.T) {
	inner := &failNTimesClient{failCount: 1}
	client := ai.NewRetryClient(inner, 3)

	resp, err := client.Embed(context.Background(), ai.EmbeddingRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Embedding, 1)
}
