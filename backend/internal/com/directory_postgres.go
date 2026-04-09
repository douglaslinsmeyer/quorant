package com

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDirectoryRepository implements DirectoryRepository using pgxpool.
type PostgresDirectoryRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresDirectoryRepository creates a new PostgresDirectoryRepository.
func NewPostgresDirectoryRepository(pool *pgxpool.Pool) *PostgresDirectoryRepository {
	return &PostgresDirectoryRepository{pool: pool}
}

// ─── Upsert ──────────────────────────────────────────────────────────────────

// Upsert inserts or updates a user's directory preference for an org.
func (r *PostgresDirectoryRepository) Upsert(ctx context.Context, p *DirectoryPreference) (*DirectoryPreference, error) {
	const q = `
		INSERT INTO directory_preferences (user_id, org_id, opt_in, show_email, show_phone, show_unit)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, org_id)
		DO UPDATE SET
			opt_in     = EXCLUDED.opt_in,
			show_email = EXCLUDED.show_email,
			show_phone = EXCLUDED.show_phone,
			show_unit  = EXCLUDED.show_unit,
			updated_at = now()
		RETURNING id, user_id, org_id, opt_in, show_email, show_phone, show_unit, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		p.UserID,
		p.OrgID,
		p.OptIn,
		p.ShowEmail,
		p.ShowPhone,
		p.ShowUnit,
	)
	result, err := scanDirectoryPreference(row)
	if err != nil {
		return nil, fmt.Errorf("com: directory Upsert: %w", err)
	}
	return result, nil
}

// ─── FindByUserAndOrg ────────────────────────────────────────────────────────

// FindByUserAndOrg returns the directory preference for a user in an org, or nil if not found.
func (r *PostgresDirectoryRepository) FindByUserAndOrg(ctx context.Context, userID, orgID uuid.UUID) (*DirectoryPreference, error) {
	const q = `
		SELECT id, user_id, org_id, opt_in, show_email, show_phone, show_unit, created_at, updated_at
		FROM directory_preferences
		WHERE user_id = $1 AND org_id = $2`

	row := r.pool.QueryRow(ctx, q, userID, orgID)
	result, err := scanDirectoryPreference(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("com: directory FindByUserAndOrg: %w", err)
	}
	return result, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func scanDirectoryPreference(row pgx.Row) (*DirectoryPreference, error) {
	var p DirectoryPreference
	err := row.Scan(
		&p.ID, &p.UserID, &p.OrgID,
		&p.OptIn, &p.ShowEmail, &p.ShowPhone, &p.ShowUnit,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
