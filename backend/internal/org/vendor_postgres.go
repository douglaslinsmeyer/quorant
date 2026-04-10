package org

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresVendorRepository implements VendorRepository using a pgxpool.
type PostgresVendorRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresVendorRepository creates a new PostgresVendorRepository backed by pool.
func NewPostgresVendorRepository(pool *pgxpool.Pool) *PostgresVendorRepository {
	return &PostgresVendorRepository{pool: pool}
}

// ─── CreateVendor ────────────────────────────────────────────────────────────

// CreateVendor inserts a new vendor and returns the fully-populated row.
func (r *PostgresVendorRepository) CreateVendor(ctx context.Context, v *Vendor) (*Vendor, error) {
	if v.ServiceTypes == nil {
		v.ServiceTypes = []string{}
	}
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(v.Metadata)
	if err != nil {
		return nil, fmt.Errorf("vendor: CreateVendor marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO vendors (
			name, contact_email, contact_phone, service_types,
			license_number, insurance_expiry, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, contact_email, contact_phone, service_types,
		          license_number, insurance_expiry, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		v.Name,
		v.ContactEmail,
		v.ContactPhone,
		v.ServiceTypes,
		v.LicenseNumber,
		v.InsuranceExpiry,
		metadataJSON,
	)

	result, err := scanVendor(row)
	if err != nil {
		return nil, fmt.Errorf("vendor: CreateVendor: %w", err)
	}
	return result, nil
}

// ─── FindVendorByID ──────────────────────────────────────────────────────────

// FindVendorByID returns the vendor with the given ID, or nil,nil if not found or soft-deleted.
func (r *PostgresVendorRepository) FindVendorByID(ctx context.Context, id uuid.UUID) (*Vendor, error) {
	const q = `
		SELECT id, name, contact_email, contact_phone, service_types,
		       license_number, insurance_expiry, metadata,
		       created_at, updated_at, deleted_at
		FROM vendors
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanVendor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vendor: FindVendorByID: %w", err)
	}
	return result, nil
}

// ─── ListVendors ─────────────────────────────────────────────────────────────

// ListVendors returns non-deleted vendors, supporting cursor-based pagination ordered by id.
func (r *PostgresVendorRepository) ListVendors(ctx context.Context, limit int, afterID *uuid.UUID) ([]Vendor, bool, error) {
	const q = `
		SELECT id, name, contact_email, contact_phone, service_types,
		       license_number, insurance_expiry, metadata,
		       created_at, updated_at, deleted_at
		FROM vendors
		WHERE deleted_at IS NULL
		  AND ($2::uuid IS NULL OR id > $2)
		ORDER BY id
		LIMIT $1`

	rows, err := r.pool.Query(ctx, q, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("vendor: ListVendors: %w", err)
	}
	defer rows.Close()

	vendors, err := collectVendors(rows, "ListVendors")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(vendors) > limit
	if hasMore {
		vendors = vendors[:limit]
	}
	return vendors, hasMore, nil
}

// ─── UpdateVendor ────────────────────────────────────────────────────────────

// UpdateVendor persists changes to an existing vendor and returns the updated row.
func (r *PostgresVendorRepository) UpdateVendor(ctx context.Context, v *Vendor) (*Vendor, error) {
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(v.Metadata)
	if err != nil {
		return nil, fmt.Errorf("vendor: UpdateVendor marshal metadata: %w", err)
	}

	const q = `
		UPDATE vendors SET
			name             = $1,
			contact_email    = $2,
			contact_phone    = $3,
			service_types    = $4,
			license_number   = $5,
			insurance_expiry = $6,
			metadata         = $7,
			updated_at       = now()
		WHERE id = $8 AND deleted_at IS NULL
		RETURNING id, name, contact_email, contact_phone, service_types,
		          license_number, insurance_expiry, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		v.Name,
		v.ContactEmail,
		v.ContactPhone,
		v.ServiceTypes,
		v.LicenseNumber,
		v.InsuranceExpiry,
		metadataJSON,
		v.ID,
	)

	result, err := scanVendor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("vendor: UpdateVendor: vendor %s not found or already deleted", v.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("vendor: UpdateVendor: %w", err)
	}
	return result, nil
}

// ─── SoftDeleteVendor ────────────────────────────────────────────────────────

// SoftDeleteVendor marks a vendor as deleted without removing the row.
func (r *PostgresVendorRepository) SoftDeleteVendor(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE vendors SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("vendor: SoftDeleteVendor: %w", err)
	}
	return nil
}

// ─── CreateAssignment ────────────────────────────────────────────────────────

