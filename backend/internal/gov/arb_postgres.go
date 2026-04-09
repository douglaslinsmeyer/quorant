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

// PostgresARBRepository implements ARBRepository using a pgxpool.
type PostgresARBRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresARBRepository creates a new PostgresARBRepository backed by pool.
func NewPostgresARBRepository(pool *pgxpool.Pool) *PostgresARBRepository {
	return &PostgresARBRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new ARB request and returns the fully-populated row.
func (r *PostgresARBRepository) Create(ctx context.Context, req *ARBRequest) (*ARBRequest, error) {
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	if req.SupportingDocIDs == nil {
		req.SupportingDocIDs = []uuid.UUID{}
	}
	if req.Conditions == nil {
		req.Conditions = json.RawMessage("[]")
	}

	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("arb: Create marshal metadata: %w", err)
	}

	supportingIDs := uuidSliceToStrings(req.SupportingDocIDs)

	const q = `
		INSERT INTO arb_requests (
			org_id, unit_id, submitted_by,
			title, description, category, status,
			reviewed_by, decision_notes, decided_at,
			supporting_doc_ids, governing_doc_id, governing_section,
			review_deadline, auto_approved, conditions,
			revision_count, metadata
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			$8, $9, $10,
			$11::uuid[], $12, $13,
			$14, $15, $16,
			$17, $18
		)
		RETURNING id, org_id, unit_id, submitted_by,
		          title, description, category, status,
		          reviewed_by, decision_notes, decided_at,
		          supporting_doc_ids, governing_doc_id, governing_section,
		          review_deadline, auto_approved, conditions,
		          revision_count, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		req.OrgID,
		req.UnitID,
		req.SubmittedBy,
		req.Title,
		req.Description,
		req.Category,
		req.Status,
		req.ReviewedBy,
		req.DecisionNotes,
		req.DecidedAt,
		supportingIDs,
		req.GoverningDocID,
		req.GoverningSection,
		req.ReviewDeadline,
		req.AutoApproved,
		req.Conditions,
		req.RevisionCount,
		metadataJSON,
	)

	result, err := scanARBRequest(row)
	if err != nil {
		return nil, fmt.Errorf("arb: Create: %w", err)
	}
	return result, nil
}

// ─── FindByID ────────────────────────────────────────────────────────────────

// FindByID returns the ARB request with the given ID, or nil,nil if not found or soft-deleted.
func (r *PostgresARBRepository) FindByID(ctx context.Context, id uuid.UUID) (*ARBRequest, error) {
	const q = `
		SELECT id, org_id, unit_id, submitted_by,
		       title, description, category, status,
		       reviewed_by, decision_notes, decided_at,
		       supporting_doc_ids, governing_doc_id, governing_section,
		       review_deadline, auto_approved, conditions,
		       revision_count, metadata,
		       created_at, updated_at, deleted_at
		FROM arb_requests
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanARBRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("arb: FindByID: %w", err)
	}
	return result, nil
}

// ─── ListByOrg ───────────────────────────────────────────────────────────────

// ListByOrg returns all non-deleted ARB requests for the given org, ordered by created_at DESC.
func (r *PostgresARBRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]ARBRequest, error) {
	const q = `
		SELECT id, org_id, unit_id, submitted_by,
		       title, description, category, status,
		       reviewed_by, decision_notes, decided_at,
		       supporting_doc_ids, governing_doc_id, governing_section,
		       review_deadline, auto_approved, conditions,
		       revision_count, metadata,
		       created_at, updated_at, deleted_at
		FROM arb_requests
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("arb: ListByOrg: %w", err)
	}
	defer rows.Close()

	return collectARBRequests(rows, "ListByOrg")
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists changes to an existing ARB request and returns the updated row.
func (r *PostgresARBRepository) Update(ctx context.Context, req *ARBRequest) (*ARBRequest, error) {
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	if req.SupportingDocIDs == nil {
		req.SupportingDocIDs = []uuid.UUID{}
	}
	if req.Conditions == nil {
		req.Conditions = json.RawMessage("[]")
	}

	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("arb: Update marshal metadata: %w", err)
	}

	supportingIDs := uuidSliceToStrings(req.SupportingDocIDs)

	const q = `
		UPDATE arb_requests SET
			title             = $1,
			description       = $2,
			category          = $3,
			status            = $4,
			reviewed_by       = $5,
			decision_notes    = $6,
			decided_at        = $7,
			supporting_doc_ids = $8::uuid[],
			governing_doc_id  = $9,
			governing_section = $10,
			review_deadline   = $11,
			auto_approved     = $12,
			conditions        = $13,
			revision_count    = $14,
			metadata          = $15,
			updated_at        = now()
		WHERE id = $16 AND deleted_at IS NULL
		RETURNING id, org_id, unit_id, submitted_by,
		          title, description, category, status,
		          reviewed_by, decision_notes, decided_at,
		          supporting_doc_ids, governing_doc_id, governing_section,
		          review_deadline, auto_approved, conditions,
		          revision_count, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		req.Title,
		req.Description,
		req.Category,
		req.Status,
		req.ReviewedBy,
		req.DecisionNotes,
		req.DecidedAt,
		supportingIDs,
		req.GoverningDocID,
		req.GoverningSection,
		req.ReviewDeadline,
		req.AutoApproved,
		req.Conditions,
		req.RevisionCount,
		metadataJSON,
		req.ID,
	)

	result, err := scanARBRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("arb: Update: request %s not found or already deleted", req.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("arb: Update: %w", err)
	}
	return result, nil
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

