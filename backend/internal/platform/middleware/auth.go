package middleware

import (
	"net/http"
	"strings"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/auth"
)

// Auth validates JWT Bearer tokens and injects Claims into the request context.
// If the token is missing or invalid, it returns a 401 JSON error response.
func Auth(validator auth.TokenValidator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract Bearer token from Authorization header.
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			api.WriteError(w, api.NewUnauthenticatedError("auth.missing_header", api.P("header", "Authorization")))
			return
		}

		// Must be "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			api.WriteError(w, api.NewUnauthenticatedError("auth.invalid_header"))
			return
		}
		tokenString := parts[1]

		// 2. Validate token.
		claims, err := validator.Validate(r.Context(), tokenString)
		if err != nil {
			api.WriteError(w, api.NewUnauthenticatedError("auth.invalid_token"))
			return
		}

		// 3. Inject claims into context, call next handler.
		ctx := auth.WithClaims(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
