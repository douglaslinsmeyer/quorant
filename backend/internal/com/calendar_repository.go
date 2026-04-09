package com

import (
	"context"

	"github.com/google/uuid"
)

// CalendarRepository persists and retrieves calendar events and RSVPs.
type CalendarRepository interface {
	CreateEvent(ctx context.Context, e *CalendarEvent) (*CalendarEvent, error)
	FindEventByID(ctx context.Context, id uuid.UUID) (*CalendarEvent, error)
	ListEventsByOrg(ctx context.Context, orgID uuid.UUID) ([]CalendarEvent, error)
	UpdateEvent(ctx context.Context, e *CalendarEvent) (*CalendarEvent, error)
	SoftDeleteEvent(ctx context.Context, id uuid.UUID) error

	CreateRSVP(ctx context.Context, r *CalendarEventRSVP) (*CalendarEventRSVP, error)
	ListRSVPsByEvent(ctx context.Context, eventID uuid.UUID) ([]CalendarEventRSVP, error)
}
