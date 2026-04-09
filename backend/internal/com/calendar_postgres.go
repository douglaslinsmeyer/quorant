package com

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresCalendarRepository implements CalendarRepository using pgxpool.
type PostgresCalendarRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresCalendarRepository creates a new PostgresCalendarRepository.
func NewPostgresCalendarRepository(pool *pgxpool.Pool) *PostgresCalendarRepository {
	return &PostgresCalendarRepository{pool: pool}
}

// ─── CreateEvent ──────────────────────────────────────────────────────────────

// CreateEvent inserts a new calendar event and returns the fully-populated row.
func (r *PostgresCalendarRepository) CreateEvent(ctx context.Context, e *CalendarEvent) (*CalendarEvent, error) {
	const q = `
		INSERT INTO calendar_events (
			org_id, title, description, event_type, location,
			is_virtual, virtual_link, starts_at, ends_at, is_all_day,
			recurrence_rule, audience_roles, rsvp_enabled, rsvp_limit, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, org_id, title, description, event_type, location,
		          is_virtual, virtual_link, starts_at, ends_at, is_all_day,
		          recurrence_rule, audience_roles, rsvp_enabled, rsvp_limit,
		          created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		e.OrgID,
		e.Title,
		e.Description,
		e.EventType,
		e.Location,
		e.IsVirtual,
		e.VirtualLink,
		e.StartsAt,
		e.EndsAt,
		e.IsAllDay,
		e.RecurrenceRule,
		coerceStringSlice(e.AudienceRoles),
		e.RSVPEnabled,
		e.RSVPLimit,
		e.CreatedBy,
	)
	result, err := scanCalendarEvent(row)
	if err != nil {
		return nil, fmt.Errorf("com: calendar CreateEvent: %w", err)
	}
	return result, nil
}

// ─── FindEventByID ────────────────────────────────────────────────────────────

// FindEventByID returns the calendar event with the given ID, or nil if not found or soft-deleted.
func (r *PostgresCalendarRepository) FindEventByID(ctx context.Context, id uuid.UUID) (*CalendarEvent, error) {
	const q = `
		SELECT id, org_id, title, description, event_type, location,
		       is_virtual, virtual_link, starts_at, ends_at, is_all_day,
		       recurrence_rule, audience_roles, rsvp_enabled, rsvp_limit,
		       created_by, created_at, updated_at, deleted_at
		FROM calendar_events
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanCalendarEvent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("com: calendar FindEventByID: %w", err)
	}
	return result, nil
}

// ─── ListEventsByOrg ──────────────────────────────────────────────────────────

// ListEventsByOrg returns all non-deleted events for an org, ordered by starts_at asc.
func (r *PostgresCalendarRepository) ListEventsByOrg(ctx context.Context, orgID uuid.UUID) ([]CalendarEvent, error) {
	const q = `
		SELECT id, org_id, title, description, event_type, location,
		       is_virtual, virtual_link, starts_at, ends_at, is_all_day,
		       recurrence_rule, audience_roles, rsvp_enabled, rsvp_limit,
		       created_by, created_at, updated_at, deleted_at
		FROM calendar_events
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY starts_at ASC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("com: calendar ListEventsByOrg: %w", err)
	}
	defer rows.Close()

	results := []CalendarEvent{}
	for rows.Next() {
		e, err := scanCalendarEventRow(rows)
		if err != nil {
			return nil, fmt.Errorf("com: calendar ListEventsByOrg scan: %w", err)
		}
		results = append(results, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("com: calendar ListEventsByOrg rows: %w", err)
	}
	return results, nil
}

// ─── UpdateEvent ──────────────────────────────────────────────────────────────

// UpdateEvent persists changes to an existing calendar event.
func (r *PostgresCalendarRepository) UpdateEvent(ctx context.Context, e *CalendarEvent) (*CalendarEvent, error) {
	const q = `
		UPDATE calendar_events SET
			title           = $1,
			description     = $2,
			event_type      = $3,
			location        = $4,
			is_virtual      = $5,
			virtual_link    = $6,
			starts_at       = $7,
			ends_at         = $8,
			is_all_day      = $9,
			recurrence_rule = $10,
			audience_roles  = $11,
			rsvp_enabled    = $12,
			rsvp_limit      = $13,
			updated_at      = now()
		WHERE id = $14 AND deleted_at IS NULL
		RETURNING id, org_id, title, description, event_type, location,
		          is_virtual, virtual_link, starts_at, ends_at, is_all_day,
		          recurrence_rule, audience_roles, rsvp_enabled, rsvp_limit,
		          created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		e.Title,
		e.Description,
		e.EventType,
		e.Location,
		e.IsVirtual,
		e.VirtualLink,
		e.StartsAt,
		e.EndsAt,
		e.IsAllDay,
		e.RecurrenceRule,
		coerceStringSlice(e.AudienceRoles),
		e.RSVPEnabled,
		e.RSVPLimit,
		e.ID,
	)
	result, err := scanCalendarEvent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("com: calendar UpdateEvent: %s not found or already deleted", e.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("com: calendar UpdateEvent: %w", err)
	}
	return result, nil
}

