package com

import (
	"context"

	"github.com/google/uuid"
)

// AnnouncementRepository persists and retrieves announcements.
type AnnouncementRepository interface {
	Create(ctx context.Context, a *Announcement) (*Announcement, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Announcement, error)
	// ListByOrg returns announcements for the org, supporting cursor-based pagination.
	// afterID is the cursor from the previous page; hasMore is true when more items exist.
	ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Announcement, bool, error)
	Update(ctx context.Context, a *Announcement) (*Announcement, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}
