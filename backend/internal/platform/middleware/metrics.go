package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/quorant/quorant/internal/platform/telemetry"
)

// Metrics records Prometheus metrics for each HTTP request using the RED method
// (Rate, Errors, Duration). It wraps the response writer to capture the status
// code written by downstream handlers.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		telemetry.HTTPRequestsInFlight.Inc()
		defer telemetry.HTTPRequestsInFlight.Dec()

		start := time.Now()

		// Reuse responseWriter from logging.go for status capture.
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)

		// Use the route pattern if available (Go 1.22+), otherwise the raw path.
		// r.Pattern returns the matched mux pattern (e.g. "GET /api/v1/orgs/{id}")
		// which prevents high-cardinality labels from concrete path parameter values.
		path := r.URL.Path
		if pattern := r.Pattern; pattern != "" {
			path = pattern
		}

		telemetry.HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		telemetry.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}
