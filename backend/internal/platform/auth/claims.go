package auth

import (
	"context"
)

// Claims holds the relevant JWT claims extracted during authentication.
type Claims struct {
	Subject string // Zitadel user ID (maps to users.idp_user_id)
	Email   string
	Name    string
}

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey struct{}

// WithClaims stores Claims in the context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, claims)
}

// ClaimsFromContext retrieves Claims from the context.
// Returns nil, false if no claims are present.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(contextKey{}).(*Claims)
	return claims, ok
}
