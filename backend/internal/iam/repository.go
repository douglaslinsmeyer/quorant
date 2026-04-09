package iam

import (
	"context"

	"github.com/google/uuid"
)

// UserRepository persists and retrieves users.
type UserRepository interface {
	// FindByIDPUserID looks up a user by their identity provider ID (Zitadel).
	FindByIDPUserID(ctx context.Context, idpUserID string) (*User, error)

	// FindByID looks up a user by their internal UUID.
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)

	// Upsert creates a new user or updates an existing one (matched by idp_user_id).
	// On conflict (idp_user_id), updates email, display_name, and updated_at.
	Upsert(ctx context.Context, user *User) (*User, error)

	// UpdateLastLogin sets the last_login_at timestamp.
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error

	// FindMembershipsByUserID returns all active memberships for a user (with role name joined).
	FindMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]Membership, error)
}
