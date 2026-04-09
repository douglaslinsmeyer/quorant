package gov

import (
	"context"

	"github.com/google/uuid"
)

// ARBRepository persists and retrieves ARB requests and votes.
type ARBRepository interface {
	Create(ctx context.Context, r *ARBRequest) (*ARBRequest, error)
	FindByID(ctx context.Context, id uuid.UUID) (*ARBRequest, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]ARBRequest, error)
	Update(ctx context.Context, r *ARBRequest) (*ARBRequest, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error

	CreateVote(ctx context.Context, v *ARBVote) (*ARBVote, error)
	ListVotesByRequest(ctx context.Context, requestID uuid.UUID) ([]ARBVote, error)
}
