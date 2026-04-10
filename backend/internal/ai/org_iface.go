package ai

import (
	"context"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/org"
)

// OrgLookup is the subset of org.OrgRepository needed by the AI module.
// Defined here to follow the interface segregation principle — the AI module
// depends on a narrow contract rather than the full OrgRepository.
// org.PostgresOrgRepository satisfies this interface via Go structural typing.
type OrgLookup interface {
	FindByID(ctx context.Context, id uuid.UUID) (*org.Organization, error)
	FindActiveManagement(ctx context.Context, hoaOrgID uuid.UUID) (*org.OrgManagement, error)
	Update(ctx context.Context, o *org.Organization) (*org.Organization, error)
	ListByJurisdiction(ctx context.Context, jurisdiction string) ([]org.Organization, error)
}
