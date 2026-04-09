package com

import (
	"context"

	"github.com/google/uuid"
)

// TemplateRepository persists and retrieves message templates.
// ListByOrg includes system defaults (org_id IS NULL).
type TemplateRepository interface {
	Create(ctx context.Context, t *MessageTemplate) (*MessageTemplate, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]MessageTemplate, error)
	Update(ctx context.Context, t *MessageTemplate) (*MessageTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error // hard delete — falls back to system default
}
