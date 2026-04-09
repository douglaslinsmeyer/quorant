package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"sync"
	"time"

	"github.com/quorant/quorant/internal/platform/api"
)

// RateLimiter implements a simple in-memory per-user token bucket rate limiter.
// For production, this should be backed by Redis for cross-instance consistency.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // tokens per interval
	burst   int           // max tokens
	interval time.Duration
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

// NewRateLimiter creates a rate limiter with the given rate (requests per interval).
func NewRateLimiter(rate int, burst int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		burst:    burst,
		interval: interval,
	}
}

// RateLimit middleware enforces per-user rate limiting.
// Users are identified by the resolved user ID from context, falling back to IP.
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Identify the client
			key := r.RemoteAddr
			if userID := UserIDFromContext(r.Context()); userID != (uuid.UUID{}) {
				key = userID.String()
			}

			if !limiter.Allow(key) {
				api.WriteError(w, api.NewRateLimitedError("rate limit exceeded"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Allow checks if the given key has tokens available.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]
	if !exists {
		rl.buckets[key] = &bucket{
			tokens:    float64(rl.burst) - 1,
			lastCheck: now,
		}
		return true
	}

	// Replenish tokens
	elapsed := now.Sub(b.lastCheck)
	b.lastCheck = now
	b.tokens += elapsed.Seconds() / rl.interval.Seconds() * float64(rl.rate)
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}
