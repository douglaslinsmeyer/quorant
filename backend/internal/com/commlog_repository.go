package com

import (
	"context"

	"github.com/google/uuid"
)

// CommLogRepository persists and retrieves communication log entries.
type CommLogRepository interface {
	Create(ctx context.Context, entry *CommunicationLog) (*CommunicationLog, error)
	FindByID(ctx context.Context, id uuid.UUID) (*CommunicationLog, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]CommunicationLog, error)
	ListByUnit(ctx context.Context, unitID uuid.UUID) ([]CommunicationLog, error)
	Update(ctx context.Context, entry *CommunicationLog) (*CommunicationLog, error)
}
