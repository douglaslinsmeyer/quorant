package gov

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresViolationRepository implements ViolationRepository using a pgxpool.
type PostgresViolationRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresViolationRepository creates a new PostgresViolationRepository backed by pool.
func NewPostgresViolationRepository(pool *pgxpool.Pool) *PostgresViolationRepository {
	return &PostgresViolationRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new violation and returns the fully-populated row.
func (r *PostgresViolationRepository) Create(ctx context.Context, v *Violation) (*Violation, error) {
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}
	if v.EvidenceDocIDs == nil {
		v.EvidenceDocIDs = []uuid.UUID{}
	}

	metadataJSON, err := json.Marshal(v.Metadata)
	if err != nil {
		return nil, fmt.Errorf("violation: Create marshal metadata: %w", err)
	}

	evidenceIDs := uuidSliceToStrings(v.EvidenceDocIDs)

	const q = `
		INSERT INTO violations (
			org_id, unit_id, reported_by, assigned_to,
			title, description, category, status, severity,
			due_date, governing_doc_id, governing_section,
			offense_number, cure_deadline, cure_verified_at, cure_verified_by,
			fine_total_cents, resolved_at, evidence_doc_ids, metadata
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19::uuid[], $20
		)
		RETURNING id, org_id, unit_id, reported_by, assigned_to,
		          title, description, category, status, severity,
		          due_date, governing_doc_id, governing_section,
		          offense_number, cure_deadline, cure_verified_at, cure_verified_by,
		          fine_total_cents, resolved_at, evidence_doc_ids, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		v.OrgID,
		v.UnitID,
		v.ReportedBy,
		v.AssignedTo,
		v.Title,
		v.Description,
		v.Category,
		v.Status,
		v.Severity,
		v.DueDate,
		v.GoverningDocID,
		v.GoverningSection,
		v.OffenseNumber,
		v.CureDeadline,
		v.CureVerifiedAt,
		v.CureVerifiedBy,
		v.FineTotalCents,
		v.ResolvedAt,
		evidenceIDs,
		metadataJSON,
	)

	result, err := scanViolation(row)
	if err != nil {
		return nil, fmt.Errorf("violation: Create: %w", err)
	}
	return result, nil
}

// ─── FindByID ────────────────────────────────────────────────────────────────

// FindByID returns the violation with the given ID, or nil,nil if not found or soft-deleted.
func (r *PostgresViolationRepository) FindByID(ctx context.Context, id uuid.UUID) (*Violation, error) {
	const q = `
		SELECT id, org_id, unit_id, reported_by, assigned_to,
		       title, description, category, status, severity,
		       due_date, governing_doc_id, governing_section,
		       offense_number, cure_deadline, cure_verified_at, cure_verified_by,
		       fine_total_cents, resolved_at, evidence_doc_ids, metadata,
		       created_at, updated_at, deleted_at
		FROM violations
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanViolation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("violation: FindByID: %w", err)
	}
	return result, nil
}

// ─── ListByOrg ───────────────────────────────────────────────────────────────

