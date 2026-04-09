package com

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresCommLogRepository implements CommLogRepository using pgxpool.
type PostgresCommLogRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresCommLogRepository creates a new PostgresCommLogRepository.
func NewPostgresCommLogRepository(pool *pgxpool.Pool) *PostgresCommLogRepository {
	return &PostgresCommLogRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new communication log entry and returns the fully-populated row.
func (r *PostgresCommLogRepository) Create(ctx context.Context, entry *CommunicationLog) (*CommunicationLog, error) {
	metaJSON, err := marshalMetadata(entry.Metadata)
	if err != nil {
		return nil, fmt.Errorf("com: commlog Create marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO communication_log (
			org_id, direction, channel, contact_user_id, contact_name, contact_info,
			initiated_by, subject, body, template_id, attachment_ids, unit_id,
			resource_type, resource_id, status, sent_at, delivered_at, opened_at,
			bounced_at, bounce_reason, duration_minutes, source, provider_ref, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23, $24
		)
		RETURNING id, org_id, direction, channel, contact_user_id, contact_name, contact_info,
		          initiated_by, subject, body, template_id, attachment_ids, unit_id,
		          resource_type, resource_id, status, sent_at, delivered_at, opened_at,
		          bounced_at, bounce_reason, duration_minutes, source, provider_ref, metadata,
		          created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		entry.OrgID,
		entry.Direction,
		entry.Channel,
		entry.ContactUserID,
		entry.ContactName,
		entry.ContactInfo,
		entry.InitiatedBy,
		entry.Subject,
		entry.Body,
		entry.TemplateID,
		entry.AttachmentIDs,
		entry.UnitID,
		entry.ResourceType,
		entry.ResourceID,
		entry.Status,
		entry.SentAt,
		entry.DeliveredAt,
		entry.OpenedAt,
		entry.BouncedAt,
		entry.BounceReason,
		entry.DurationMinutes,
		entry.Source,
		entry.ProviderRef,
		metaJSON,
	)
	result, err := scanCommLog(row)
	if err != nil {
		return nil, fmt.Errorf("com: commlog Create: %w", err)
	}
	return result, nil
}

// ─── FindByID ────────────────────────────────────────────────────────────────

// FindByID returns the communication log entry with the given ID, or nil if not found.
func (r *PostgresCommLogRepository) FindByID(ctx context.Context, id uuid.UUID) (*CommunicationLog, error) {
	const q = `
		SELECT id, org_id, direction, channel, contact_user_id, contact_name, contact_info,
		       initiated_by, subject, body, template_id, attachment_ids, unit_id,
		       resource_type, resource_id, status, sent_at, delivered_at, opened_at,
		       bounced_at, bounce_reason, duration_minutes, source, provider_ref, metadata,
		       created_at, updated_at
		FROM communication_log
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanCommLog(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("com: commlog FindByID: %w", err)
	}
	return result, nil
}

// ─── ListByOrg ───────────────────────────────────────────────────────────────

// ListByOrg returns all communication log entries for an org, ordered by created_at desc.
func (r *PostgresCommLogRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]CommunicationLog, error) {
	const q = `
		SELECT id, org_id, direction, channel, contact_user_id, contact_name, contact_info,
		       initiated_by, subject, body, template_id, attachment_ids, unit_id,
		       resource_type, resource_id, status, sent_at, delivered_at, opened_at,
		       bounced_at, bounce_reason, duration_minutes, source, provider_ref, metadata,
		       created_at, updated_at
		FROM communication_log
		WHERE org_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("com: commlog ListByOrg: %w", err)
	}
	defer rows.Close()

	return collectCommLogs(rows, "ListByOrg")
}

// ─── ListByUnit ──────────────────────────────────────────────────────────────