// ─── SoftDeleteEvent ──────────────────────────────────────────────────────────

// SoftDeleteEvent marks a calendar event as deleted without removing the row.
func (r *PostgresCalendarRepository) SoftDeleteEvent(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE calendar_events SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("com: calendar SoftDeleteEvent: %w", err)
	}
	return nil
}

// ─── CreateRSVP ──────────────────────────────────────────────────────────────

// CreateRSVP inserts or updates a user's RSVP for an event.
func (r *PostgresCalendarRepository) CreateRSVP(ctx context.Context, rsvp *CalendarEventRSVP) (*CalendarEventRSVP, error) {
	const q = `
		INSERT INTO calendar_event_rsvps (event_id, user_id, status, guest_count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (event_id, user_id)
		DO UPDATE SET status = EXCLUDED.status, guest_count = EXCLUDED.guest_count, updated_at = now()
		RETURNING id, event_id, user_id, status, guest_count, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		rsvp.EventID,
		rsvp.UserID,
		rsvp.Status,
		rsvp.GuestCount,
	)
	result, err := scanRSVP(row)
	if err != nil {
		return nil, fmt.Errorf("com: calendar CreateRSVP: %w", err)
	}
	return result, nil
}

// ─── ListRSVPsByEvent ────────────────────────────────────────────────────────

// ListRSVPsByEvent returns all RSVPs for a calendar event, ordered by created_at.
func (r *PostgresCalendarRepository) ListRSVPsByEvent(ctx context.Context, eventID uuid.UUID) ([]CalendarEventRSVP, error) {
	const q = `
		SELECT id, event_id, user_id, status, guest_count, created_at, updated_at
		FROM calendar_event_rsvps
		WHERE event_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("com: calendar ListRSVPsByEvent: %w", err)
	}
	defer rows.Close()

	results := []CalendarEventRSVP{}
	for rows.Next() {
		var rsvp CalendarEventRSVP
		if err := rows.Scan(
			&rsvp.ID, &rsvp.EventID, &rsvp.UserID, &rsvp.Status, &rsvp.GuestCount,
			&rsvp.CreatedAt, &rsvp.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("com: calendar ListRSVPsByEvent scan: %w", err)
		}
		results = append(results, rsvp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("com: calendar ListRSVPsByEvent rows: %w", err)
	}
	return results, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func scanCalendarEvent(row pgx.Row) (*CalendarEvent, error) {
	var e CalendarEvent
	err := row.Scan(
		&e.ID, &e.OrgID, &e.Title, &e.Description, &e.EventType, &e.Location,
		&e.IsVirtual, &e.VirtualLink, &e.StartsAt, &e.EndsAt, &e.IsAllDay,
		&e.RecurrenceRule, &e.AudienceRoles, &e.RSVPEnabled, &e.RSVPLimit,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if e.AudienceRoles == nil {
		e.AudienceRoles = []string{}
	}
	return &e, nil
}

func scanCalendarEventRow(rows pgx.Rows) (*CalendarEvent, error) {
	var e CalendarEvent
	err := rows.Scan(
		&e.ID, &e.OrgID, &e.Title, &e.Description, &e.EventType, &e.Location,
		&e.IsVirtual, &e.VirtualLink, &e.StartsAt, &e.EndsAt, &e.IsAllDay,
		&e.RecurrenceRule, &e.AudienceRoles, &e.RSVPEnabled, &e.RSVPLimit,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if e.AudienceRoles == nil {
		e.AudienceRoles = []string{}
	}
	return &e, nil
}

func scanRSVP(row pgx.Row) (*CalendarEventRSVP, error) {
	var r CalendarEventRSVP
	err := row.Scan(&r.ID, &r.EventID, &r.UserID, &r.Status, &r.GuestCount, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
