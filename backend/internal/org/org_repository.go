package org

import (
	"context"

	"github.com/google/uuid"
)

// OrgRepository persists and retrieves organizations.
type OrgRepository interface {
	Create(ctx context.Context, org *Organization) (*Organization, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	FindBySlug(ctx context.Context, slug string) (*Organization, error)
	// ListByUserAccess returns orgs the given user has memberships in.
	// afterID is an optional cursor (ID of the last item from the previous page).
	// hasMore is true when additional items exist beyond the returned page.
	ListByUserAccess(ctx context.Context, userID uuid.UUID, limit int, afterID *uuid.UUID) ([]Organization, bool, error)
	Update(ctx context.Context, org *Organization) (*Organization, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	// ListChildren returns direct children of an org (same type).
	ListChildren(ctx context.Context, parentID uuid.UUID) ([]Organization, error)

	// Management relationship operations
	ConnectManagement(ctx context.Context, firmOrgID, hoaOrgID uuid.UUID) (*OrgManagement, error)
	DisconnectManagement(ctx context.Context, hoaOrgID uuid.UUID) error
	ListManagementHistory(ctx context.Context, hoaOrgID uuid.UUID) ([]OrgManagement, error)
	// FindActiveManagement returns the active management firm for an HOA (nil if self-managed).
	FindActiveManagement(ctx context.Context, hoaOrgID uuid.UUID) (*OrgManagement, error)
}
