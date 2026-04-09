package com

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresNotificationRepository implements NotificationRepository using pgxpool.
type PostgresNotificationRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresNotificationRepository creates a new PostgresNotificationRepository.
func NewPostgresNotificationRepository(pool *pgxpool.Pool) *PostgresNotificationRepository {
	return &PostgresNotificationRepository{pool: pool}
}

// ─── UpsertPreference ─────────────────────────────────────────────────────────

// UpsertPreference inserts or updates a notification preference based on the unique
// (user_id, org_id, channel, event_type) constraint.
func (r *PostgresNotificationRepository) UpsertPreference(ctx context.Context, p *NotificationPreference) (*NotificationPreference, error) {
	const q = `
		INSERT INTO notification_preferences (user_id, org_id, channel, event_type, enabled)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, org_id, channel, event_type)
		DO UPDATE SET enabled = EXCLUDED.enabled
		RETURNING id, user_id, org_id, channel, event_type, enabled`

	row := r.pool.QueryRow(ctx, q,
		p.UserID,
		p.OrgID,
		p.Channel,
		p.EventType,
		p.Enabled,
	)
	result, err := scanNotificationPreference(row)
	if err != nil {
		return nil, fmt.Errorf("com: notification UpsertPreference: %w", err)
	}
	return result, nil
}

// ─── ListPreferencesByUser ────────────────────────────────────────────────────

// ListPreferencesByUser returns all notification preferences for a user in an org.
func (r *PostgresNotificationRepository) ListPreferencesByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]NotificationPreference, error) {
	const q = `
		SELECT id, user_id, org_id, channel, event_type, enabled
		FROM notification_preferences
		WHERE user_id = $1 AND org_id = $2
		ORDER BY channel, event_type`

	rows, err := r.pool.Query(ctx, q, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("com: notification ListPreferencesByUser: %w", err)
	}
	defer rows.Close()

	results := []NotificationPreference{}
	for rows.Next() {
		var p NotificationPreference
		if err := rows.Scan(&p.ID, &p.UserID, &p.OrgID, &p.Channel, &p.EventType, &p.Enabled); err != nil {
			return nil, fmt.Errorf("com: notification ListPreferencesByUser scan: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("com: notification ListPreferencesByUser rows: %w", err)
	}
	return results, nil
}

// ─── CreatePushToken ──────────────────────────────────────────────────────────

// CreatePushToken registers a new device push token.
func (r *PostgresNotificationRepository) CreatePushToken(ctx context.Context, t *PushToken) (*PushToken, error) {
	const q = `
		INSERT INTO push_tokens (user_id, token, platform, device_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, token, platform, device_name, created_at, last_used_at`

	row := r.pool.QueryRow(ctx, q,
		t.UserID,
		t.Token,
		t.Platform,
		t.DeviceName,
	)
	result, err := scanPushToken(row)
	if err != nil {
		return nil, fmt.Errorf("com: notification CreatePushToken: %w", err)
	}
	return result, nil
}

// ─── DeletePushToken ──────────────────────────────────────────────────────────

// DeletePushToken removes a push token by ID (hard delete).
func (r *PostgresNotificationRepository) DeletePushToken(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM push_tokens WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("com: notification DeletePushToken: %w", err)
	}
	return nil
}

// ─── ListPushTokensByUser ─────────────────────────────────────────────────────

// ListPushTokensByUser returns all push tokens for a user.
func (r *PostgresNotificationRepository) ListPushTokensByUser(ctx context.Context, userID uuid.UUID) ([]PushToken, error) {
	const q = `
		SELECT id, user_id, token, platform, device_name, created_at, last_used_at
		FROM push_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("com: notification ListPushTokensByUser: %w", err)
	}
	defer rows.Close()

	results := []PushToken{}
	for rows.Next() {
		t, err := scanPushTokenRow(rows)
		if err != nil {
			return nil, fmt.Errorf("com: notification ListPushTokensByUser scan: %w", err)
		}
		results = append(results, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("com: notification ListPushTokensByUser rows: %w", err)
	}
	return results, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func scanNotificationPreference(row pgx.Row) (*NotificationPreference, error) {
	var p NotificationPreference
	err := row.Scan(&p.ID, &p.UserID, &p.OrgID, &p.Channel, &p.EventType, &p.Enabled)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanPushToken(row pgx.Row) (*PushToken, error) {
	var t PushToken
	err := row.Scan(&t.ID, &t.UserID, &t.Token, &t.Platform, &t.DeviceName, &t.CreatedAt, &t.LastUsedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanPushTokenRow(rows pgx.Rows) (*PushToken, error) {
	var t PushToken
	err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.Platform, &t.DeviceName, &t.CreatedAt, &t.LastUsedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
