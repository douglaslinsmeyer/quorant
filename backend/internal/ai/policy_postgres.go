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

// PostgresPolicyRepository implements PolicyRepository using pgxpool.
type PostgresPolicyRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPolicyRepository constructs a PostgresPolicyRepository backed by pool.
func NewPostgresPolicyRepository(pool *pgxpool.Pool) *PostgresPolicyRepository {
	return &PostgresPolicyRepository{pool: pool}
}

// ─── Governing Documents ──────────────────────────────────────────────────────

// CreateGoverningDoc inserts a new governing document and returns the persisted record.
func (r *PostgresPolicyRepository) CreateGoverningDoc(ctx context.Context, doc *GoverningDocument) (*GoverningDocument, error) {
	const q = `
		INSERT INTO governing_documents (
			org_id, document_id, doc_type, title,
			effective_date, supersedes_id, indexing_status
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7
		)
		RETURNING id, org_id, document_id, doc_type, title,
		          effective_date, supersedes_id, indexing_status,
		          indexed_at, chunk_count, extraction_count, created_at`

	row := r.pool.QueryRow(ctx, q,
		doc.OrgID,
		doc.DocumentID,
		doc.DocType,
		doc.Title,
		doc.EffectiveDate,
		doc.SupersedesID,
		doc.IndexingStatus,
	)
	result, err := scanGoverningDoc(row)
	if err != nil {
		return nil, fmt.Errorf("ai: CreateGoverningDoc: %w", err)
	}
	return result, nil
}

// FindGoverningDocByID returns the governing document with the given ID, or nil if not found.
func (r *PostgresPolicyRepository) FindGoverningDocByID(ctx context.Context, id uuid.UUID) (*GoverningDocument, error) {
	const q = `
		SELECT id, org_id, document_id, doc_type, title,
		       effective_date, supersedes_id, indexing_status,
		       indexed_at, chunk_count, extraction_count, created_at
		FROM governing_documents
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanGoverningDoc(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: FindGoverningDocByID: %w", err)
	}
	return result, nil
}

// ListGoverningDocsByOrg returns all governing documents for the given org, ordered by effective_date DESC.
func (r *PostgresPolicyRepository) ListGoverningDocsByOrg(ctx context.Context, orgID uuid.UUID) ([]GoverningDocument, error) {
	const q = `
		SELECT id, org_id, document_id, doc_type, title,
		       effective_date, supersedes_id, indexing_status,
		       indexed_at, chunk_count, extraction_count, created_at
		FROM governing_documents
		WHERE org_id = $1
		ORDER BY effective_date DESC, created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: ListGoverningDocsByOrg: %w", err)
	}
	defer rows.Close()

	return collectGoverningDocs(rows, "ListGoverningDocsByOrg")
}

// UpdateGoverningDoc persists changes to an existing governing document and returns the updated row.
func (r *PostgresPolicyRepository) UpdateGoverningDoc(ctx context.Context, doc *GoverningDocument) (*GoverningDocument, error) {
	const q = `
		UPDATE governing_documents SET
			doc_type         = $1,
			title            = $2,
			effective_date   = $3,
			supersedes_id    = $4,
			indexing_status  = $5,
			indexed_at       = $6,
			chunk_count      = $7,
			extraction_count = $8
		WHERE id = $9
		RETURNING id, org_id, document_id, doc_type, title,
		          effective_date, supersedes_id, indexing_status,
		          indexed_at, chunk_count, extraction_count, created_at`

	row := r.pool.QueryRow(ctx, q,
		doc.DocType,
		doc.Title,
		doc.EffectiveDate,
		doc.SupersedesID,
		doc.IndexingStatus,
		doc.IndexedAt,
		doc.ChunkCount,
		doc.ExtractionCount,
		doc.ID,
	)
	result, err := scanGoverningDoc(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ai: UpdateGoverningDoc: %s not found", doc.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("ai: UpdateGoverningDoc: %w", err)
	}
	return result, nil
}

// ─── Policy Extractions ───────────────────────────────────────────────────────

