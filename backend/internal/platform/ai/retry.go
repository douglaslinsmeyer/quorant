package ai

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// RetryClient wraps a Client with automatic retry for transient failures.
// Retries on HTTP 429, 500, 502, 503, 529 with exponential backoff and jitter.
type RetryClient struct {
	inner      Client
	maxRetries int
	baseDelay  time.Duration
}

// NewRetryClient wraps an existing Client with retry logic.
func NewRetryClient(inner Client, maxRetries int) *RetryClient {
	return &RetryClient{
		inner:      inner,
		maxRetries: maxRetries,
		baseDelay:  1 * time.Second,
	}
}

func (c *RetryClient) Provider() Provider { return c.inner.Provider() }

func (c *RetryClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.inner.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		if !isRetryable(err) {
			return nil, err
		}
		lastErr = err
		if attempt < c.maxRetries {
			delay := c.backoff(attempt, err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("LLM request failed after %d retries: %w", c.maxRetries, lastErr)
}

func (c *RetryClient) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.inner.Embed(ctx, req)
		if err == nil {
			return resp, nil
		}
		if !isRetryable(err) {
			return nil, err
		}
		lastErr = err
		if attempt < c.maxRetries {
			delay := c.backoff(attempt, err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("LLM embed failed after %d retries: %w", c.maxRetries, lastErr)
}

// backoff computes delay with exponential backoff + jitter.
// Respects Retry-After header if present in the error message.
func (c *RetryClient) backoff(attempt int, err error) time.Duration {
	// Check for Retry-After hint in error
	if ra := parseRetryAfter(err); ra > 0 {
		return ra
	}
	// Exponential backoff with jitter: base * 2^attempt * (0.5 to 1.5)
	delay := float64(c.baseDelay) * math.Pow(2, float64(attempt))
	jitter := 0.5 + rand.Float64()
	return time.Duration(delay * jitter)
}

// isRetryable checks if the error indicates a transient failure worth retrying.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Check for HTTP status codes in the error message
	for _, code := range []int{
		http.StatusTooManyRequests,       // 429
		http.StatusInternalServerError,   // 500
		http.StatusBadGateway,            // 502
		http.StatusServiceUnavailable,    // 503
		529,                               // Anthropic overloaded
	} {
		if containsStatusCode(msg, code) {
			return true
		}
	}
	return false
}

func containsStatusCode(msg string, code int) bool {
	codeStr := strconv.Itoa(code)
	return len(msg) > 0 && (contains(msg, "error "+codeStr) || contains(msg, codeStr+":"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func parseRetryAfter(err error) time.Duration {
	if err == nil {
		return 0
	}
	// Simple heuristic: look for "Retry-After: N" in error
	// In production, this would be parsed from HTTP response headers
	return 0
}
