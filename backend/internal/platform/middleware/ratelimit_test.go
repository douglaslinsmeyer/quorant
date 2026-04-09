package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit_AllowsWithinLimit(t *testing.T) {
	limiter := middleware.NewRateLimiter(10, 10, time.Second)
	handler := middleware.RateLimit(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should be allowed", i)
	}
}

func TestRateLimit_RejectsOverLimit(t *testing.T) {
	limiter := middleware.NewRateLimiter(2, 2, time.Second)
	handler := middleware.RateLimit(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use up the burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Next request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimit_ReplenishesTokens(t *testing.T) {
	limiter := middleware.NewRateLimiter(100, 1, time.Second)
	handler := middleware.RateLimit(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use the single token
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Should be blocked
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)

	// Wait for replenishment
	time.Sleep(50 * time.Millisecond)

	// Should be allowed again
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req)
	assert.Equal(t, http.StatusOK, w3.Code)
}
