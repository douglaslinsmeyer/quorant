package org

import (
	"context"

	"github.com/google/uuid"
)

// UnitRepository manages units and their related entities.
type UnitRepository interface {
	// Unit CRUD
	CreateUnit(ctx context.Context, unit *Unit) (*Unit, error)
	FindUnitByID(ctx context.Context, id uuid.UUID) (*Unit, error)
	ListUnitsByOrg(ctx context.Context, orgID uuid.UUID) ([]Unit, error)
	UpdateUnit(ctx context.Context, unit *Unit) (*Unit, error)
	SoftDeleteUnit(ctx context.Context, id uuid.UUID) error

	// Property
	GetProperty(ctx context.Context, unitID uuid.UUID) (*Property, error)
	UpsertProperty(ctx context.Context, prop *Property) (*Property, error)

	// Unit Memberships
	CreateUnitMembership(ctx context.Context, m *UnitMembership) (*UnitMembership, error)
	ListUnitMemberships(ctx context.Context, unitID uuid.UUID) ([]UnitMembership, error)
	UpdateUnitMembership(ctx context.Context, m *UnitMembership) (*UnitMembership, error)
	EndUnitMembership(ctx context.Context, id uuid.UUID) error // sets ended_at
}
