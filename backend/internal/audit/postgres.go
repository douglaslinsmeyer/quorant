package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAuditor writes audit entries to the audit_log table.
type PostgresAuditor struct {
	pool *pgxpool.Pool
}

func NewPostgresAuditor(pool *pgxpool.Pool) *PostgresAuditor {
	return &PostgresAuditor{pool: pool}
}

// Record inserts an audit entry into audit_log.
// This should ideally be called within the same database transaction as the
// data mutation to ensure atomicity. For Phase 4, we use the pool directly
// (separate transaction). Transaction-scoped auditing will be added when
// we introduce a transaction helper.
func (a *PostgresAuditor) Record(ctx context.Context, entry AuditEntry) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO audit_log (org_id, actor_id, impersonator_id, action, resource_type, resource_id, module, before_state, after_state, metadata, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, entry.OrgID, entry.ActorID, entry.ImpersonatorID, entry.Action, entry.ResourceType, entry.ResourceID, entry.Module, entry.BeforeState, entry.AfterState, entry.Metadata, entry.OccurredAt)
	return err
}

// AuditQuery filters for querying the audit log.
type AuditQuery struct {
	OrgID        *uuid.UUID
	ActorID      *uuid.UUID
	ResourceType *string
	ResourceID   *uuid.UUID
	Action       *string
	Module       *string
	Limit        int
	Offset       int
}

// AuditLogEntry is the read model for audit log entries (includes the auto-generated id and event_id).
type AuditLogEntry struct {
	ID             int64             `json:"id"`
	EventID        uuid.UUID         `json:"event_id"`
	OrgID          uuid.UUID         `json:"org_id"`
	ActorID        uuid.UUID         `json:"actor_id"`
	ImpersonatorID *uuid.UUID        `json:"impersonator_id,omitempty"`
	Action         string            `json:"action"`
	ResourceType   string            `json:"resource_type"`
	ResourceID     uuid.UUID         `json:"resource_id"`
	Module         string            `json:"module"`
	BeforeState    json.RawMessage   `json:"before_state,omitempty"`
	AfterState     json.RawMessage   `json:"after_state,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	OccurredAt     time.Time         `json:"occurred_at"`
}

// Query retrieves audit log entries matching the given filters.
func (a *PostgresAuditor) Query(ctx context.Context, q AuditQuery) ([]AuditLogEntry, error) {
	args := []any{}
	argIdx := 1
	where := ""

	addFilter := func(col string, val any) {
		if where == "" {
			where = "WHERE "
		} else {
			where += " AND "
		}
		where += fmt.Sprintf("%s = $%d", col, argIdx)
		args = append(args, val)
		argIdx++
	}

	if q.OrgID != nil {
		addFilter("org_id", *q.OrgID)
	}
	if q.ActorID != nil {
		addFilter("actor_id", *q.ActorID)
	}
	if q.ResourceType != nil {
		addFilter("resource_type", *q.ResourceType)
	}
	if q.ResourceID != nil {
		addFilter("resource_id", *q.ResourceID)
	}
	if q.Action != nil {
		addFilter("action", *q.Action)
	}
	if q.Module != nil {
		addFilter("module", *q.Module)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`
		SELECT id, event_id, org_id, actor_id, impersonator_id, action, resource_type, resource_id, module, before_state, after_state, metadata, occurred_at
		FROM audit_log
		%s
		ORDER BY occurred_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, q.Offset)

	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("audit query: %w", err)
	}
	defer rows.Close()

	var entries []AuditLogEntry
	for rows.Next() {
		var e AuditLogEntry
		var metadataRaw []byte
		if err := rows.Scan(
			&e.ID,
			&e.EventID,
			&e.OrgID,
			&e.ActorID,
			&e.ImpersonatorID,
			&e.Action,
			&e.ResourceType,
			&e.ResourceID,
			&e.Module,
			&e.BeforeState,
			&e.AfterState,
			&metadataRaw,
			&e.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("audit row scan: %w", err)
		}
		if len(metadataRaw) > 0 && string(metadataRaw) != "null" {
			if err := json.Unmarshal(metadataRaw, &e.Metadata); err != nil {
				return nil, fmt.Errorf("audit metadata unmarshal: %w", err)
			}
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit rows error: %w", err)
	}

	return entries, nil
}
