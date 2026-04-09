package gov

import (
	"context"

	"github.com/google/uuid"
)

// ViolationRepository persists and retrieves violations and their actions.
type ViolationRepository interface {
	Create(ctx context.Context, v *Violation) (*Violation, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Violation, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Violation, error)
	ListByUnit(ctx context.Context, unitID uuid.UUID) ([]Violation, error)
	Update(ctx context.Context, v *Violation) (*Violation, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error

	CreateAction(ctx context.Context, a *ViolationAction) (*ViolationAction, error)
	ListActionsByViolation(ctx context.Context, violationID uuid.UUID) ([]ViolationAction, error)

	// GetOffenseCount returns the number of previous violations for the same unit+category.
	GetOffenseCount(ctx context.Context, unitID uuid.UUID, category string) (int, error)
}
