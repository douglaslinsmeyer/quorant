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

// PostgresBallotRepository implements BallotRepository using a pgxpool.
type PostgresBallotRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresBallotRepository creates a new PostgresBallotRepository backed by pool.
func NewPostgresBallotRepository(pool *pgxpool.Pool) *PostgresBallotRepository {
	return &PostgresBallotRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new ballot and returns the fully-populated row.
func (r *PostgresBallotRepository) Create(ctx context.Context, b *Ballot) (*Ballot, error) {
	if b.Options == nil {
		b.Options = json.RawMessage("[]")
	}

	const q = `
		INSERT INTO ballots (
			org_id, title, description, ballot_type, status,
			options, eligible_role, opens_at, closes_at,
			quorum_percent, pass_percent, eligible_units,
			votes_cast, quorum_met, weight_method, results, created_by
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15, $16, $17
		)
		RETURNING id, org_id, title, description, ballot_type, status,
		          options, eligible_role, opens_at, closes_at,
		          quorum_percent, pass_percent, eligible_units,
		          votes_cast, quorum_met, weight_method, results, created_by,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		b.OrgID,
		b.Title,
		b.Description,
		b.BallotType,
		b.Status,
		b.Options,
		b.EligibleRole,
		b.OpensAt,
		b.ClosesAt,
		b.QuorumPercent,
		b.PassPercent,
		b.EligibleUnits,
		b.VotesCast,
		b.QuorumMet,
		b.WeightMethod,
		b.Results,
		b.CreatedBy,
	)

	result, err := scanBallot(row)
	if err != nil {
		return nil, fmt.Errorf("ballot: Create: %w", err)
	}
	return result, nil
}

// ─── FindByID ────────────────────────────────────────────────────────────────

// FindByID returns the ballot with the given ID, or nil,nil if not found or soft-deleted.
func (r *PostgresBallotRepository) FindByID(ctx context.Context, id uuid.UUID) (*Ballot, error) {
	const q = `
		SELECT id, org_id, title, description, ballot_type, status,
		       options, eligible_role, opens_at, closes_at,
		       quorum_percent, pass_percent, eligible_units,
		       votes_cast, quorum_met, weight_method, results, created_by,
		       created_at, updated_at, deleted_at
		FROM ballots
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanBallot(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ballot: FindByID: %w", err)
	}
	return result, nil
}

// ─── ListByOrg ───────────────────────────────────────────────────────────────

// ListByOrg returns all non-deleted ballots for the given org, ordered by created_at DESC.
func (r *PostgresBallotRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Ballot, error) {
	const q = `
		SELECT id, org_id, title, description, ballot_type, status,
		       options, eligible_role, opens_at, closes_at,
		       quorum_percent, pass_percent, eligible_units,
		       votes_cast, quorum_met, weight_method, results, created_by,
		       created_at, updated_at, deleted_at
		FROM ballots
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("ballot: ListByOrg: %w", err)
	}
	defer rows.Close()

	return collectBallots(rows, "ListByOrg")
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists changes to an existing ballot and returns the updated row.
func (r *PostgresBallotRepository) Update(ctx context.Context, b *Ballot) (*Ballot, error) {
	if b.Options == nil {
		b.Options = json.RawMessage("[]")
	}

	const q = `
		UPDATE ballots SET
			title          = $1,
			description    = $2,
			ballot_type    = $3,
			status         = $4,
			options        = $5,
			eligible_role  = $6,
			opens_at       = $7,
			closes_at      = $8,
			quorum_percent = $9,
			pass_percent   = $10,
			eligible_units = $11,
			votes_cast     = $12,
			quorum_met     = $13,
			weight_method  = $14,
			results        = $15,
			updated_at     = now()
		WHERE id = $16 AND deleted_at IS NULL
		RETURNING id, org_id, title, description, ballot_type, status,
		          options, eligible_role, opens_at, closes_at,
		          quorum_percent, pass_percent, eligible_units,
		          votes_cast, quorum_met, weight_method, results, created_by,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		b.Title,
		b.Description,
		b.BallotType,
		b.Status,
		b.Options,
		b.EligibleRole,
		b.OpensAt,
		b.ClosesAt,
		b.QuorumPercent,
		b.PassPercent,
		b.EligibleUnits,
		b.VotesCast,
		b.QuorumMet,
		b.WeightMethod,
		b.Results,
		b.ID,
	)

	result, err := scanBallot(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ballot: Update: ballot %s not found or already deleted", b.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("ballot: Update: %w", err)
	}
	return result, nil
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

// SoftDelete marks a ballot as deleted without removing the row.
func (r *PostgresBallotRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE ballots SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("ballot: SoftDelete: %w", err)
	}
	return nil
}

