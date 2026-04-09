package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const complianceCheckCols = "id, org_id, rule_id, status, details, checked_at, resolved_at, resolution_notes"

// PostgresComplianceCheckRepository implements ComplianceCheckRepository using pgxpool.
type PostgresComplianceCheckRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresComplianceCheckRepository constructs a PostgresComplianceCheckRepository backed by pool.
func NewPostgresComplianceCheckRepository(pool *pgxpool.Pool) *PostgresComplianceCheckRepository {
	return &PostgresComplianceCheckRepository{pool: pool}
}

// Create inserts a new compliance check and returns the persisted record.
func (r *PostgresComplianceCheckRepository) Create(ctx context.Context, check *ComplianceCheck) (*ComplianceCheck, error) {
	const q = `
		INSERT INTO compliance_checks (
			org_id, rule_id, status, details
		) VALUES (
			$1, $2, $3, $4
		)
		RETURNING ` + complianceCheckCols

	row := r.pool.QueryRow(ctx, q,
		check.OrgID,
		check.RuleID,
		check.Status,
		marshalRawOrNull(check.Details),
	)
	result, err := scanComplianceCheck(row)
	if err != nil {
		return nil, fmt.Errorf("ai: Create: %w", err)
	}
	return result, nil
}

// ListByOrg returns a cursor-paginated list of compliance checks for the given org.
// afterID is the last ID from the previous page (exclusive). hasMore is true if there are more rows.
func (r *PostgresComplianceCheckRepository) ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]ComplianceCheck, bool, error) {
	const q = `
		SELECT ` + complianceCheckCols + `
		FROM compliance_checks
		WHERE org_id = $1
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, orgID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("ai: ListByOrg: %w", err)
	}
	defer rows.Close()

	results, err := collectComplianceChecks(rows, "ListByOrg")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	return results, hasMore, nil
}

// GetLatestByOrgAndRule returns the most recent compliance check for the given org and rule,
// or nil if none exists.
func (r *PostgresComplianceCheckRepository) GetLatestByOrgAndRule(ctx context.Context, orgID, ruleID uuid.UUID) (*ComplianceCheck, error) {
	const q = `
		SELECT ` + complianceCheckCols + `
		FROM compliance_checks
		WHERE org_id = $1
		  AND rule_id = $2
		ORDER BY checked_at DESC
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, orgID, ruleID)
	result, err := scanComplianceCheck(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: GetLatestByOrgAndRule: %w", err)
	}
	return result, nil
}

// Resolve marks a compliance check as resolved with the given notes and returns the updated record.
func (r *PostgresComplianceCheckRepository) Resolve(ctx context.Context, id uuid.UUID, notes string) (*ComplianceCheck, error) {
	const q = `
		UPDATE compliance_checks SET
			resolved_at      = now(),
			resolution_notes = $1
		WHERE id = $2
		RETURNING ` + complianceCheckCols

	row := r.pool.QueryRow(ctx, q, notes, id)
	result, err := scanComplianceCheck(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ai: Resolve: %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("ai: Resolve: %w", err)
	}
	return result, nil
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

func scanComplianceCheck(row pgx.Row) (*ComplianceCheck, error) {
	var check ComplianceCheck
	var detailsRaw []byte
	err := row.Scan(
		&check.ID,
		&check.OrgID,
		&check.RuleID,
		&check.Status,
		&detailsRaw,
		&check.CheckedAt,
		&check.ResolvedAt,
		&check.ResolutionNotes,
	)
	if err != nil {
		return nil, err
	}
	if len(detailsRaw) > 0 {
		check.Details = json.RawMessage(detailsRaw)
	}
	return &check, nil
}

func collectComplianceChecks(rows pgx.Rows, op string) ([]ComplianceCheck, error) {
	results := []ComplianceCheck{}
	for rows.Next() {
		var check ComplianceCheck
		var detailsRaw []byte
		if err := rows.Scan(
			&check.ID,
			&check.OrgID,
			&check.RuleID,
			&check.Status,
			&detailsRaw,
			&check.CheckedAt,
			&check.ResolvedAt,
			&check.ResolutionNotes,
		); err != nil {
			return nil, fmt.Errorf("ai: %s scan: %w", op, err)
		}
		if len(detailsRaw) > 0 {
			check.Details = json.RawMessage(detailsRaw)
		}
		results = append(results, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: %s rows: %w", op, err)
	}
	return results, nil
}
