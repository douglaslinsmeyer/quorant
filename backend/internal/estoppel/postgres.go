package estoppel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements EstoppelRepository using a pgxpool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgresRepository backed by pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ─── Requests ─────────────────────────────────────────────────────────────────

// CreateRequest inserts a new estoppel request and returns the
// fully-populated row.
func (r *PostgresRepository) CreateRequest(ctx context.Context, req *EstoppelRequest) (*EstoppelRequest, error) {
	metaJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("estoppel: CreateRequest marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO estoppel_requests (
			org_id, unit_id, task_id,
			request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name,
			closing_date, rush_requested, status,
			fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of,
			metadata, created_by
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7, $8, $9,
			$10, $11,
			$12, $13, $14,
			$15, $16, $17, $18,
			$19, $20, $21,
			$22, $23
		)
		RETURNING
			id, org_id, unit_id, task_id,
			request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name,
			closing_date, rush_requested, status,
			fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of,
			metadata, created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		req.OrgID,
		req.UnitID,
		req.TaskID,
		req.RequestType,
		req.RequestorType,
		req.RequestorName,
		req.RequestorEmail,
		req.RequestorPhone,
		req.RequestorCompany,
		req.PropertyAddress,
		req.OwnerName,
		req.ClosingDate,
		req.RushRequested,
		req.Status,
		req.FeeCents,
		req.RushFeeCents,
		req.DelinquentSurchargeCents,
		req.TotalFeeCents,
		req.DeadlineAt,
		req.AssignedTo,
		req.AmendmentOf,
		metaJSON,
		req.CreatedBy,
	)

	result, err := scanRequest(row)
	if err != nil {
		return nil, fmt.Errorf("estoppel: CreateRequest: %w", err)
	}
	return result, nil
}

// FindRequestByID returns the request with the given id, or nil, nil if not
// found or soft-deleted.
func (r *PostgresRepository) FindRequestByID(ctx context.Context, id uuid.UUID) (*EstoppelRequest, error) {
	const q = `
		SELECT
			id, org_id, unit_id, task_id,
			request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name,
			closing_date, rush_requested, status,
			fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of,
			metadata, created_by, created_at, updated_at, deleted_at
		FROM estoppel_requests
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("estoppel: FindRequestByID: %w", err)
	}
	return result, nil
}

// ListRequestsByOrg returns non-deleted requests for the given org, optionally
// filtered by status. Pagination is cursor-based ordered by created_at DESC.
// afterID is the cursor from the previous page; hasMore is true when more items
// exist beyond the returned page.
func (r *PostgresRepository) ListRequestsByOrg(
	ctx context.Context,
	orgID uuid.UUID,
	status *string,
	limit int,
	afterID *uuid.UUID,
) ([]EstoppelRequest, bool, error) {
	const q = `
		SELECT
			id, org_id, unit_id, task_id,
			request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name,
			closing_date, rush_requested, status,
			fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of,
			metadata, created_by, created_at, updated_at, deleted_at
		FROM estoppel_requests
		WHERE org_id = $1
		  AND deleted_at IS NULL
		  AND ($2::text IS NULL OR status = $2)
		  AND ($4::uuid IS NULL OR id < $4)
		ORDER BY created_at DESC, id DESC
		LIMIT $3`

	rows, err := r.pool.Query(ctx, q, orgID, status, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("estoppel: ListRequestsByOrg: %w", err)
	}
	defer rows.Close()

	requests, err := collectRequests(rows, "ListRequestsByOrg")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(requests) > limit
	if hasMore {
		requests = requests[:limit]
	}
	return requests, hasMore, nil
}

// UpdateRequestStatus updates the status field and updated_at for the given
// request, returning the updated row.
func (r *PostgresRepository) UpdateRequestStatus(ctx context.Context, id uuid.UUID, status string) (*EstoppelRequest, error) {
	const q = `
		UPDATE estoppel_requests
		SET status = $1, updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			id, org_id, unit_id, task_id,
			request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name,
			closing_date, rush_requested, status,
			fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of,
			metadata, created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q, status, id)
	result, err := scanRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("estoppel: UpdateRequestStatus: request %s not found or already deleted", id)
	}
	if err != nil {
		return nil, fmt.Errorf("estoppel: UpdateRequestStatus: %w", err)
	}
	return result, nil
}

