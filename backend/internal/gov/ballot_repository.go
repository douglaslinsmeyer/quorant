package gov

import (
	"context"

	"github.com/google/uuid"
)

// BallotRepository persists and retrieves ballots, votes, and proxy authorizations.
type BallotRepository interface {
	Create(ctx context.Context, b *Ballot) (*Ballot, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Ballot, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Ballot, error)
	Update(ctx context.Context, b *Ballot) (*Ballot, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error

	CastVote(ctx context.Context, v *BallotVote) (*BallotVote, error)
	ListVotesByBallot(ctx context.Context, ballotID uuid.UUID) ([]BallotVote, error)

	FileProxy(ctx context.Context, p *ProxyAuthorization) (*ProxyAuthorization, error)
	RevokeProxy(ctx context.Context, id uuid.UUID) error
	ListProxiesByBallot(ctx context.Context, ballotID uuid.UUID) ([]ProxyAuthorization, error)
}
