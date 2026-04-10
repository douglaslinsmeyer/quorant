package com

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresTemplateRepository implements TemplateRepository using pgxpool.
type PostgresTemplateRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresTemplateRepository creates a new PostgresTemplateRepository.
func NewPostgresTemplateRepository(pool *pgxpool.Pool) *PostgresTemplateRepository {
	return &PostgresTemplateRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new message template and returns the fully-populated row.
func (r *PostgresTemplateRepository) Create(ctx context.Context, t *MessageTemplate) (*MessageTemplate, error) {
	const q = `
		INSERT INTO message_templates (org_id, template_key, channel, locale, subject, body, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, org_id, template_key, channel, locale, subject, body, is_active, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		t.OrgID,
		t.TemplateKey,
		t.Channel,
		t.Locale,
		t.Subject,
		t.Body,
		t.IsActive,
	)
	result, err := scanTemplate(row)
	if err != nil {
		return nil, fmt.Errorf("com: template Create: %w", err)
	}
	return result, nil
}

// ─── ListByOrg ───────────────────────────────────────────────────────────────

// ListByOrg returns templates belonging to the given org plus system defaults (org_id IS NULL).
func (r *PostgresTemplateRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]MessageTemplate, error) {
	const q = `
		SELECT id, org_id, template_key, channel, locale, subject, body, is_active, created_at, updated_at
		FROM message_templates
		WHERE org_id = $1 OR org_id IS NULL
		ORDER BY org_id NULLS LAST, template_key, channel`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("com: template ListByOrg: %w", err)
	}
	defer rows.Close()

	results := []MessageTemplate{}
	for rows.Next() {
		t, err := scanTemplateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("com: template ListByOrg scan: %w", err)
		}
		results = append(results, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("com: template ListByOrg rows: %w", err)
	}
	return results, nil
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists changes to an existing template and returns the updated row.
func (r *PostgresTemplateRepository) Update(ctx context.Context, t *MessageTemplate) (*MessageTemplate, error) {
	const q = `
		UPDATE message_templates SET
			template_key = $1,
			channel      = $2,
			locale       = $3,
			subject      = $4,
			body         = $5,
			is_active    = $6,
			updated_at   = now()
		WHERE id = $7
		RETURNING id, org_id, template_key, channel, locale, subject, body, is_active, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		t.TemplateKey,
		t.Channel,
		t.Locale,
		t.Subject,
		t.Body,
		t.IsActive,
		t.ID,
	)
	result, err := scanTemplate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("com: template Update: %s not found", t.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("com: template Update: %w", err)
	}
	return result, nil
}

// ─── Delete ──────────────────────────────────────────────────────────────────

// Delete hard-deletes a message template. Callers fall back to system defaults.
func (r *PostgresTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM message_templates WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("com: template Delete: %w", err)
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func scanTemplate(row pgx.Row) (*MessageTemplate, error) {
	var t MessageTemplate
	err := row.Scan(
		&t.ID, &t.OrgID, &t.TemplateKey, &t.Channel, &t.Locale,
		&t.Subject, &t.Body, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanTemplateRow(rows pgx.Rows) (*MessageTemplate, error) {
	var t MessageTemplate
	err := rows.Scan(
		&t.ID, &t.OrgID, &t.TemplateKey, &t.Channel, &t.Locale,
		&t.Subject, &t.Body, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