// ListByOrg returns non-deleted violations for the given org, supporting cursor-based
// pagination ordered by id DESC. afterID is the cursor from the previous page.
func (r *PostgresViolationRepository) ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Violation, bool, error) {
	const q = `
		SELECT id, org_id, unit_id, reported_by, assigned_to,
		       title, description, category, status, severity,
		       due_date, governing_doc_id, governing_section,
		       offense_number, cure_deadline, cure_verified_at, cure_verified_by,
		       fine_total_cents, resolved_at, evidence_doc_ids, metadata,
		       created_at, updated_at, deleted_at
		FROM violations
		WHERE org_id = $1 AND deleted_at IS NULL
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, orgID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("violation: ListByOrg: %w", err)
	}
	defer rows.Close()

	violations, err := collectViolations(rows, "ListByOrg")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(violations) > limit
	if hasMore {
		violations = violations[:limit]
	}
	return violations, hasMore, nil
}

// ─── ListByUnit ──────────────────────────────────────────────────────────────

// ListByUnit returns all non-deleted violations for the given unit, ordered by created_at DESC.
func (r *PostgresViolationRepository) ListByUnit(ctx context.Context, unitID uuid.UUID) ([]Violation, error) {
	const q = `
		SELECT id, org_id, unit_id, reported_by, assigned_to,
		       title, description, category, status, severity,
		       due_date, governing_doc_id, governing_section,
		       offense_number, cure_deadline, cure_verified_at, cure_verified_by,
		       fine_total_cents, resolved_at, evidence_doc_ids, metadata,
		       created_at, updated_at, deleted_at
		FROM violations
		WHERE unit_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, fmt.Errorf("violation: ListByUnit: %w", err)
	}
	defer rows.Close()

	return collectViolations(rows, "ListByUnit")
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists changes to an existing violation and returns the updated row.
func (r *PostgresViolationRepository) Update(ctx context.Context, v *Violation) (*Violation, error) {
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}
	if v.EvidenceDocIDs == nil {
		v.EvidenceDocIDs = []uuid.UUID{}
	}

	metadataJSON, err := json.Marshal(v.Metadata)
	if err != nil {
		return nil, fmt.Errorf("violation: Update marshal metadata: %w", err)
	}

	evidenceIDs := uuidSliceToStrings(v.EvidenceDocIDs)

	const q = `
		UPDATE violations SET
			assigned_to       = $1,
			title             = $2,
			description       = $3,
			category          = $4,
			status            = $5,
			severity          = $6,
			due_date          = $7,
			governing_doc_id  = $8,
			governing_section = $9,
			offense_number    = $10,
			cure_deadline     = $11,
			cure_verified_at  = $12,
			cure_verified_by  = $13,
			fine_total_cents  = $14,
			resolved_at       = $15,
			evidence_doc_ids  = $16::uuid[],
			metadata          = $17,
			updated_at        = now()
		WHERE id = $18 AND deleted_at IS NULL
		RETURNING id, org_id, unit_id, reported_by, assigned_to,
		          title, description, category, status, severity,
		          due_date, governing_doc_id, governing_section,
		          offense_number, cure_deadline, cure_verified_at, cure_verified_by,
		          fine_total_cents, resolved_at, evidence_doc_ids, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		v.AssignedTo,
		v.Title,
		v.Description,
		v.Category,
		v.Status,
		v.Severity,
		v.DueDate,
		v.GoverningDocID,
		v.GoverningSection,
		v.OffenseNumber,
		v.CureDeadline,
		v.CureVerifiedAt,
		v.CureVerifiedBy,
		v.FineTotalCents,
		v.ResolvedAt,
		evidenceIDs,
		metadataJSON,
		v.ID,
	)

	result, err := scanViolation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("violation: Update: violation %s not found or already deleted", v.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("violation: Update: %w", err)
	}
	return result, nil
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

// SoftDelete marks a violation as deleted without removing the row.
func (r *PostgresViolationRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE violations SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("violation: SoftDelete: %w", err)
	}
	return nil
}

// ─── CreateAction ────────────────────────────────────────────────────────────

// CreateAction inserts a new violation action and returns the fully-populated row.
func (r *PostgresViolationRepository) CreateAction(ctx context.Context, a *ViolationAction) (*ViolationAction, error) {
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(a.Metadata)
	if err != nil {
		return nil, fmt.Errorf("violation: CreateAction marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO violation_actions (violation_id, actor_id, action_type, notes, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, violation_id, actor_id, action_type, notes, metadata, created_at`

	row := r.pool.QueryRow(ctx, q,
		a.ViolationID,
		a.ActorID,
		a.ActionType,
		a.Notes,
		metadataJSON,
	)

	result, err := scanViolationAction(row)
	if err != nil {
		return nil, fmt.Errorf("violation: CreateAction: %w", err)
	}
	return result, nil
}

// ─── ListActionsByViolation ──────────────────────────────────────────────────

// ListActionsByViolation returns all actions for the given violation, ordered by created_at ASC.
func (r *PostgresViolationRepository) ListActionsByViolation(ctx context.Context, violationID uuid.UUID) ([]ViolationAction, error) {
	const q = `
		SELECT id, violation_id, actor_id, action_type, notes, metadata, created_at
		FROM violation_actions
		WHERE violation_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, violationID)
	if err != nil {
		return nil, fmt.Errorf("violation: ListActionsByViolation: %w", err)
	}
	defer rows.Close()

	return collectViolationActions(rows, "ListActionsByViolation")
}

// ─── GetOffenseCount ─────────────────────────────────────────────────────────

// GetOffenseCount returns the number of violations for the same unit+category (excluding deleted).
func (r *PostgresViolationRepository) GetOffenseCount(ctx context.Context, unitID uuid.UUID, category string) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM violations
		WHERE unit_id = $1 AND category = $2 AND deleted_at IS NULL`

	var count int
	err := r.pool.QueryRow(ctx, q, unitID, category).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("violation: GetOffenseCount: %w", err)
	}
	return count, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// uuidSliceToStrings converts []uuid.UUID to []string for pgx array binding.