// CreateExtraction inserts a new policy extraction and returns the persisted record.
func (r *PostgresPolicyRepository) CreateExtraction(ctx context.Context, e *PolicyExtraction) (*PolicyExtraction, error) {
	const q = `
		INSERT INTO policy_extractions (
			org_id, domain, policy_key, config,
			confidence, source_doc_id, source_text,
			source_section, source_page,
			review_status, reviewed_by, reviewed_at, human_override,
			model_version, effective_at, superseded_by
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9,
			$10, $11, $12, $13,
			$14, $15, $16
		)
		RETURNING id, org_id, domain, policy_key, config,
		          confidence, source_doc_id, source_text,
		          source_section, source_page,
		          review_status, reviewed_by, reviewed_at, human_override,
		          model_version, effective_at, superseded_by, created_at`

	row := r.pool.QueryRow(ctx, q,
		e.OrgID,
		e.Domain,
		e.PolicyKey,
		marshalRawOrNull(e.Config),
		e.Confidence,
		e.SourceDocID,
		e.SourceText,
		e.SourceSection,
		e.SourcePage,
		e.ReviewStatus,
		e.ReviewedBy,
		e.ReviewedAt,
		marshalRawOrNull(e.HumanOverride),
		e.ModelVersion,
		e.EffectiveAt,
		e.SupersededBy,
	)
	result, err := scanExtraction(row)
	if err != nil {
		return nil, fmt.Errorf("ai: CreateExtraction: %w", err)
	}
	return result, nil
}

// FindExtractionByID returns the policy extraction with the given ID, or nil if not found.
func (r *PostgresPolicyRepository) FindExtractionByID(ctx context.Context, id uuid.UUID) (*PolicyExtraction, error) {
	const q = `
		SELECT id, org_id, domain, policy_key, config,
		       confidence, source_doc_id, source_text,
		       source_section, source_page,
		       review_status, reviewed_by, reviewed_at, human_override,
		       model_version, effective_at, superseded_by, created_at
		FROM policy_extractions
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanExtraction(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: FindExtractionByID: %w", err)
	}
	return result, nil
}

// ListExtractionsByOrg returns all policy extractions for the given org, ordered by created_at DESC.
func (r *PostgresPolicyRepository) ListExtractionsByOrg(ctx context.Context, orgID uuid.UUID) ([]PolicyExtraction, error) {
	const q = `
		SELECT id, org_id, domain, policy_key, config,
		       confidence, source_doc_id, source_text,
		       source_section, source_page,
		       review_status, reviewed_by, reviewed_at, human_override,
		       model_version, effective_at, superseded_by, created_at
		FROM policy_extractions
		WHERE org_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: ListExtractionsByOrg: %w", err)
	}
	defer rows.Close()

	return collectExtractions(rows, "ListExtractionsByOrg")
}

