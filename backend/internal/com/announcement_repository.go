package com

import (
	"context"

	"github.com/google/uuid"
)

// AnnouncementRepository persists and retrieves announcements.
type AnnouncementRepository interface {
	Create(ctx context.Context, a *Announcement) (*Announcement, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Announcement, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Announcement, error)
	Update(ctx context.Context, a *Announcement) (*Announcement, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}
