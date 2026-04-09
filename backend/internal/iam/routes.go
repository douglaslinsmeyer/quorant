package iam

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers IAM routes on the mux.
// validator is used by the auth middleware to validate JWT Bearer tokens.
func RegisterRoutes(mux *http.ServeMux, handler *Handler, validator auth.TokenValidator) {
	// authMw wraps a handler with JWT authentication.
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(h))
	}

	// Auth-protected routes.
	mux.Handle("GET /api/v1/auth/me", authMw(handler.GetMe))
	mux.Handle("PATCH /api/v1/auth/me", authMw(handler.UpdateMe))

	// Webhook route — NOT auth-protected (Zitadel calls this directly).
	// TODO: Add HMAC signature verification in a later phase.
	mux.HandleFunc("POST /api/v1/webhooks/zitadel", handler.ZitadelWebhook)
}
