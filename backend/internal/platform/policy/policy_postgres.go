package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	dbpkg "github.com/quorant/quorant/internal/platform/db"
)

// ─── PolicyRecord Repository ──────────────────────────────────────────────────

// PostgresPolicyRecordRepository implements PolicyRecordRepository using a DBTX.
type PostgresPolicyRecordRepository struct {
	db dbpkg.DBTX
}

// NewPostgresPolicyRecordRepository creates a new PostgresPolicyRecordRepository
// backed by pool.
func NewPostgresPolicyRecordRepository(pool *pgxpool.Pool) *PostgresPolicyRecordRepository {
	return &PostgresPolicyRecordRepository{db: pool}
}

// WithTx returns a new PostgresPolicyRecordRepository scoped to the given
// transaction, enabling participation in a caller-managed transaction.
func (r *PostgresPolicyRecordRepository) WithTx(tx pgx.Tx) PolicyRecordRepository {
	return &PostgresPolicyRecordRepository{db: tx}
}

// CreateRecord inserts a new policy record and returns the fully-populated row.
func (r *PostgresPolicyRecordRepository) CreateRecord(ctx context.Context, rec *PolicyRecord) (*PolicyRecord, error) {
	const q = `
		INSERT INTO policy_records (
			scope, jurisdiction, org_id, unit_id,
			category, key, value, priority_hint,
			statute_reference, source_doc_id,
			effective_date, expiration_date,
			is_active, created_by
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10,
			$11, $12,
			$13, $14
		)
		RETURNING id, scope, jurisdiction, org_id, unit_id,
		          category, key, value, priority_hint,
		          statute_reference, source_doc_id,
		          effective_date, expiration_date,
		          is_active, created_by, created_at, updated_at`

	row := r.db.QueryRow(ctx, q,
		rec.Scope,
		rec.Jurisdiction,
		rec.OrgID,
		rec.UnitID,
		rec.Category,
		rec.Key,
		marshalPolicyValue(rec.Value),
		rec.PriorityHint,
		rec.StatuteRef,
		rec.SourceDocID,
		rec.EffectiveDate,
		rec.ExpirationDate,
		rec.IsActive,
		rec.CreatedBy,
	)

	result, err := scanPolicyRecord(row)
	if err != nil {
		return nil, fmt.Errorf("policy: CreateRecord: %w", err)
	}
	return result, nil
}

// FindRecordByID returns the policy record with the given ID, or nil if not found.
func (r *PostgresPolicyRecordRepository) FindRecordByID(ctx context.Context, id uuid.UUID) (*PolicyRecord, error) {
	const q = `
		SELECT id, scope, jurisdiction, org_id, unit_id,
		       category, key, value, priority_hint,
		       statute_reference, source_doc_id,
		       effective_date, expiration_date,
		       is_active, created_by, created_at, updated_at
		FROM policy_records
		WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	result, err := scanPolicyRecord(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("policy: FindRecordByID: %w", err)
	}
	return result, nil
}

// GatherForResolution returns all active, in-effect policy records matching the
// given category, jurisdiction, org, and optionally unit. This is the Tier 1
// deterministic gather step of the policy resolution pipeline.
func (r *PostgresPolicyRecordRepository) GatherForResolution(ctx context.Context, category string, jurisdiction string, orgID uuid.UUID, unitID *uuid.UUID) ([]PolicyRecord, error) {
	const q = `
		SELECT id, scope, jurisdiction, org_id, unit_id,
		       category, key, value, priority_hint,
		       statute_reference, source_doc_id,
		       effective_date, expiration_date,
		       is_active, created_by, created_at, updated_at
		FROM policy_records
		WHERE category = $1
		  AND is_active = true
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		  AND effective_date <= CURRENT_DATE
		  AND (
		      (scope = 'jurisdiction' AND jurisdiction = $2)
		      OR (scope = 'org' AND org_id = $3)
		      OR (scope = 'unit' AND unit_id = $4)
		  )
		ORDER BY scope, priority_hint, effective_date`

	rows, err := r.db.Query(ctx, q, category, jurisdiction, orgID, unitID)
	if err != nil {
		return nil, fmt.Errorf("policy: GatherForResolution: %w", err)
	}
	defer rows.Close()

	return collectPolicyRecords(rows, "GatherForResolution")
}

// DeactivateRecord sets is_active = false for the given policy record.
func (r *PostgresPolicyRecordRepository) DeactivateRecord(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE policy_records
		SET is_active = false, updated_at = now()
		WHERE id = $1`

	_, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("policy: DeactivateRecord: %w", err)
	}
	return nil
}