// ListByUnit returns all communication log entries for a unit, ordered by created_at desc.
func (r *PostgresCommLogRepository) ListByUnit(ctx context.Context, unitID uuid.UUID) ([]CommunicationLog, error) {
	const q = `
		SELECT id, org_id, direction, channel, contact_user_id, contact_name, contact_info,
		       initiated_by, subject, body, template_id, attachment_ids, unit_id,
		       resource_type, resource_id, status, sent_at, delivered_at, opened_at,
		       bounced_at, bounce_reason, duration_minutes, source, provider_ref, metadata,
		       created_at, updated_at
		FROM communication_log
		WHERE unit_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, fmt.Errorf("com: commlog ListByUnit: %w", err)
	}
	defer rows.Close()

	return collectCommLogs(rows, "ListByUnit")
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists status and delivery timestamp changes to an existing log entry.
func (r *PostgresCommLogRepository) Update(ctx context.Context, entry *CommunicationLog) (*CommunicationLog, error) {
	metaJSON, err := marshalMetadata(entry.Metadata)
	if err != nil {
		return nil, fmt.Errorf("com: commlog Update marshal metadata: %w", err)
	}

	const q = `
		UPDATE communication_log SET
			status           = $1,
			sent_at          = $2,
			delivered_at     = $3,
			opened_at        = $4,
			bounced_at       = $5,
			bounce_reason    = $6,
			duration_minutes = $7,
			provider_ref     = $8,
			metadata         = $9,
			updated_at       = now()
		WHERE id = $10
		RETURNING id, org_id, direction, channel, contact_user_id, contact_name, contact_info,
		          initiated_by, subject, body, template_id, attachment_ids, unit_id,
		          resource_type, resource_id, status, sent_at, delivered_at, opened_at,
		          bounced_at, bounce_reason, duration_minutes, source, provider_ref, metadata,
		          created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		entry.Status,
		entry.SentAt,
		entry.DeliveredAt,
		entry.OpenedAt,
		entry.BouncedAt,
		entry.BounceReason,
		entry.DurationMinutes,
		entry.ProviderRef,
		metaJSON,
		entry.ID,
	)
	result, err := scanCommLog(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("com: commlog Update: %s not found", entry.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("com: commlog Update: %w", err)
	}
	return result, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func marshalMetadata(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func scanCommLog(row pgx.Row) (*CommunicationLog, error) {
	var e CommunicationLog
	var metaRaw []byte
	err := row.Scan(
		&e.ID, &e.OrgID, &e.Direction, &e.Channel,
		&e.ContactUserID, &e.ContactName, &e.ContactInfo,
		&e.InitiatedBy, &e.Subject, &e.Body, &e.TemplateID, &e.AttachmentIDs, &e.UnitID,
		&e.ResourceType, &e.ResourceID, &e.Status,
		&e.SentAt, &e.DeliveredAt, &e.OpenedAt,
		&e.BouncedAt, &e.BounceReason, &e.DurationMinutes,
		&e.Source, &e.ProviderRef, &metaRaw,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &e.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	if e.AttachmentIDs == nil {
		e.AttachmentIDs = []uuid.UUID{}
	}
	return &e, nil
}

func collectCommLogs(rows pgx.Rows, op string) ([]CommunicationLog, error) {
	results := []CommunicationLog{}
	for rows.Next() {
		var e CommunicationLog
		var metaRaw []byte
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.Direction, &e.Channel,
			&e.ContactUserID, &e.ContactName, &e.ContactInfo,
			&e.InitiatedBy, &e.Subject, &e.Body, &e.TemplateID, &e.AttachmentIDs, &e.UnitID,
			&e.ResourceType, &e.ResourceID, &e.Status,
			&e.SentAt, &e.DeliveredAt, &e.OpenedAt,
			&e.BouncedAt, &e.BounceReason, &e.DurationMinutes,
			&e.Source, &e.ProviderRef, &metaRaw,
			&e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("com: commlog %s scan: %w", op, err)
		}
		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &e.Metadata); err != nil {
				return nil, fmt.Errorf("com: commlog %s unmarshal metadata: %w", op, err)
			}
		}
		if e.Metadata == nil {
			e.Metadata = map[string]any{}
		}
		if e.AttachmentIDs == nil {
			e.AttachmentIDs = []uuid.UUID{}
		}
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("com: commlog %s rows: %w", op, err)
	}
	return results, nil
}
