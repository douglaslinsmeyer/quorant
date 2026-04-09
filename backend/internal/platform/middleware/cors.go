package middleware

import (
	"net/http"
	"strings"
)

const (
	allowedMethods = "GET, POST, PATCH, PUT, DELETE, OPTIONS"
	allowedHeaders = "Content-Type, Authorization, X-Request-ID"
)

// CORS is middleware that handles Cross-Origin Resource Sharing headers.
// If the request Origin matches any entry in allowedOrigins (or if allowedOrigins
// contains "*"), the appropriate CORS headers are set on the response.
// OPTIONS preflight requests are answered with 204 No Content.
func CORS(allowedOrigins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" && isOriginAllowed(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed reports whether origin is permitted by the allowedOrigins list.
// The wildcard "*" permits any origin.
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}
