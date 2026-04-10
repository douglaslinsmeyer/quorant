package policy

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PolicyRecordRepository persists and retrieves policy records.
type PolicyRecordRepository interface {
	CreateRecord(ctx context.Context, r *PolicyRecord) (*PolicyRecord, error)
	FindRecordByID(ctx context.Context, id uuid.UUID) (*PolicyRecord, error)
	GatherForResolution(ctx context.Context, category string, jurisdiction string, orgID uuid.UUID, unitID *uuid.UUID) ([]PolicyRecord, error)
	DeactivateRecord(ctx context.Context, id uuid.UUID) error
	WithTx(tx pgx.Tx) PolicyRecordRepository
}

// ResolutionRepository persists and retrieves policy resolutions.
type ResolutionRepository interface {
	CreateResolution(ctx context.Context, r *ResolutionRecord) (*ResolutionRecord, error)
	FindResolutionByID(ctx context.Context, id uuid.UUID) (*ResolutionRecord, error)
	UpdateReviewStatus(ctx context.Context, id uuid.UUID, status string, reviewedBy *uuid.UUID, notes *string) error
	ListPendingReviews(ctx context.Context) ([]ResolutionRecord, error)
	WithTx(tx pgx.Tx) ResolutionRepository
}
