package org

import (
	"context"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/iam"
)

// MembershipRepository manages org memberships.
type MembershipRepository interface {
	Create(ctx context.Context, m *iam.Membership) (*iam.Membership, error)
	FindByID(ctx context.Context, id uuid.UUID) (*iam.Membership, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]iam.Membership, error)
	Update(ctx context.Context, m *iam.Membership) (*iam.Membership, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}
