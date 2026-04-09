package com

import (
	"context"

	"github.com/google/uuid"
)

// DirectoryRepository persists and retrieves HOA directory preferences.
type DirectoryRepository interface {
	Upsert(ctx context.Context, p *DirectoryPreference) (*DirectoryPreference, error)
	FindByUserAndOrg(ctx context.Context, userID, orgID uuid.UUID) (*DirectoryPreference, error)
}
