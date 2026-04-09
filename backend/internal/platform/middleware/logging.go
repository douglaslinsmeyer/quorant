package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code written
// by the downstream handler. If WriteHeader is never called, statusCode
// defaults to 200 (the HTTP spec default).
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *responseWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Logging is middleware that logs each HTTP request after it completes.
// Log level is chosen based on the response status code:
//   - 2xx / 3xx  → INFO
//   - 4xx        → WARN
//   - 5xx        → ERROR
func Logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default if WriteHeader is never called
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Milliseconds()
		requestID := RequestIDFromContext(r.Context())

		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.statusCode),
			slog.Int64("duration_ms", duration),
		}
		if requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}

		switch {
		case rw.statusCode >= 500:
			logger.Error("request completed", attrs...)
		case rw.statusCode >= 400:
			logger.Warn("request completed", attrs...)
		default:
			logger.Info("request completed", attrs...)
		}
	})
}
