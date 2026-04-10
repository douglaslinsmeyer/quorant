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

	// ResolveTemplate finds the best-matching template for the given key, channel, and locale.
	// Fallback order:
	//   1. Org-specific template for the requested locale
	//   2. Org-specific template for en_US
	//   3. System default (org_id IS NULL) for the requested locale
	//   4. System default for en_US
	ResolveTemplate(ctx context.Context, orgID uuid.UUID, key, channel, locale string) (*MessageTemplate, error)
}