// UpdateRequestNarratives stores the generated narrative sections as JSON in
// the request's metadata field under the key "narrative_sections". Returns the
// updated row.
func (r *PostgresRepository) UpdateRequestNarratives(ctx context.Context, id uuid.UUID, narrativeSections []byte) (*EstoppelRequest, error) {
	const q = `
		UPDATE estoppel_requests
		SET metadata   = metadata || jsonb_build_object('narrative_sections', $1::jsonb),
		    updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			id, org_id, unit_id, task_id,
			request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name,
			closing_date, rush_requested, status,
			fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of,
			metadata, created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q, narrativeSections, id)
	result, err := scanRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("estoppel: UpdateRequestNarratives: request %s not found or already deleted", id)
	}
	if err != nil {
		return nil, fmt.Errorf("estoppel: UpdateRequestNarratives: %w", err)
	}
	return result, nil
}

// ─── Certificates ─────────────────────────────────────────────────────────────

// CreateCertificate inserts a new estoppel certificate and returns the
// fully-populated row.
func (r *PostgresRepository) CreateCertificate(ctx context.Context, c *EstoppelCertificate) (*EstoppelCertificate, error) {
	const q = `
		INSERT INTO estoppel_certificates (
			request_id, org_id, unit_id, document_id,
			jurisdiction, effective_date, expires_at,
			data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version,
			amendment_of
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9,
			$10, $11, $12, $13,
			$14
		)
		RETURNING
			id, request_id, org_id, unit_id, document_id,
			jurisdiction, effective_date, expires_at,
			data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version,
			amendment_of, created_at`

	row := r.pool.QueryRow(ctx, q,
		c.RequestID,
		c.OrgID,
		c.UnitID,
		c.DocumentID,
		c.Jurisdiction,
		c.EffectiveDate,
		c.ExpiresAt,
		c.DataSnapshot,
		c.NarrativeSections,
		c.SignedBy,
		c.SignedAt,
		c.SignerTitle,
		c.TemplateVersion,
		c.AmendmentOf,
	)

	result, err := scanCertificate(row)
	if err != nil {
		return nil, fmt.Errorf("estoppel: CreateCertificate: %w", err)
	}
	return result, nil
}

// FindCertificateByID returns the certificate with the given id, or nil, nil
// if not found.
func (r *PostgresRepository) FindCertificateByID(ctx context.Context, id uuid.UUID) (*EstoppelCertificate, error) {
	const q = `
		SELECT
			id, request_id, org_id, unit_id, document_id,
			jurisdiction, effective_date, expires_at,
			data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version,
			amendment_of, created_at
		FROM estoppel_certificates
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanCertificate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("estoppel: FindCertificateByID: %w", err)
	}
	return result, nil
}

// FindCertificateByRequestID returns the most recently created certificate for
// the given request, or nil, nil if none exists.
func (r *PostgresRepository) FindCertificateByRequestID(ctx context.Context, requestID uuid.UUID) (*EstoppelCertificate, error) {
	const q = `
		SELECT
			id, request_id, org_id, unit_id, document_id,
			jurisdiction, effective_date, expires_at,
			data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version,
			amendment_of, created_at
		FROM estoppel_certificates
		WHERE request_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, requestID)
	result, err := scanCertificate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("estoppel: FindCertificateByRequestID: %w", err)
	}
	return result, nil
}

// ListCertificatesByOrg returns all certificates for the given org ordered by
// effective_date DESC. Returns an empty (non-nil) slice when none exist.
func (r *PostgresRepository) ListCertificatesByOrg(ctx context.Context, orgID uuid.UUID) ([]EstoppelCertificate, error) {
	const q = `
		SELECT
			id, request_id, org_id, unit_id, document_id,
			jurisdiction, effective_date, expires_at,
			data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version,
			amendment_of, created_at
		FROM estoppel_certificates
		WHERE org_id = $1
		ORDER BY effective_date DESC, created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("estoppel: ListCertificatesByOrg: %w", err)
	}
	defer rows.Close()

	return collectCertificates(rows, "ListCertificatesByOrg")
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

