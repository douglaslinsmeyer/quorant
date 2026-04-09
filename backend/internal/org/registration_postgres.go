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

// PostgresRegistrationRepository implements RegistrationRepository using a pgxpool.
type PostgresRegistrationRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRegistrationRepository creates a new PostgresRegistrationRepository backed by pool.
func NewPostgresRegistrationRepository(pool *pgxpool.Pool) *PostgresRegistrationRepository {
	return &PostgresRegistrationRepository{pool: pool}
}

// ─── CreateRegistrationType ──────────────────────────────────────────────────

// CreateRegistrationType inserts a new registration type and returns the fully-populated row.
func (r *PostgresRegistrationRepository) CreateRegistrationType(ctx context.Context, rt *RegistrationType) (*RegistrationType, error) {
	if rt.Schema == nil {
		rt.Schema = map[string]any{}
	}

	schemaJSON, err := json.Marshal(rt.Schema)
	if err != nil {
		return nil, fmt.Errorf("registration: CreateRegistrationType marshal schema: %w", err)
	}

	const q = `
		INSERT INTO unit_registration_types (
			org_id, name, slug, schema, max_per_unit, requires_approval, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, org_id, name, slug, schema, max_per_unit,
		          requires_approval, is_active, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		rt.OrgID,
		rt.Name,
		rt.Slug,
		schemaJSON,
		rt.MaxPerUnit,
		rt.RequiresApproval,
		rt.IsActive,
	)

	result, err := scanRegistrationType(row)
	if err != nil {
		return nil, fmt.Errorf("registration: CreateRegistrationType: %w", err)
	}
	return result, nil
}

// ─── FindRegistrationTypeByID ────────────────────────────────────────────────

// FindRegistrationTypeByID returns a registration type by ID, or nil if not found.
func (r *PostgresRegistrationRepository) FindRegistrationTypeByID(ctx context.Context, id uuid.UUID) (*RegistrationType, error) {
	const q = `
		SELECT id, org_id, name, slug, schema, max_per_unit,
		       requires_approval, is_active, created_at, updated_at
		FROM unit_registration_types
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanRegistrationType(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registration: FindRegistrationTypeByID: %w", err)
	}
	return result, nil
}

// ─── ListRegistrationTypesByOrg ──────────────────────────────────────────────

// ListRegistrationTypesByOrg returns all registration types for the given org.
func (r *PostgresRegistrationRepository) ListRegistrationTypesByOrg(ctx context.Context, orgID uuid.UUID) ([]RegistrationType, error) {
	const q = `
		SELECT id, org_id, name, slug, schema, max_per_unit,
		       requires_approval, is_active, created_at, updated_at
		FROM unit_registration_types
		WHERE org_id = $1
		ORDER BY name`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("registration: ListRegistrationTypesByOrg: %w", err)
	}
	defer rows.Close()

	return collectRegistrationTypes(rows, "ListRegistrationTypesByOrg")
}

// ─── UpdateRegistrationType ──────────────────────────────────────────────────

