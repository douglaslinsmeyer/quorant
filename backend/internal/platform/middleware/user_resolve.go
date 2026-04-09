package middleware

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
)

// UserFinder looks up a user's internal UUID by their identity provider ID.
type UserFinder interface {
	FindByIDPUserID(ctx context.Context, idpUserID string) (userID uuid.UUID, err error)
}

// NewUserIDResolver returns a function that resolves the authenticated user's
// internal UUID from JWT claims stored in the context.
//
// The returned function reads the Subject claim (IDP user ID) and looks up the
// corresponding internal user UUID via the provided UserFinder.
func NewUserIDResolver(finder UserFinder) func(ctx context.Context) (uuid.UUID, error) {
	return func(ctx context.Context) (uuid.UUID, error) {
		claims, ok := auth.ClaimsFromContext(ctx)
		if !ok || claims == nil {
			return uuid.Nil, fmt.Errorf("no claims in context")
		}
		userID, err := finder.FindByIDPUserID(ctx, claims.Subject)
		if err != nil {
			return uuid.Nil, fmt.Errorf("user not found: %w", err)
		}
		return userID, nil
	}
}