func uuidSliceToStrings(ids []uuid.UUID) []string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	return strs
}

// scanViolation reads a single violation row.
func scanViolation(row pgx.Row) (*Violation, error) {
	var v Violation
	var metadataRaw []byte
	var evidenceRaw []string

	err := row.Scan(
		&v.ID,
		&v.OrgID,
		&v.UnitID,
		&v.ReportedBy,
		&v.AssignedTo,
		&v.Title,
		&v.Description,
		&v.Category,
		&v.Status,
		&v.Severity,
		&v.DueDate,
		&v.GoverningDocID,
		&v.GoverningSection,
		&v.OffenseNumber,
		&v.CureDeadline,
		&v.CureVerifiedAt,
		&v.CureVerifiedBy,
		&v.FineTotalCents,
		&v.ResolvedAt,
		&evidenceRaw,
		&metadataRaw,
		&v.CreatedAt,
		&v.UpdatedAt,
		&v.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	v.EvidenceDocIDs = parseUUIDSlice(evidenceRaw)

	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &v.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal violation metadata: %w", err)
		}
	}
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}
	return &v, nil
}

// collectViolations drains pgx.Rows into a slice of Violation values.
func collectViolations(rows pgx.Rows, op string) ([]Violation, error) {
	var violations []Violation
	for rows.Next() {
		var v Violation
		var metadataRaw []byte
		var evidenceRaw []string

		if err := rows.Scan(
			&v.ID,
			&v.OrgID,
			&v.UnitID,
			&v.ReportedBy,
			&v.AssignedTo,
			&v.Title,
			&v.Description,
			&v.Category,
			&v.Status,
			&v.Severity,
			&v.DueDate,
			&v.GoverningDocID,
			&v.GoverningSection,
			&v.OffenseNumber,
			&v.CureDeadline,
			&v.CureVerifiedAt,
			&v.CureVerifiedBy,
			&v.FineTotalCents,
			&v.ResolvedAt,
			&evidenceRaw,
			&metadataRaw,
			&v.CreatedAt,
			&v.UpdatedAt,
			&v.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("violation: %s scan: %w", op, err)
		}

		v.EvidenceDocIDs = parseUUIDSlice(evidenceRaw)

		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &v.Metadata); err != nil {
				return nil, fmt.Errorf("violation: %s unmarshal metadata: %w", op, err)
			}
		}
		if v.Metadata == nil {
			v.Metadata = map[string]any{}
		}

		violations = append(violations, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("violation: %s rows: %w", op, err)
	}
	return violations, nil
}

// scanViolationAction reads a single violation_action row.
func scanViolationAction(row pgx.Row) (*ViolationAction, error) {
	var a ViolationAction
	var metadataRaw []byte

	err := row.Scan(
		&a.ID,
		&a.ViolationID,
		&a.ActorID,
		&a.ActionType,
		&a.Notes,
		&metadataRaw,
		&a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &a.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal action metadata: %w", err)
		}
	}
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}
	return &a, nil
}

// collectViolationActions drains pgx.Rows into a slice of ViolationAction values.
func collectViolationActions(rows pgx.Rows, op string) ([]ViolationAction, error) {
	var actions []ViolationAction
	for rows.Next() {
		var a ViolationAction
		var metadataRaw []byte

		if err := rows.Scan(
			&a.ID,
			&a.ViolationID,
			&a.ActorID,
			&a.ActionType,
			&a.Notes,
			&metadataRaw,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("violation: %s scan: %w", op, err)
		}

		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &a.Metadata); err != nil {
				return nil, fmt.Errorf("violation: %s unmarshal metadata: %w", op, err)
			}
		}
		if a.Metadata == nil {
			a.Metadata = map[string]any{}
		}

		actions = append(actions, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("violation: %s rows: %w", op, err)
	}
	return actions, nil
}

// parseUUIDSlice converts []string from pgx array into []uuid.UUID.
// Invalid entries are silently skipped.
func parseUUIDSlice(strs []string) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		if id, err := uuid.Parse(s); err == nil {
			result = append(result, id)
		}
	}
	return result
}
