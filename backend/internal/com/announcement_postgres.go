package com

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAnnouncementRepository implements AnnouncementRepository using pgxpool.
type PostgresAnnouncementRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAnnouncementRepository creates a new PostgresAnnouncementRepository.
func NewPostgresAnnouncementRepository(pool *pgxpool.Pool) *PostgresAnnouncementRepository {
	return &PostgresAnnouncementRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new announcement and returns the fully-populated row.
func (r *PostgresAnnouncementRepository) Create(ctx context.Context, a *Announcement) (*Announcement, error) {
	const q = `
		INSERT INTO announcements (
			org_id, author_id, title, body, is_pinned,
			audience_roles, scheduled_for, published_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, org_id, author_id, title, body, is_pinned,
		          audience_roles, scheduled_for, published_at,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		a.OrgID,
		a.AuthorID,
		a.Title,
		a.Body,
		a.IsPinned,
		coerceStringSlice(a.AudienceRoles),
		a.ScheduledFor,
		a.PublishedAt,
	)
	result, err := scanAnnouncement(row)
	if err != nil {
		return nil, fmt.Errorf("com: announcement Create: %w", err)
	}
	return result, nil
}

// ─── FindByID ────────────────────────────────────────────────────────────────

// FindByID returns the announcement with the given ID, or nil if not found or soft-deleted.
func (r *PostgresAnnouncementRepository) FindByID(ctx context.Context, id uuid.UUID) (*Announcement, error) {
	const q = `
		SELECT id, org_id, author_id, title, body, is_pinned,
		       audience_roles, scheduled_for, published_at,
		       created_at, updated_at, deleted_at
		FROM announcements
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanAnnouncement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("com: announcement FindByID: %w", err)
	}
	return result, nil
}

// ─── ListByOrg ───────────────────────────────────────────────────────────────

// ListByOrg returns non-deleted announcements for the given org, supporting
// cursor-based pagination ordered by id DESC.
// afterID is the cursor from the previous page; hasMore is true when more items exist.
func (r *PostgresAnnouncementRepository) ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Announcement, bool, error) {
	const q = `
		SELECT id, org_id, author_id, title, body, is_pinned,
		       audience_roles, scheduled_for, published_at,
		       created_at, updated_at, deleted_at
		FROM announcements
		WHERE org_id = $1 AND deleted_at IS NULL
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, orgID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("com: announcement ListByOrg: %w", err)
	}
	defer rows.Close()

	results := []Announcement{}
	for rows.Next() {
		a, err := scanAnnouncementRow(rows)
		if err != nil {
			return nil, false, fmt.Errorf("com: announcement ListByOrg scan: %w", err)
		}
		results = append(results, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("com: announcement ListByOrg rows: %w", err)
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	return results, hasMore, nil
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists changes to an existing announcement and returns the updated row.
func (r *PostgresAnnouncementRepository) Update(ctx context.Context, a *Announcement) (*Announcement, error) {
	const q = `
		UPDATE announcements SET
			title          = $1,
			body           = $2,
			is_pinned      = $3,
			audience_roles = $4,
			scheduled_for  = $5,
			published_at   = $6,
			updated_at     = now()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING id, org_id, author_id, title, body, is_pinned,
		          audience_roles, scheduled_for, published_at,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		a.Title,
		a.Body,
		a.IsPinned,
		coerceStringSlice(a.AudienceRoles),
		a.ScheduledFor,
		a.PublishedAt,
		a.ID,
	)
	result, err := scanAnnouncement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("com: announcement Update: %s not found or already deleted", a.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("com: announcement Update: %w", err)
	}
	return result, nil
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

// SoftDelete marks an announcement as deleted without removing the row.
func (r *PostgresAnnouncementRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE announcements SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("com: announcement SoftDelete: %w", err)
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// coerceStringSlice returns an empty (non-nil) slice when s is nil.
// Postgres NOT NULL TEXT[] columns reject NULL inputs from pgx nil slices.
func coerceStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func scanAnnouncement(row pgx.Row) (*Announcement, error) {
	var a Announcement
	err := row.Scan(
		&a.ID, &a.OrgID, &a.AuthorID, &a.Title, &a.Body, &a.IsPinned,
		&a.AudienceRoles, &a.ScheduledFor, &a.PublishedAt,
		&a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if a.AudienceRoles == nil {
		a.AudienceRoles = []string{}
	}
	return &a, nil
}

func scanAnnouncementRow(rows pgx.Rows) (*Announcement, error) {
	var a Announcement
	err := rows.Scan(
		&a.ID, &a.OrgID, &a.AuthorID, &a.Title, &a.Body, &a.IsPinned,
		&a.AudienceRoles, &a.ScheduledFor, &a.PublishedAt,
		&a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if a.AudienceRoles == nil {
		a.AudienceRoles = []string{}
	}
	return &a, nil
}