// FindActiveExtraction returns the active (not superseded, approved or pending) extraction
// for the given org and policy key, or nil if none exists.
func (r *PostgresPolicyRepository) FindActiveExtraction(ctx context.Context, orgID uuid.UUID, policyKey string) (*PolicyExtraction, error) {
	const q = `
		SELECT id, org_id, domain, policy_key, config,
		       confidence, source_doc_id, source_text,
		       source_section, source_page,
		       review_status, reviewed_by, reviewed_at, human_override,
		       model_version, effective_at, superseded_by, created_at
		FROM policy_extractions
		WHERE org_id = $1
		  AND policy_key = $2
		  AND superseded_by IS NULL
		  AND review_status IN ('approved', 'pending')
		ORDER BY effective_at DESC
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, orgID, policyKey)
	result, err := scanExtraction(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: FindActiveExtraction: %w", err)
	}
	return result, nil
}

// UpdateExtraction persists changes to an existing policy extraction and returns the updated row.
func (r *PostgresPolicyRepository) UpdateExtraction(ctx context.Context, e *PolicyExtraction) (*PolicyExtraction, error) {
	const q = `
		UPDATE policy_extractions SET
			domain         = $1,
			policy_key     = $2,
			config         = $3,
			confidence     = $4,
			source_text    = $5,
			source_section = $6,
			source_page    = $7,
			review_status  = $8,
			reviewed_by    = $9,
			reviewed_at    = $10,
			human_override = $11,
			model_version  = $12,
			effective_at   = $13,
			superseded_by  = $14
		WHERE id = $15
		RETURNING id, org_id, domain, policy_key, config,
		          confidence, source_doc_id, source_text,
		          source_section, source_page,
		          review_status, reviewed_by, reviewed_at, human_override,
		          model_version, effective_at, superseded_by, created_at`

	row := r.pool.QueryRow(ctx, q,
		e.Domain,
		e.PolicyKey,
		marshalRawOrNull(e.Config),
		e.Confidence,
		e.SourceText,
		e.SourceSection,
		e.SourcePage,
		e.ReviewStatus,
		e.ReviewedBy,
		e.ReviewedAt,
		marshalRawOrNull(e.HumanOverride),
		e.ModelVersion,
		e.EffectiveAt,
		e.SupersededBy,
		e.ID,
	)
	result, err := scanExtraction(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ai: UpdateExtraction: %s not found", e.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("ai: UpdateExtraction: %w", err)
	}
	return result, nil
}

// ─── Policy Resolutions ───────────────────────────────────────────────────────

// CreateResolution inserts a new policy resolution log entry and returns the persisted record.
func (r *PostgresPolicyRepository) CreateResolution(ctx context.Context, res *PolicyResolution) (*PolicyResolution, error) {
	const q = `
		INSERT INTO policy_resolutions (
			org_id, query, policy_keys, resolution,
			reasoning, source_passages, confidence,
			resolution_type, model_version, latency_ms,
			requesting_module, requesting_context,
			human_decision, decided_by, decided_at, fed_back
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			$11, $12,
			$13, $14, $15, $16
		)
		RETURNING id, org_id, query, policy_keys, resolution,
		          reasoning, source_passages, confidence,
		          resolution_type, model_version, latency_ms,
		          requesting_module, requesting_context,
		          human_decision, decided_by, decided_at, fed_back, created_at`

	row := r.pool.QueryRow(ctx, q,
		res.OrgID,
		res.Query,
		res.PolicyKeys,
		marshalRawOrEmpty(res.Resolution),
		res.Reasoning,
		marshalRawOrEmpty(res.SourcePassages),
		res.Confidence,
		res.ResolutionType,
		res.ModelVersion,
		res.LatencyMs,
		res.RequestingModule,
		marshalRawOrEmpty(res.RequestingContext),
		marshalRawOrNull(res.HumanDecision),
		res.DecidedBy,
		res.DecidedAt,
		res.FedBack,
	)
	result, err := scanResolution(row)
	if err != nil {
		return nil, fmt.Errorf("ai: CreateResolution: %w", err)
	}
	return result, nil
}

// FindResolutionByID returns the policy resolution with the given ID, or nil if not found.
func (r *PostgresPolicyRepository) FindResolutionByID(ctx context.Context, id uuid.UUID) (*PolicyResolution, error) {
	const q = `
		SELECT id, org_id, query, policy_keys, resolution,
		       reasoning, source_passages, confidence,
		       resolution_type, model_version, latency_ms,
		       requesting_module, requesting_context,
		       human_decision, decided_by, decided_at, fed_back, created_at
		FROM policy_resolutions
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanResolution(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: FindResolutionByID: %w", err)
	}
	return result, nil
}

// ListResolutionsByOrg returns all policy resolutions for the given org, ordered by created_at DESC.
func (r *PostgresPolicyRepository) ListResolutionsByOrg(ctx context.Context, orgID uuid.UUID) ([]PolicyResolution, error) {
	const q = `
		SELECT id, org_id, query, policy_keys, resolution,
		       reasoning, source_passages, confidence,
		       resolution_type, model_version, latency_ms,
		       requesting_module, requesting_context,
		       human_decision, decided_by, decided_at, fed_back, created_at
		FROM policy_resolutions
		WHERE org_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: ListResolutionsByOrg: %w", err)
	}
	defer rows.Close()

	return collectResolutions(rows, "ListResolutionsByOrg")
}

// ListPendingEscalations returns human-escalated resolutions that have not been decided yet.
func (r *PostgresPolicyRepository) ListPendingEscalations(ctx context.Context, orgID uuid.UUID) ([]PolicyResolution, error) {
	const q = `
		SELECT id, org_id, query, policy_keys, resolution,
		       reasoning, source_passages, confidence,
		       resolution_type, model_version, latency_ms,
		       requesting_module, requesting_context,
		       human_decision, decided_by, decided_at, fed_back, created_at
		FROM policy_resolutions
		WHERE org_id = $1
		  AND resolution_type = 'human_escalated'
		  AND decided_at IS NULL
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: ListPendingEscalations: %w", err)
	}
	defer rows.Close()

	return collectResolutions(rows, "ListPendingEscalations")
}

