package org

import (
	"context"

	"github.com/google/uuid"
)

// RegistrationRepository manages unit registration types and registrations.
type RegistrationRepository interface {
	// Registration Types
	CreateRegistrationType(ctx context.Context, rt *RegistrationType) (*RegistrationType, error)
	FindRegistrationTypeByID(ctx context.Context, id uuid.UUID) (*RegistrationType, error)
	ListRegistrationTypesByOrg(ctx context.Context, orgID uuid.UUID) ([]RegistrationType, error)
	UpdateRegistrationType(ctx context.Context, rt *RegistrationType) (*RegistrationType, error)

	// Registrations
	CreateRegistration(ctx context.Context, reg *Registration) (*Registration, error)
	FindRegistrationByID(ctx context.Context, id uuid.UUID) (*Registration, error)
	ListRegistrationsByUnit(ctx context.Context, unitID uuid.UUID) ([]Registration, error)
	UpdateRegistration(ctx context.Context, reg *Registration) (*Registration, error)
}