// UpdateRegistrationType persists changes to an existing registration type and returns the updated row.
func (r *PostgresRegistrationRepository) UpdateRegistrationType(ctx context.Context, rt *RegistrationType) (*RegistrationType, error) {
	if rt.Schema == nil {
		rt.Schema = map[string]any{}
	}

	schemaJSON, err := json.Marshal(rt.Schema)
	if err != nil {
		return nil, fmt.Errorf("registration: UpdateRegistrationType marshal schema: %w", err)
	}

	const q = `
		UPDATE unit_registration_types SET
			name              = $1,
			slug              = $2,
			schema            = $3,
			max_per_unit      = $4,
			requires_approval = $5,
			is_active         = $6,
			updated_at        = now()
		WHERE id = $7
		RETURNING id, org_id, name, slug, schema, max_per_unit,
		          requires_approval, is_active, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		rt.Name,
		rt.Slug,
		schemaJSON,
		rt.MaxPerUnit,
		rt.RequiresApproval,
		rt.IsActive,
		rt.ID,
	)

	result, err := scanRegistrationType(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("registration: UpdateRegistrationType: type %s not found", rt.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("registration: UpdateRegistrationType: %w", err)
	}
	return result, nil
}

// ─── CreateRegistration ──────────────────────────────────────────────────────

// CreateRegistration inserts a new registration and returns the fully-populated row.
func (r *PostgresRegistrationRepository) CreateRegistration(ctx context.Context, reg *Registration) (*Registration, error) {
	if reg.Data == nil {
		reg.Data = map[string]any{}
	}

	dataJSON, err := json.Marshal(reg.Data)
	if err != nil {
		return nil, fmt.Errorf("registration: CreateRegistration marshal data: %w", err)
	}

	status := reg.Status
	if status == "" {
		status = "active"
	}

	const q = `
		INSERT INTO unit_registrations (
			org_id, unit_id, user_id, registration_type_id, data,
			status, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, org_id, unit_id, user_id, registration_type_id, data,
		          status, approved_by, approved_at, expires_at,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		reg.OrgID,
		reg.UnitID,
		reg.UserID,
		reg.RegistrationTypeID,
		dataJSON,
		status,
		reg.ExpiresAt,
	)

	result, err := scanRegistration(row)
	if err != nil {
		return nil, fmt.Errorf("registration: CreateRegistration: %w", err)
	}
	return result, nil
}

// ─── FindRegistrationByID ────────────────────────────────────────────────────

// FindRegistrationByID returns a registration by ID, or nil if not found or soft-deleted.
func (r *PostgresRegistrationRepository) FindRegistrationByID(ctx context.Context, id uuid.UUID) (*Registration, error) {
	const q = `
		SELECT id, org_id, unit_id, user_id, registration_type_id, data,
		       status, approved_by, approved_at, expires_at,
		       created_at, updated_at, deleted_at
		FROM unit_registrations
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanRegistration(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registration: FindRegistrationByID: %w", err)
	}
	return result, nil
}

// ─── ListRegistrationsByUnit ─────────────────────────────────────────────────

// ListRegistrationsByUnit returns all active registrations for the given unit.
func (r *PostgresRegistrationRepository) ListRegistrationsByUnit(ctx context.Context, unitID uuid.UUID) ([]Registration, error) {
	const q = `
		SELECT id, org_id, unit_id, user_id, registration_type_id, data,
		       status, approved_by, approved_at, expires_at,
		       created_at, updated_at, deleted_at
		FROM unit_registrations
		WHERE unit_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, fmt.Errorf("registration: ListRegistrationsByUnit: %w", err)
	}
	defer rows.Close()

	return collectRegistrations(rows, "ListRegistrationsByUnit")
}

// ─── UpdateRegistration ──────────────────────────────────────────────────────