// UpdateResolution persists changes to an existing policy resolution and returns the updated row.
func (r *PostgresPolicyRepository) UpdateResolution(ctx context.Context, res *PolicyResolution) (*PolicyResolution, error) {
	const q = `
		UPDATE policy_resolutions SET
			query              = $1,
			policy_keys        = $2,
			resolution         = $3,
			reasoning          = $4,
			source_passages    = $5,
			confidence         = $6,
			resolution_type    = $7,
			model_version      = $8,
			latency_ms         = $9,
			requesting_module  = $10,
			requesting_context = $11,
			human_decision     = $12,
			decided_by         = $13,
			decided_at         = $14,
			fed_back           = $15
		WHERE id = $16
		RETURNING id, org_id, query, policy_keys, resolution,
		          reasoning, source_passages, confidence,
		          resolution_type, model_version, latency_ms,
		          requesting_module, requesting_context,
		          human_decision, decided_by, decided_at, fed_back, created_at`

	row := r.pool.QueryRow(ctx, q,
		res.Query,
		res.PolicyKeys,
		marshalRawOrEmpty(res.Resolution),
		res.Reasoning,
		marshalRawOrEmpty(res.SourcePassages),
		res.Confidence,
		res.ResolutionType,
		res.ModelVersion,
		res.LatencyMs,
		res.RequestingModule,
		marshalRawOrEmpty(res.RequestingContext),
		marshalRawOrNull(res.HumanDecision),
		res.DecidedBy,
		res.DecidedAt,
		res.FedBack,
		res.ID,
	)
	result, err := scanResolution(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ai: UpdateResolution: %s not found", res.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("ai: UpdateResolution: %w", err)
	}
	return result, nil
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

func scanGoverningDoc(row pgx.Row) (*GoverningDocument, error) {
	var d GoverningDocument
	err := row.Scan(
		&d.ID,
		&d.OrgID,
		&d.DocumentID,
		&d.DocType,
		&d.Title,
		&d.EffectiveDate,
		&d.SupersedesID,
		&d.IndexingStatus,
		&d.IndexedAt,
		&d.ChunkCount,
		&d.ExtractionCount,
		&d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func collectGoverningDocs(rows pgx.Rows, op string) ([]GoverningDocument, error) {
	results := []GoverningDocument{}
	for rows.Next() {
		var d GoverningDocument
		if err := rows.Scan(
			&d.ID,
			&d.OrgID,
			&d.DocumentID,
			&d.DocType,
			&d.Title,
			&d.EffectiveDate,
			&d.SupersedesID,
			&d.IndexingStatus,
			&d.IndexedAt,
			&d.ChunkCount,
			&d.ExtractionCount,
			&d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ai: %s scan: %w", op, err)
		}
		results = append(results, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: %s rows: %w", op, err)
	}
	return results, nil
}

func scanExtraction(row pgx.Row) (*PolicyExtraction, error) {
	var e PolicyExtraction
	var configRaw, overrideRaw []byte
	err := row.Scan(
		&e.ID,
		&e.OrgID,
		&e.Domain,
		&e.PolicyKey,
		&configRaw,
		&e.Confidence,
		&e.SourceDocID,
		&e.SourceText,
		&e.SourceSection,
		&e.SourcePage,
		&e.ReviewStatus,
		&e.ReviewedBy,
		&e.ReviewedAt,
		&overrideRaw,
		&e.ModelVersion,
		&e.EffectiveAt,
		&e.SupersededBy,
		&e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(configRaw) > 0 {
		e.Config = json.RawMessage(configRaw)
	}
	if len(overrideRaw) > 0 {
		e.HumanOverride = json.RawMessage(overrideRaw)
	}
	return &e, nil
}

func collectExtractions(rows pgx.Rows, op string) ([]PolicyExtraction, error) {
	results := []PolicyExtraction{}
	for rows.Next() {
		var e PolicyExtraction
		var configRaw, overrideRaw []byte
		if err := rows.Scan(
			&e.ID,
			&e.OrgID,
			&e.Domain,
			&e.PolicyKey,
			&configRaw,
			&e.Confidence,
			&e.SourceDocID,
			&e.SourceText,
			&e.SourceSection,
			&e.SourcePage,
			&e.ReviewStatus,
			&e.ReviewedBy,
			&e.ReviewedAt,
			&overrideRaw,
			&e.ModelVersion,
			&e.EffectiveAt,
			&e.SupersededBy,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ai: %s scan: %w", op, err)
		}
		if len(configRaw) > 0 {
			e.Config = json.RawMessage(configRaw)
		}
		if len(overrideRaw) > 0 {
			e.HumanOverride = json.RawMessage(overrideRaw)
		}
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: %s rows: %w", op, err)
	}
	return results, nil
}

func scanResolution(row pgx.Row) (*PolicyResolution, error) {
	var r PolicyResolution
	var resolutionRaw, passagesRaw, ctxRaw, decisionRaw []byte
	err := row.Scan(
		&r.ID,
		&r.OrgID,
		&r.Query,
		&r.PolicyKeys,
		&resolutionRaw,
		&r.Reasoning,
		&passagesRaw,
		&r.Confidence,
		&r.ResolutionType,
		&r.ModelVersion,
		&r.LatencyMs,
		&r.RequestingModule,
		&ctxRaw,
		&decisionRaw,
		&r.DecidedBy,
		&r.DecidedAt,
		&r.FedBack,
		&r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if r.PolicyKeys == nil {
		r.PolicyKeys = []string{}
	}
	if len(resolutionRaw) > 0 {
		r.Resolution = json.RawMessage(resolutionRaw)
	}
	if len(passagesRaw) > 0 {
		r.SourcePassages = json.RawMessage(passagesRaw)
	}
	if len(ctxRaw) > 0 {
		r.RequestingContext = json.RawMessage(ctxRaw)
	}
	if len(decisionRaw) > 0 {
		r.HumanDecision = json.RawMessage(decisionRaw)
	}
	return &r, nil
}

func collectResolutions(rows pgx.Rows, op string) ([]PolicyResolution, error) {
	results := []PolicyResolution{}
	for rows.Next() {
		var r PolicyResolution
		var resolutionRaw, passagesRaw, ctxRaw, decisionRaw []byte
		if err := rows.Scan(
			&r.ID,
			&r.OrgID,
			&r.Query,
			&r.PolicyKeys,
			&resolutionRaw,
			&r.Reasoning,
			&passagesRaw,
			&r.Confidence,
			&r.ResolutionType,
			&r.ModelVersion,
			&r.LatencyMs,
			&r.RequestingModule,
			&ctxRaw,
			&decisionRaw,
			&r.DecidedBy,
			&r.DecidedAt,
			&r.FedBack,
			&r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ai: %s scan: %w", op, err)
		}
		if r.PolicyKeys == nil {
			r.PolicyKeys = []string{}
		}
		if len(resolutionRaw) > 0 {
			r.Resolution = json.RawMessage(resolutionRaw)
		}
		if len(passagesRaw) > 0 {
			r.SourcePassages = json.RawMessage(passagesRaw)
		}
		if len(ctxRaw) > 0 {
			r.RequestingContext = json.RawMessage(ctxRaw)
		}
		if len(decisionRaw) > 0 {
			r.HumanDecision = json.RawMessage(decisionRaw)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: %s rows: %w", op, err)
	}
	return results, nil
}

// ─── Marshal helpers ──────────────────────────────────────────────────────────

// marshalRawOrNull converts a json.RawMessage to bytes, returning nil if empty.
func marshalRawOrNull(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

// marshalRawOrEmpty converts a json.RawMessage to bytes, returning '{}' if empty.
func marshalRawOrEmpty(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return []byte(raw)
}

