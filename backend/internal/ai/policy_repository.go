package ai

import (
	"context"

	"github.com/google/uuid"
)

// PolicyRepository defines persistence operations for the policy engine.
type PolicyRepository interface {
	// Governing Documents
	CreateGoverningDoc(ctx context.Context, doc *GoverningDocument) (*GoverningDocument, error)
	FindGoverningDocByID(ctx context.Context, id uuid.UUID) (*GoverningDocument, error)
	ListGoverningDocsByOrg(ctx context.Context, orgID uuid.UUID) ([]GoverningDocument, error)
	UpdateGoverningDoc(ctx context.Context, doc *GoverningDocument) (*GoverningDocument, error)

	// Policy Extractions
	CreateExtraction(ctx context.Context, e *PolicyExtraction) (*PolicyExtraction, error)
	FindExtractionByID(ctx context.Context, id uuid.UUID) (*PolicyExtraction, error)
	ListExtractionsByOrg(ctx context.Context, orgID uuid.UUID) ([]PolicyExtraction, error)
	ListActiveExtractionsByOrg(ctx context.Context, orgID uuid.UUID) ([]PolicyExtraction, error)
	FindActiveExtraction(ctx context.Context, orgID uuid.UUID, policyKey string) (*PolicyExtraction, error)
	UpdateExtraction(ctx context.Context, e *PolicyExtraction) (*PolicyExtraction, error)

	// Policy Resolutions
	CreateResolution(ctx context.Context, r *PolicyResolution) (*PolicyResolution, error)
	FindResolutionByID(ctx context.Context, id uuid.UUID) (*PolicyResolution, error)
	ListResolutionsByOrg(ctx context.Context, orgID uuid.UUID) ([]PolicyResolution, error)
	ListPendingEscalations(ctx context.Context, orgID uuid.UUID) ([]PolicyResolution, error)
	UpdateResolution(ctx context.Context, r *PolicyResolution) (*PolicyResolution, error)
}
