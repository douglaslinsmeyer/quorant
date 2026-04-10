package iam

import (
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// User represents an application user profile linked to Zitadel.
type User struct {
	ID          uuid.UUID  `json:"id"`
	IDPUserID   string     `json:"idp_user_id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	Phone       *string    `json:"phone,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// Role represents a system or custom role.
type Role struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
}

// Permission represents a granular permission.
type Permission struct {
	ID          uuid.UUID `json:"id"`
	Key         string    `json:"key"`
	Description *string   `json:"description,omitempty"`
	Module      string    `json:"module"`
}

// Membership represents a user's role within an organization.
type Membership struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	OrgID     uuid.UUID  `json:"org_id"`
	RoleID    uuid.UUID  `json:"role_id"`
	RoleName  string     `json:"role_name"`  // denormalized for convenience
	Status    string     `json:"status"`     // 'active', 'inactive', 'invited'
	InvitedBy *uuid.UUID `json:"invited_by,omitempty"`
	JoinedAt  *time.Time `json:"joined_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// UserProfile is the response shape for GET /api/v1/auth/me.
type UserProfile struct {
	ID          uuid.UUID    `json:"id"`
	Email       string       `json:"email"`
	DisplayName string       `json:"display_name"`
	Phone       *string      `json:"phone,omitempty"`
	AvatarURL   *string      `json:"avatar_url,omitempty"`
	Memberships []Membership `json:"memberships"`
}

// UpdateProfileRequest is the request shape for PATCH /api/v1/auth/me.
type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

// Validate checks that the UpdateProfileRequest has at least one field set.
func (r UpdateProfileRequest) Validate() error {
	if r.DisplayName == nil && r.Phone == nil && r.AvatarURL == nil {
		return api.NewValidationError("validation.at_least_one", "")
	}
	return nil
}
