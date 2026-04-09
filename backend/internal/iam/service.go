package iam

import (
	"context"
	"fmt"

	"github.com/quorant/quorant/internal/platform/auth"
)

// UserService provides business logic for user operations.
type UserService struct {
	repo UserRepository
}

// NewUserService constructs a UserService backed by the given repository.
func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

// GetOrCreateUser finds a user by their IDP user ID, or creates one if not found.
// Also updates last_login_at. Called during authentication flow.
func (s *UserService) GetOrCreateUser(ctx context.Context, claims *auth.Claims) (*User, error) {
	// 1. Try to find by IDP user ID.
	user, err := s.repo.FindByIDPUserID(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	if user == nil {
		// 2. Not found — create via Upsert.
		user, err = s.repo.Upsert(ctx, &User{
			IDPUserID:   claims.Subject,
			Email:       claims.Email,
			DisplayName: claims.Name,
			IsActive:    true,
		})
		if err != nil {
			return nil, fmt.Errorf("creating user: %w", err)
		}
	}

	// 3. Update last login. Non-critical: ignore any error.
	_ = s.repo.UpdateLastLogin(ctx, user.ID)

	return user, nil
}

// GetCurrentUser extracts claims from the context and returns the user profile
// with memberships.
func (s *UserService) GetCurrentUser(ctx context.Context) (*UserProfile, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no claims in context")
	}

	user, err := s.repo.FindByIDPUserID(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found for idp_user_id: %s", claims.Subject)
	}

	memberships, err := s.repo.FindMembershipsByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("finding memberships: %w", err)
	}

	return &UserProfile{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Phone:       user.Phone,
		AvatarURL:   user.AvatarURL,
		Memberships: memberships,
	}, nil
}

// UpdateProfile updates the current user's profile fields.
func (s *UserService) UpdateProfile(ctx context.Context, req UpdateProfileRequest) (*UserProfile, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no claims in context")
	}

	user, err := s.repo.FindByIDPUserID(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Apply updates.
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.Phone != nil {
		user.Phone = req.Phone
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}

	updated, err := s.repo.Upsert(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}

	memberships, err := s.repo.FindMembershipsByUserID(ctx, updated.ID)
	if err != nil {
		return nil, fmt.Errorf("finding memberships: %w", err)
	}

	return &UserProfile{
		ID:          updated.ID,
		Email:       updated.Email,
		DisplayName: updated.DisplayName,
		Phone:       updated.Phone,
		AvatarURL:   updated.AvatarURL,
		Memberships: memberships,
	}, nil
}