// UpdateRegistration persists changes to an existing registration and returns the updated row.
func (r *PostgresRegistrationRepository) UpdateRegistration(ctx context.Context, reg *Registration) (*Registration, error) {
	if reg.Data == nil {
		reg.Data = map[string]any{}
	}

	dataJSON, err := json.Marshal(reg.Data)
	if err != nil {
		return nil, fmt.Errorf("registration: UpdateRegistration marshal data: %w", err)
	}

	const q = `
		UPDATE unit_registrations SET
			data        = $1,
			status      = $2,
			approved_by = $3,
			approved_at = $4,
			expires_at  = $5,
			deleted_at  = $6,
			updated_at  = now()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING id, org_id, unit_id, user_id, registration_type_id, data,
		          status, approved_by, approved_at, expires_at,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		dataJSON,
		reg.Status,
		reg.ApprovedBy,
		reg.ApprovedAt,
		reg.ExpiresAt,
		reg.DeletedAt,
		reg.ID,
	)

	result, err := scanRegistration(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("registration: UpdateRegistration: registration %s not found or already deleted", reg.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("registration: UpdateRegistration: %w", err)
	}
	return result, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// scanRegistrationType reads a single unit_registration_types row.
func scanRegistrationType(row pgx.Row) (*RegistrationType, error) {
	var rt RegistrationType
	var schemaRaw []byte
	err := row.Scan(
		&rt.ID,
		&rt.OrgID,
		&rt.Name,
		&rt.Slug,
		&schemaRaw,
		&rt.MaxPerUnit,
		&rt.RequiresApproval,
		&rt.IsActive,
		&rt.CreatedAt,
		&rt.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(schemaRaw) > 0 {
		if err := json.Unmarshal(schemaRaw, &rt.Schema); err != nil {
			return nil, fmt.Errorf("unmarshal registration_type schema: %w", err)
		}
	}
	if rt.Schema == nil {
		rt.Schema = map[string]any{}
	}
	return &rt, nil
}

// collectRegistrationTypes drains a pgx.Rows into a slice of RegistrationType values.
func collectRegistrationTypes(rows pgx.Rows, op string) ([]RegistrationType, error) {
	var types []RegistrationType
	for rows.Next() {
		var rt RegistrationType
		var schemaRaw []byte
		if err := rows.Scan(
			&rt.ID,
			&rt.OrgID,
			&rt.Name,
			&rt.Slug,
			&schemaRaw,
			&rt.MaxPerUnit,
			&rt.RequiresApproval,
			&rt.IsActive,
			&rt.CreatedAt,
			&rt.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("registration: %s scan: %w", op, err)
		}
		if len(schemaRaw) > 0 {
			if err := json.Unmarshal(schemaRaw, &rt.Schema); err != nil {
				return nil, fmt.Errorf("registration: %s unmarshal schema: %w", op, err)
			}
		}
		if rt.Schema == nil {
			rt.Schema = map[string]any{}
		}
		types = append(types, rt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("registration: %s rows: %w", op, err)
	}
	return types, nil
}

// scanRegistration reads a single unit_registrations row.
func scanRegistration(row pgx.Row) (*Registration, error) {
	var reg Registration
	var dataRaw []byte
	err := row.Scan(
		&reg.ID,
		&reg.OrgID,
		&reg.UnitID,
		&reg.UserID,
		&reg.RegistrationTypeID,
		&dataRaw,
		&reg.Status,
		&reg.ApprovedBy,
		&reg.ApprovedAt,
		&reg.ExpiresAt,
		&reg.CreatedAt,
		&reg.UpdatedAt,
		&reg.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(dataRaw) > 0 {
		if err := json.Unmarshal(dataRaw, &reg.Data); err != nil {
			return nil, fmt.Errorf("unmarshal registration data: %w", err)
		}
	}
	if reg.Data == nil {
		reg.Data = map[string]any{}
	}
	return &reg, nil
}

// collectRegistrations drains a pgx.Rows into a slice of Registration values.
func collectRegistrations(rows pgx.Rows, op string) ([]Registration, error) {
	var registrations []Registration
	for rows.Next() {
		var reg Registration
		var dataRaw []byte
		if err := rows.Scan(
			&reg.ID,
			&reg.OrgID,
			&reg.UnitID,
			&reg.UserID,
			&reg.RegistrationTypeID,
			&dataRaw,
			&reg.Status,
			&reg.ApprovedBy,
			&reg.ApprovedAt,
			&reg.ExpiresAt,
			&reg.CreatedAt,
			&reg.UpdatedAt,
			&reg.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("registration: %s scan: %w", op, err)
		}
		if len(dataRaw) > 0 {
			if err := json.Unmarshal(dataRaw, &reg.Data); err != nil {
				return nil, fmt.Errorf("registration: %s unmarshal data: %w", op, err)
			}
		}
		if reg.Data == nil {
			reg.Data = map[string]any{}
		}
		registrations = append(registrations, reg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("registration: %s rows: %w", op, err)
	}
	return registrations, nil
}