// ─── CastVote ────────────────────────────────────────────────────────────────

// CastVote inserts a ballot vote and returns the fully-populated row.
// Returns an error if the unit has already voted (UNIQUE ballot_id, unit_id).
func (r *PostgresBallotRepository) CastVote(ctx context.Context, v *BallotVote) (*BallotVote, error) {
	const q = `
		INSERT INTO ballot_votes (ballot_id, voter_id, unit_id, selection, vote_weight)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, ballot_id, voter_id, unit_id, selection, vote_weight, created_at`

	row := r.pool.QueryRow(ctx, q,
		v.BallotID,
		v.VoterID,
		v.UnitID,
		v.Selection,
		v.VoteWeight,
	)

	result, err := scanBallotVote(row)
	if err != nil {
		return nil, fmt.Errorf("ballot: CastVote: %w", err)
	}
	return result, nil
}

// ─── ListVotesByBallot ───────────────────────────────────────────────────────

// ListVotesByBallot returns all votes for the given ballot, ordered by created_at ASC.
func (r *PostgresBallotRepository) ListVotesByBallot(ctx context.Context, ballotID uuid.UUID) ([]BallotVote, error) {
	const q = `
		SELECT id, ballot_id, voter_id, unit_id, selection, vote_weight, created_at
		FROM ballot_votes
		WHERE ballot_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, ballotID)
	if err != nil {
		return nil, fmt.Errorf("ballot: ListVotesByBallot: %w", err)
	}
	defer rows.Close()

	return collectBallotVotes(rows, "ListVotesByBallot")
}

// ─── FileProxy ───────────────────────────────────────────────────────────────

// FileProxy inserts a proxy authorization and returns the fully-populated row.
// Returns an error if the unit already has an active proxy (UNIQUE ballot_id, unit_id).
func (r *PostgresBallotRepository) FileProxy(ctx context.Context, p *ProxyAuthorization) (*ProxyAuthorization, error) {
	const q = `
		INSERT INTO proxy_authorizations (ballot_id, unit_id, grantor_id, proxy_id, filed_at, revoked_at, document_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, ballot_id, unit_id, grantor_id, proxy_id, filed_at, revoked_at, document_id, created_at`

	row := r.pool.QueryRow(ctx, q,
		p.BallotID,
		p.UnitID,
		p.GrantorID,
		p.ProxyID,
		p.FiledAt,
		p.RevokedAt,
		p.DocumentID,
	)

	result, err := scanProxyAuthorization(row)
	if err != nil {
		return nil, fmt.Errorf("ballot: FileProxy: %w", err)
	}
	return result, nil
}

// ─── RevokeProxy ─────────────────────────────────────────────────────────────

// RevokeProxy sets revoked_at = now() on a proxy authorization that has not yet been revoked.
func (r *PostgresBallotRepository) RevokeProxy(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE proxy_authorizations SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("ballot: RevokeProxy: %w", err)
	}
	return nil
}

// ─── ListProxiesByBallot ──────────────────────────────────────────────────────

// ListProxiesByBallot returns all proxy authorizations for the given ballot, ordered by created_at ASC.
func (r *PostgresBallotRepository) ListProxiesByBallot(ctx context.Context, ballotID uuid.UUID) ([]ProxyAuthorization, error) {
	const q = `
		SELECT id, ballot_id, unit_id, grantor_id, proxy_id, filed_at, revoked_at, document_id, created_at
		FROM proxy_authorizations
		WHERE ballot_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, ballotID)
	if err != nil {
		return nil, fmt.Errorf("ballot: ListProxiesByBallot: %w", err)
	}
	defer rows.Close()

	return collectProxyAuthorizations(rows, "ListProxiesByBallot")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// scanBallot reads a single ballots row.
func scanBallot(row pgx.Row) (*Ballot, error) {
	var b Ballot
	var optionsRaw []byte
	var resultsRaw []byte

	err := row.Scan(
		&b.ID,
		&b.OrgID,
		&b.Title,
		&b.Description,
		&b.BallotType,
		&b.Status,
		&optionsRaw,
		&b.EligibleRole,
		&b.OpensAt,
		&b.ClosesAt,
		&b.QuorumPercent,
		&b.PassPercent,
		&b.EligibleUnits,
		&b.VotesCast,
		&b.QuorumMet,
		&b.WeightMethod,
		&resultsRaw,
		&b.CreatedBy,
		&b.CreatedAt,
		&b.UpdatedAt,
		&b.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(optionsRaw) > 0 {
		b.Options = json.RawMessage(optionsRaw)
	} else {
		b.Options = json.RawMessage("[]")
	}

	if len(resultsRaw) > 0 {
		b.Results = json.RawMessage(resultsRaw)
	}

	return &b, nil
}

// collectBallots drains pgx.Rows into a slice of Ballot values.
func collectBallots(rows pgx.Rows, op string) ([]Ballot, error) {
	var ballots []Ballot
	for rows.Next() {
		var b Ballot
		var optionsRaw []byte
		var resultsRaw []byte

		if err := rows.Scan(
			&b.ID,
			&b.OrgID,
			&b.Title,
			&b.Description,
			&b.BallotType,
			&b.Status,
			&optionsRaw,
			&b.EligibleRole,
			&b.OpensAt,
			&b.ClosesAt,
			&b.QuorumPercent,
			&b.PassPercent,
			&b.EligibleUnits,
			&b.VotesCast,
			&b.QuorumMet,
			&b.WeightMethod,
			&resultsRaw,
			&b.CreatedBy,
			&b.CreatedAt,
			&b.UpdatedAt,
			&b.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("ballot: %s scan: %w", op, err)
		}

		if len(optionsRaw) > 0 {
			b.Options = json.RawMessage(optionsRaw)
		} else {
			b.Options = json.RawMessage("[]")
		}

		if len(resultsRaw) > 0 {
			b.Results = json.RawMessage(resultsRaw)
		}

		ballots = append(ballots, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ballot: %s rows: %w", op, err)
	}
	return ballots, nil
}

// scanBallotVote reads a single ballot_votes row.
func scanBallotVote(row pgx.Row) (*BallotVote, error) {
	var v BallotVote
	var selectionRaw []byte

	err := row.Scan(
		&v.ID,
		&v.BallotID,
		&v.VoterID,
		&v.UnitID,
		&selectionRaw,
		&v.VoteWeight,
		&v.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(selectionRaw) > 0 {
		v.Selection = json.RawMessage(selectionRaw)
	}

	return &v, nil
}

// collectBallotVotes drains pgx.Rows into a slice of BallotVote values.
func collectBallotVotes(rows pgx.Rows, op string) ([]BallotVote, error) {
	var votes []BallotVote
	for rows.Next() {
		var v BallotVote
		var selectionRaw []byte

		if err := rows.Scan(
			&v.ID,
			&v.BallotID,
			&v.VoterID,
			&v.UnitID,
			&selectionRaw,
			&v.VoteWeight,
			&v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ballot: %s scan: %w", op, err)
		}

		if len(selectionRaw) > 0 {
			v.Selection = json.RawMessage(selectionRaw)
		}

		votes = append(votes, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ballot: %s rows: %w", op, err)
	}
	return votes, nil
}

// scanProxyAuthorization reads a single proxy_authorizations row.
func scanProxyAuthorization(row pgx.Row) (*ProxyAuthorization, error) {
	var p ProxyAuthorization

	err := row.Scan(
		&p.ID,
		&p.BallotID,
		&p.UnitID,
		&p.GrantorID,
		&p.ProxyID,
		&p.FiledAt,
		&p.RevokedAt,
		&p.DocumentID,
		&p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// collectProxyAuthorizations drains pgx.Rows into a slice of ProxyAuthorization values.
func collectProxyAuthorizations(rows pgx.Rows, op string) ([]ProxyAuthorization, error) {
	var proxies []ProxyAuthorization
	for rows.Next() {
		var p ProxyAuthorization

		if err := rows.Scan(
			&p.ID,
			&p.BallotID,
			&p.UnitID,
			&p.GrantorID,
			&p.ProxyID,
			&p.FiledAt,
			&p.RevokedAt,
			&p.DocumentID,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ballot: %s scan: %w", op, err)
		}

		proxies = append(proxies, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ballot: %s rows: %w", op, err)
	}
	return proxies, nil
}