// scanRequest reads a single estoppel_requests row from a pgx.Row.
func scanRequest(row pgx.Row) (*EstoppelRequest, error) {
	var req EstoppelRequest
	var metaRaw []byte

	err := row.Scan(
		&req.ID,
		&req.OrgID,
		&req.UnitID,
		&req.TaskID,
		&req.RequestType,
		&req.RequestorType,
		&req.RequestorName,
		&req.RequestorEmail,
		&req.RequestorPhone,
		&req.RequestorCompany,
		&req.PropertyAddress,
		&req.OwnerName,
		&req.ClosingDate,
		&req.RushRequested,
		&req.Status,
		&req.FeeCents,
		&req.RushFeeCents,
		&req.DelinquentSurchargeCents,
		&req.TotalFeeCents,
		&req.DeadlineAt,
		&req.AssignedTo,
		&req.AmendmentOf,
		&metaRaw,
		&req.CreatedBy,
		&req.CreatedAt,
		&req.UpdatedAt,
		&req.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &req.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}

	return &req, nil
}

// collectRequests drains pgx.Rows into a slice of EstoppelRequest values.
func collectRequests(rows pgx.Rows, op string) ([]EstoppelRequest, error) {
	requests := []EstoppelRequest{}
	for rows.Next() {
		var req EstoppelRequest
		var metaRaw []byte

		if err := rows.Scan(
			&req.ID,
			&req.OrgID,
			&req.UnitID,
			&req.TaskID,
			&req.RequestType,
			&req.RequestorType,
			&req.RequestorName,
			&req.RequestorEmail,
			&req.RequestorPhone,
			&req.RequestorCompany,
			&req.PropertyAddress,
			&req.OwnerName,
			&req.ClosingDate,
			&req.RushRequested,
			&req.Status,
			&req.FeeCents,
			&req.RushFeeCents,
			&req.DelinquentSurchargeCents,
			&req.TotalFeeCents,
			&req.DeadlineAt,
			&req.AssignedTo,
			&req.AmendmentOf,
			&metaRaw,
			&req.CreatedBy,
			&req.CreatedAt,
			&req.UpdatedAt,
			&req.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("estoppel: %s scan: %w", op, err)
		}

		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &req.Metadata); err != nil {
				return nil, fmt.Errorf("estoppel: %s unmarshal metadata: %w", op, err)
			}
		}
		if req.Metadata == nil {
			req.Metadata = map[string]any{}
		}

		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("estoppel: %s rows: %w", op, err)
	}
	return requests, nil
}

// scanCertificate reads a single estoppel_certificates row from a pgx.Row.
func scanCertificate(row pgx.Row) (*EstoppelCertificate, error) {
	var cert EstoppelCertificate

	err := row.Scan(
		&cert.ID,
		&cert.RequestID,
		&cert.OrgID,
		&cert.UnitID,
		&cert.DocumentID,
		&cert.Jurisdiction,
		&cert.EffectiveDate,
		&cert.ExpiresAt,
		&cert.DataSnapshot,
		&cert.NarrativeSections,
		&cert.SignedBy,
		&cert.SignedAt,
		&cert.SignerTitle,
		&cert.TemplateVersion,
		&cert.AmendmentOf,
		&cert.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &cert, nil
}

// collectCertificates drains pgx.Rows into a slice of EstoppelCertificate
// values.
func collectCertificates(rows pgx.Rows, op string) ([]EstoppelCertificate, error) {
	certs := []EstoppelCertificate{}
	for rows.Next() {
		var cert EstoppelCertificate

		if err := rows.Scan(
			&cert.ID,
			&cert.RequestID,
			&cert.OrgID,
			&cert.UnitID,
			&cert.DocumentID,
			&cert.Jurisdiction,
			&cert.EffectiveDate,
			&cert.ExpiresAt,
			&cert.DataSnapshot,
			&cert.NarrativeSections,
			&cert.SignedBy,
			&cert.SignedAt,
			&cert.SignerTitle,
			&cert.TemplateVersion,
			&cert.AmendmentOf,
			&cert.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("estoppel: %s scan: %w", op, err)
		}

		certs = append(certs, cert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("estoppel: %s rows: %w", op, err)
	}
	return certs, nil
}