// CreateAssignment inserts a new vendor assignment and returns the fully-populated row.
func (r *PostgresVendorRepository) CreateAssignment(ctx context.Context, a *VendorAssignment) (*VendorAssignment, error) {
	const q = `
		INSERT INTO vendor_assignments (
			vendor_id, org_id, service_scope, contract_ref, started_at
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, vendor_id, org_id, service_scope, contract_ref,
		          started_at, ended_at, created_at`

	startedAt := a.StartedAt
	if startedAt.IsZero() {
		startedAt = a.CreatedAt
	}

	row := r.pool.QueryRow(ctx, q,
		a.VendorID,
		a.OrgID,
		a.ServiceScope,
		a.ContractRef,
		startedAt,
	)

	result, err := scanVendorAssignment(row)
	if err != nil {
		return nil, fmt.Errorf("vendor: CreateAssignment: %w", err)
	}
	return result, nil
}

// ─── ListAssignmentsByOrg ────────────────────────────────────────────────────

// ListAssignmentsByOrg returns all active (ended_at IS NULL) vendor assignments for the given org.
func (r *PostgresVendorRepository) ListAssignmentsByOrg(ctx context.Context, orgID uuid.UUID) ([]VendorAssignment, error) {
	const q = `
		SELECT id, vendor_id, org_id, service_scope, contract_ref,
		       started_at, ended_at, created_at
		FROM vendor_assignments
		WHERE org_id = $1 AND ended_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("vendor: ListAssignmentsByOrg: %w", err)
	}
	defer rows.Close()

	return collectVendorAssignments(rows, "ListAssignmentsByOrg")
}

// ─── DeleteAssignment ────────────────────────────────────────────────────────

// DeleteAssignment soft-ends a vendor assignment by setting ended_at to now().
func (r *PostgresVendorRepository) DeleteAssignment(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE vendor_assignments SET ended_at = now() WHERE id = $1 AND ended_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("vendor: DeleteAssignment: %w", err)
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// scanVendor reads a single vendor row.
func scanVendor(row pgx.Row) (*Vendor, error) {
	var v Vendor
	var metadataRaw []byte
	err := row.Scan(
		&v.ID,
		&v.Name,
		&v.ContactEmail,
		&v.ContactPhone,
		&v.ServiceTypes,
		&v.LicenseNumber,
		&v.InsuranceExpiry,
		&metadataRaw,
		&v.CreatedAt,
		&v.UpdatedAt,
		&v.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &v.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal vendor metadata: %w", err)
		}
	}
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}
	if v.ServiceTypes == nil {
		v.ServiceTypes = []string{}
	}
	return &v, nil
}

// collectVendors drains a pgx.Rows into a slice of Vendor values.
func collectVendors(rows pgx.Rows, op string) ([]Vendor, error) {
	var vendors []Vendor
	for rows.Next() {
		var v Vendor
		var metadataRaw []byte
		if err := rows.Scan(
			&v.ID,
			&v.Name,
			&v.ContactEmail,
			&v.ContactPhone,
			&v.ServiceTypes,
			&v.LicenseNumber,
			&v.InsuranceExpiry,
			&metadataRaw,
			&v.CreatedAt,
			&v.UpdatedAt,
			&v.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("vendor: %s scan: %w", op, err)
		}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &v.Metadata); err != nil {
				return nil, fmt.Errorf("vendor: %s unmarshal metadata: %w", op, err)
			}
		}
		if v.Metadata == nil {
			v.Metadata = map[string]any{}
		}
		if v.ServiceTypes == nil {
			v.ServiceTypes = []string{}
		}
		vendors = append(vendors, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vendor: %s rows: %w", op, err)
	}
	return vendors, nil
}

// scanVendorAssignment reads a single vendor_assignment row.
func scanVendorAssignment(row pgx.Row) (*VendorAssignment, error) {
	var a VendorAssignment
	err := row.Scan(
		&a.ID,
		&a.VendorID,
		&a.OrgID,
		&a.ServiceScope,
		&a.ContractRef,
		&a.StartedAt,
		&a.EndedAt,
		&a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// collectVendorAssignments drains a pgx.Rows into a slice of VendorAssignment values.
func collectVendorAssignments(rows pgx.Rows, op string) ([]VendorAssignment, error) {
	var assignments []VendorAssignment
	for rows.Next() {
		var a VendorAssignment
		if err := rows.Scan(
			&a.ID,
			&a.VendorID,
			&a.OrgID,
			&a.ServiceScope,
			&a.ContractRef,
			&a.StartedAt,
			&a.EndedAt,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("vendor: %s scan: %w", op, err)
		}
		assignments = append(assignments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vendor: %s rows: %w", op, err)
	}
	return assignments, nil
}