// ─── Resolution Repository ────────────────────────────────────────────────────

// PostgresResolutionRepository implements ResolutionRepository using a DBTX.
type PostgresResolutionRepository struct {
	db dbpkg.DBTX
}

// NewPostgresResolutionRepository creates a new PostgresResolutionRepository
// backed by pool.
func NewPostgresResolutionRepository(pool *pgxpool.Pool) *PostgresResolutionRepository {
	return &PostgresResolutionRepository{db: pool}
}

// WithTx returns a new PostgresResolutionRepository scoped to the given
// transaction, enabling participation in a caller-managed transaction.
func (r *PostgresResolutionRepository) WithTx(tx pgx.Tx) ResolutionRepository {
	return &PostgresResolutionRepository{db: tx}
}

// CreateResolution inserts a new policy resolution and returns the
// fully-populated row. When review_status is "pending_review" or "ai_unavailable",
// review_sla_deadline is set to now()+24h.
func (r *PostgresResolutionRepository) CreateResolution(ctx context.Context, rec *ResolutionRecord) (*ResolutionRecord, error) {
	const q = `
		INSERT INTO policy_resolutions (
			org_id, unit_id, category, input_policy_ids,
			ruling, reasoning, confidence, model_id,
			parent_resolution_id, review_status,
			review_sla_deadline
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10,
			CASE WHEN $10 IN ('pending_review', 'ai_unavailable') THEN now() + interval '24 hours' ELSE NULL END
		)
		RETURNING id, org_id, unit_id, category, input_policy_ids,
		          ruling, reasoning, confidence, model_id,
		          parent_resolution_id, review_status,
		          review_sla_deadline, reviewed_by, review_notes, reviewed_at,
		          created_at`

	inputIDs := rec.InputPolicyIDs
	if inputIDs == nil {
		inputIDs = []uuid.UUID{}
	}

	row := r.db.QueryRow(ctx, q,
		rec.OrgID,
		rec.UnitID,
		rec.Category,
		inputIDs,
		marshalPolicyValue(rec.Ruling),
		rec.Reasoning,
		rec.Confidence,
		rec.ModelID,
		rec.ParentResolutionID,
		rec.ReviewStatus,
	)

	result, err := scanResolutionRecord(row)
	if err != nil {
		return nil, fmt.Errorf("policy: CreateResolution: %w", err)
	}
	return result, nil
}