// SoftDelete marks an ARB request as deleted without removing the row.
func (r *PostgresARBRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE arb_requests SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("arb: SoftDelete: %w", err)
	}
	return nil
}

// ─── CreateVote ──────────────────────────────────────────────────────────────

// CreateVote inserts a new ARB vote and returns the fully-populated row.
func (r *PostgresARBRepository) CreateVote(ctx context.Context, v *ARBVote) (*ARBVote, error) {
	const q = `
		INSERT INTO arb_votes (arb_request_id, voter_id, vote, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, arb_request_id, voter_id, vote, notes, created_at`

	row := r.pool.QueryRow(ctx, q,
		v.ARBRequestID,
		v.VoterID,
		v.Vote,
		v.Notes,
	)

	result, err := scanARBVote(row)
	if err != nil {
		return nil, fmt.Errorf("arb: CreateVote: %w", err)
	}
	return result, nil
}

// ─── ListVotesByRequest ──────────────────────────────────────────────────────

// ListVotesByRequest returns all votes for the given ARB request, ordered by created_at ASC.
func (r *PostgresARBRepository) ListVotesByRequest(ctx context.Context, requestID uuid.UUID) ([]ARBVote, error) {
	const q = `
		SELECT id, arb_request_id, voter_id, vote, notes, created_at
		FROM arb_votes
		WHERE arb_request_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, requestID)
	if err != nil {
		return nil, fmt.Errorf("arb: ListVotesByRequest: %w", err)
	}
	defer rows.Close()

	return collectARBVotes(rows, "ListVotesByRequest")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// scanARBRequest reads a single arb_requests row.
func scanARBRequest(row pgx.Row) (*ARBRequest, error) {
	var r ARBRequest
	var metadataRaw []byte
	var conditionsRaw []byte
	var supportingRaw []string

	err := row.Scan(
		&r.ID,
		&r.OrgID,
		&r.UnitID,
		&r.SubmittedBy,
		&r.Title,
		&r.Description,
		&r.Category,
		&r.Status,
		&r.ReviewedBy,
		&r.DecisionNotes,
		&r.DecidedAt,
		&supportingRaw,
		&r.GoverningDocID,
		&r.GoverningSection,
		&r.ReviewDeadline,
		&r.AutoApproved,
		&conditionsRaw,
		&r.RevisionCount,
		&metadataRaw,
		&r.CreatedAt,
		&r.UpdatedAt,
		&r.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	r.SupportingDocIDs = parseUUIDSlice(supportingRaw)

	if len(conditionsRaw) > 0 {
		r.Conditions = json.RawMessage(conditionsRaw)
	} else {
		r.Conditions = json.RawMessage("[]")
	}

	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &r.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal arb metadata: %w", err)
		}
	}
	if r.Metadata == nil {
		r.Metadata = map[string]any{}
	}
	return &r, nil
}

// collectARBRequests drains pgx.Rows into a slice of ARBRequest values.
func collectARBRequests(rows pgx.Rows, op string) ([]ARBRequest, error) {
	var requests []ARBRequest
	for rows.Next() {
		var req ARBRequest
		var metadataRaw []byte
		var conditionsRaw []byte
		var supportingRaw []string

		if err := rows.Scan(
			&req.ID,
			&req.OrgID,
			&req.UnitID,
			&req.SubmittedBy,
			&req.Title,
			&req.Description,
			&req.Category,
			&req.Status,
			&req.ReviewedBy,
			&req.DecisionNotes,
			&req.DecidedAt,
			&supportingRaw,
			&req.GoverningDocID,
			&req.GoverningSection,
			&req.ReviewDeadline,
			&req.AutoApproved,
			&conditionsRaw,
			&req.RevisionCount,
			&metadataRaw,
			&req.CreatedAt,
			&req.UpdatedAt,
			&req.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("arb: %s scan: %w", op, err)
		}

		req.SupportingDocIDs = parseUUIDSlice(supportingRaw)

		if len(conditionsRaw) > 0 {
			req.Conditions = json.RawMessage(conditionsRaw)
		} else {
			req.Conditions = json.RawMessage("[]")
		}

		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &req.Metadata); err != nil {
				return nil, fmt.Errorf("arb: %s unmarshal metadata: %w", op, err)
			}
		}
		if req.Metadata == nil {
			req.Metadata = map[string]any{}
		}

		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("arb: %s rows: %w", op, err)
	}
	return requests, nil
}

// scanARBVote reads a single arb_votes row.
func scanARBVote(row pgx.Row) (*ARBVote, error) {
	var v ARBVote
	err := row.Scan(
		&v.ID,
		&v.ARBRequestID,
		&v.VoterID,
		&v.Vote,
		&v.Notes,
		&v.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// collectARBVotes drains pgx.Rows into a slice of ARBVote values.
func collectARBVotes(rows pgx.Rows, op string) ([]ARBVote, error) {
	var votes []ARBVote
	for rows.Next() {
		var v ARBVote
		if err := rows.Scan(
			&v.ID,
			&v.ARBRequestID,
			&v.VoterID,
			&v.Vote,
			&v.Notes,
			&v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("arb: %s scan: %w", op, err)
		}
		votes = append(votes, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("arb: %s rows: %w", op, err)
	}
	return votes, nil
}
