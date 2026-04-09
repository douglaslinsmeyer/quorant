package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/quorant/quorant/internal/platform/api"
)

// Recovery is middleware that catches panics from downstream handlers, logs the
// panic value and stack trace, and returns a 500 Internal Server Error response
// using the standard api error envelope.
func Recovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				logger.Error("panic recovered",
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(stack),
					"method", r.Method,
					"path", r.URL.Path,
				)
				api.WriteError(w, api.NewInternalError(fmt.Errorf("%v", rec)))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