// FindResolutionByID returns the resolution with the given ID, or nil if not found.
func (r *PostgresResolutionRepository) FindResolutionByID(ctx context.Context, id uuid.UUID) (*ResolutionRecord, error) {
	const q = `
		SELECT id, org_id, unit_id, category, input_policy_ids,
		       ruling, reasoning, confidence, model_id,
		       parent_resolution_id, review_status,
		       review_sla_deadline, reviewed_by, review_notes, reviewed_at,
		       created_at
		FROM policy_resolutions
		WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	result, err := scanResolutionRecord(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("policy: FindResolutionByID: %w", err)
	}
	return result, nil
}

// UpdateReviewStatus updates the review lifecycle fields for the given resolution.
func (r *PostgresResolutionRepository) UpdateReviewStatus(ctx context.Context, id uuid.UUID, status string, reviewedBy *uuid.UUID, notes *string) error {
	const q = `
		UPDATE policy_resolutions
		SET review_status = $2,
		    reviewed_by   = $3,
		    review_notes  = $4,
		    reviewed_at   = CASE WHEN $3 IS NOT NULL THEN now() ELSE reviewed_at END
		WHERE id = $1`

	_, err := r.db.Exec(ctx, q, id, status, reviewedBy, notes)
	if err != nil {
		return fmt.Errorf("policy: UpdateReviewStatus: %w", err)
	}
	return nil
}

// ListPendingReviews returns all resolutions with review_status in
// ('pending_review', 'ai_unavailable'), ordered by review_sla_deadline ASC.
// Returns an empty (non-nil) slice when none exist.
func (r *PostgresResolutionRepository) ListPendingReviews(ctx context.Context) ([]ResolutionRecord, error) {
	const q = `
		SELECT id, org_id, unit_id, category, input_policy_ids,
		       ruling, reasoning, confidence, model_id,
		       parent_resolution_id, review_status,
		       review_sla_deadline, reviewed_by, review_notes, reviewed_at,
		       created_at
		FROM policy_resolutions
		WHERE review_status IN ('pending_review', 'ai_unavailable')
		ORDER BY review_sla_deadline ASC NULLS LAST, created_at ASC`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("policy: ListPendingReviews: %w", err)
	}
	defer rows.Close()

	return collectResolutionRecords(rows, "ListPendingReviews")
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

func scanPolicyRecord(row pgx.Row) (*PolicyRecord, error) {
	var rec PolicyRecord
	var valueRaw []byte
	err := row.Scan(
		&rec.ID,
		&rec.Scope,
		&rec.Jurisdiction,
		&rec.OrgID,
		&rec.UnitID,
		&rec.Category,
		&rec.Key,
		&valueRaw,
		&rec.PriorityHint,
		&rec.StatuteRef,
		&rec.SourceDocID,
		&rec.EffectiveDate,
		&rec.ExpirationDate,
		&rec.IsActive,
		&rec.CreatedBy,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(valueRaw) > 0 {
		rec.Value = json.RawMessage(valueRaw)
	}
	return &rec, nil
}

func collectPolicyRecords(rows pgx.Rows, op string) ([]PolicyRecord, error) {
	results := []PolicyRecord{}
	for rows.Next() {
		var rec PolicyRecord
		var valueRaw []byte
		if err := rows.Scan(
			&rec.ID,
			&rec.Scope,
			&rec.Jurisdiction,
			&rec.OrgID,
			&rec.UnitID,
			&rec.Category,
			&rec.Key,
			&valueRaw,
			&rec.PriorityHint,
			&rec.StatuteRef,
			&rec.SourceDocID,
			&rec.EffectiveDate,
			&rec.ExpirationDate,
			&rec.IsActive,
			&rec.CreatedBy,
			&rec.CreatedAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("policy: %s scan: %w", op, err)
		}
		if len(valueRaw) > 0 {
			rec.Value = json.RawMessage(valueRaw)
		}
		results = append(results, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("policy: %s rows: %w", op, err)
	}
	return results, nil
}

func scanResolutionRecord(row pgx.Row) (*ResolutionRecord, error) {
	var rec ResolutionRecord
	var rulingRaw []byte
	err := row.Scan(
		&rec.ID,
		&rec.OrgID,
		&rec.UnitID,
		&rec.Category,
		&rec.InputPolicyIDs,
		&rulingRaw,
		&rec.Reasoning,
		&rec.Confidence,
		&rec.ModelID,
		&rec.ParentResolutionID,
		&rec.ReviewStatus,
		&rec.ReviewSLADeadline,
		&rec.ReviewedBy,
		&rec.ReviewNotes,
		&rec.ReviewedAt,
		&rec.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if rec.InputPolicyIDs == nil {
		rec.InputPolicyIDs = []uuid.UUID{}
	}
	if len(rulingRaw) > 0 {
		rec.Ruling = json.RawMessage(rulingRaw)
	}
	return &rec, nil
}

func collectResolutionRecords(rows pgx.Rows, op string) ([]ResolutionRecord, error) {
	results := []ResolutionRecord{}
	for rows.Next() {
		var rec ResolutionRecord
		var rulingRaw []byte
		if err := rows.Scan(
			&rec.ID,
			&rec.OrgID,
			&rec.UnitID,
			&rec.Category,
			&rec.InputPolicyIDs,
			&rulingRaw,
			&rec.Reasoning,
			&rec.Confidence,
			&rec.ModelID,
			&rec.ParentResolutionID,
			&rec.ReviewStatus,
			&rec.ReviewSLADeadline,
			&rec.ReviewedBy,
			&rec.ReviewNotes,
			&rec.ReviewedAt,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("policy: %s scan: %w", op, err)
		}
		if rec.InputPolicyIDs == nil {
			rec.InputPolicyIDs = []uuid.UUID{}
		}
		if len(rulingRaw) > 0 {
			rec.Ruling = json.RawMessage(rulingRaw)
		}
		results = append(results, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("policy: %s rows: %w", op, err)
	}
	return results, nil
}

// ─── Marshal helpers ──────────────────────────────────────────────────────────

// marshalPolicyValue converts a json.RawMessage to bytes for JSONB columns,
// returning '{}' when the message is empty.
func marshalPolicyValue(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return []byte(raw)
}

