// Package middleware provides reusable HTTP middleware for the Quorant API server.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// contextKey is an unexported type for context keys in this package,
// preventing collisions with keys from other packages.
type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID is middleware that ensures every request has a unique identifier.
// It reads the X-Request-ID header if present and propagates it; otherwise it
// generates a new UUID v4. The request ID is stored in the request context and
// set on the response via the X-Request-ID header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext retrieves the request ID stored in ctx by RequestID
// middleware. It returns an empty string if no request ID is present.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
