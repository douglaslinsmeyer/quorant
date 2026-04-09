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

// PostgresUnitRepository implements UnitRepository using a pgxpool.
type PostgresUnitRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresUnitRepository creates a new PostgresUnitRepository backed by pool.
func NewPostgresUnitRepository(pool *pgxpool.Pool) *PostgresUnitRepository {
	return &PostgresUnitRepository{pool: pool}
}

// ─── CreateUnit ──────────────────────────────────────────────────────────────

// CreateUnit inserts a new unit. voting_weight defaults to 1.00 if not set.
// JSONB metadata defaults to '{}' if nil.
func (r *PostgresUnitRepository) CreateUnit(ctx context.Context, unit *Unit) (*Unit, error) {
	if unit.VotingWeight == 0 {
		unit.VotingWeight = 1.00
	}
	if unit.Metadata == nil {
		unit.Metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(unit.Metadata)
	if err != nil {
		return nil, fmt.Errorf("unit: CreateUnit marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO units (
			org_id, label, unit_type,
			address_line1, address_line2, city, state, zip,
			status, lot_size_sqft, voting_weight, metadata
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7, $8,
			$9, $10, $11, $12
		)
		RETURNING id, org_id, label, unit_type,
		          address_line1, address_line2, city, state, zip,
		          status, lot_size_sqft, voting_weight, metadata,
		          created_at, updated_at, deleted_at`

	status := unit.Status
	if status == "" {
		status = "occupied"
	}

	row := r.pool.QueryRow(ctx, q,
		unit.OrgID,
		unit.Label,
		unit.UnitType,
		unit.AddressLine1,
		unit.AddressLine2,
		unit.City,
		unit.State,
		unit.Zip,
		status,
		unit.LotSizeSqft,
		unit.VotingWeight,
		metadataJSON,
	)

	result, err := scanUnit(row)
	if err != nil {
		return nil, fmt.Errorf("unit: CreateUnit: %w", err)
	}
	return result, nil
}

// ─── FindUnitByID ────────────────────────────────────────────────────────────

// FindUnitByID returns the unit with the given ID, or nil,nil if not found or soft-deleted.
func (r *PostgresUnitRepository) FindUnitByID(ctx context.Context, id uuid.UUID) (*Unit, error) {
	const q = `
		SELECT id, org_id, label, unit_type,
		       address_line1, address_line2, city, state, zip,
		       status, lot_size_sqft, voting_weight, metadata,
		       created_at, updated_at, deleted_at
		FROM units
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanUnit(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unit: FindUnitByID: %w", err)
	}
	return result, nil
}

// ─── ListUnitsByOrg ──────────────────────────────────────────────────────────

// ListUnitsByOrg returns all non-deleted units for the given org, ordered by label.
func (r *PostgresUnitRepository) ListUnitsByOrg(ctx context.Context, orgID uuid.UUID) ([]Unit, error) {
	const q = `
		SELECT id, org_id, label, unit_type,
		       address_line1, address_line2, city, state, zip,
		       status, lot_size_sqft, voting_weight, metadata,
		       created_at, updated_at, deleted_at
		FROM units
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY label`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("unit: ListUnitsByOrg: %w", err)
	}
	defer rows.Close()

	return collectUnits(rows, "ListUnitsByOrg")
}

// ─── UpdateUnit ──────────────────────────────────────────────────────────────

// UpdateUnit persists changes to an existing unit and returns the updated row.
func (r *PostgresUnitRepository) UpdateUnit(ctx context.Context, unit *Unit) (*Unit, error) {
	if unit.Metadata == nil {
		unit.Metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(unit.Metadata)
	if err != nil {
		return nil, fmt.Errorf("unit: UpdateUnit marshal metadata: %w", err)
	}

	const q = `
		UPDATE units SET
			label         = $1,
			unit_type     = $2,
			address_line1 = $3,
			address_line2 = $4,
			city          = $5,
			state         = $6,
			zip           = $7,
			status        = $8,
			lot_size_sqft = $9,
			voting_weight = $10,
			metadata      = $11,
			updated_at    = now()
		WHERE id = $12 AND deleted_at IS NULL
		RETURNING id, org_id, label, unit_type,
		          address_line1, address_line2, city, state, zip,
		          status, lot_size_sqft, voting_weight, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		unit.Label,
		unit.UnitType,
		unit.AddressLine1,
		unit.AddressLine2,
		unit.City,
		unit.State,
		unit.Zip,
		unit.Status,
		unit.LotSizeSqft,
		unit.VotingWeight,
		metadataJSON,
		unit.ID,
	)

	result, err := scanUnit(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("unit: UpdateUnit: unit %s not found or already deleted", unit.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("unit: UpdateUnit: %w", err)
	}
	return result, nil
}

// ─── SoftDeleteUnit ──────────────────────────────────────────────────────────

// SoftDeleteUnit marks a unit as deleted without removing the row.
func (r *PostgresUnitRepository) SoftDeleteUnit(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE units SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("unit: SoftDeleteUnit: %w", err)
	}
	return nil
}

// ─── GetProperty ─────────────────────────────────────────────────────────────

// GetProperty returns the property record for a unit, or nil,nil if not found.
func (r *PostgresUnitRepository) GetProperty(ctx context.Context, unitID uuid.UUID) (*Property, error) {
	const q = `
		SELECT id, unit_id, parcel_number, square_feet, bedrooms, bathrooms,
		       year_built, metadata, created_at, updated_at
		FROM properties
		WHERE unit_id = $1`

	row := r.pool.QueryRow(ctx, q, unitID)
	result, err := scanProperty(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unit: GetProperty: %w", err)
	}
	return result, nil
}

// ─── UpsertProperty ──────────────────────────────────────────────────────────

// UpsertProperty creates the property if it does not exist, or updates it if it does.
// Since the schema has no unique constraint on unit_id, we check first and then insert or update.
func (r *PostgresUnitRepository) UpsertProperty(ctx context.Context, prop *Property) (*Property, error) {
	if prop.Metadata == nil {
		prop.Metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(prop.Metadata)
	if err != nil {
		return nil, fmt.Errorf("unit: UpsertProperty marshal metadata: %w", err)
	}

	// Check whether a property already exists for this unit.
	existing, err := r.GetProperty(ctx, prop.UnitID)
	if err != nil {
		return nil, fmt.Errorf("unit: UpsertProperty check: %w", err)
	}

	if existing == nil {
		// INSERT
		const insertQ = `
			INSERT INTO properties (
				unit_id, parcel_number, square_feet, bedrooms,
				bathrooms, year_built, metadata
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, unit_id, parcel_number, square_feet, bedrooms, bathrooms,
			          year_built, metadata, created_at, updated_at`

		row := r.pool.QueryRow(ctx, insertQ,
			prop.UnitID,
			prop.ParcelNumber,
			prop.SquareFeet,
			prop.Bedrooms,
			prop.Bathrooms,
			prop.YearBuilt,
			metadataJSON,
		)
		result, err := scanProperty(row)
		if err != nil {
			return nil, fmt.Errorf("unit: UpsertProperty insert: %w", err)
		}
		return result, nil
	}

	// UPDATE
	const updateQ = `
		UPDATE properties SET
			parcel_number = $1,
			square_feet   = $2,
			bedrooms      = $3,
			bathrooms     = $4,
			year_built    = $5,
			metadata      = $6,
			updated_at    = now()
		WHERE unit_id = $7
		RETURNING id, unit_id, parcel_number, square_feet, bedrooms, bathrooms,
		          year_built, metadata, created_at, updated_at`

	row := r.pool.QueryRow(ctx, updateQ,
		prop.ParcelNumber,
		prop.SquareFeet,
		prop.Bedrooms,
		prop.Bathrooms,
		prop.YearBuilt,
		metadataJSON,
		prop.UnitID,
	)
	result, err := scanProperty(row)
	if err != nil {
		return nil, fmt.Errorf("unit: UpsertProperty update: %w", err)
	}
	return result, nil
}

// ─── CreateUnitMembership ────────────────────────────────────────────────────

// CreateUnitMembership inserts a new unit membership and returns the fully-populated row.
func (r *PostgresUnitRepository) CreateUnitMembership(ctx context.Context, m *UnitMembership) (*UnitMembership, error) {
	const q = `
		INSERT INTO unit_memberships (
			unit_id, user_id, relationship, is_voter, started_at, notes
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, unit_id, user_id, relationship, is_voter,
		          started_at, ended_at, notes, created_at, updated_at`

	startedAt := m.StartedAt
	if startedAt.IsZero() {
		startedAt = m.CreatedAt
	}

	row := r.pool.QueryRow(ctx, q,
		m.UnitID,
		m.UserID,
		m.Relationship,
		m.IsVoter,
		startedAt,
		m.Notes,
	)

	result, err := scanUnitMembership(row)
	if err != nil {
		return nil, fmt.Errorf("unit: CreateUnitMembership: %w", err)
	}
	return result, nil
}

// ─── FindUnitMembershipByID ──────────────────────────────────────────────────

// FindUnitMembershipByID returns a single unit membership by its ID, or nil if not found.
func (r *PostgresUnitRepository) FindUnitMembershipByID(ctx context.Context, id uuid.UUID) (*UnitMembership, error) {
	const q = `
		SELECT id, unit_id, user_id, relationship, is_voter,
		       started_at, ended_at, notes, created_at, updated_at
		FROM unit_memberships
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanUnitMembership(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unit: FindUnitMembershipByID: %w", err)
	}
	return result, nil
}

// ─── ListUnitMemberships ─────────────────────────────────────────────────────

// ListUnitMemberships returns all active (ended_at IS NULL) memberships for a unit,
// ordered by relationship.
func (r *PostgresUnitRepository) ListUnitMemberships(ctx context.Context, unitID uuid.UUID) ([]UnitMembership, error) {
	const q = `
		SELECT id, unit_id, user_id, relationship, is_voter,
		       started_at, ended_at, notes, created_at, updated_at
		FROM unit_memberships
		WHERE unit_id = $1 AND ended_at IS NULL
		ORDER BY relationship`

	rows, err := r.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, fmt.Errorf("unit: ListUnitMemberships: %w", err)
	}
	defer rows.Close()

	return collectUnitMemberships(rows, "ListUnitMemberships")
}

// ─── UpdateUnitMembership ────────────────────────────────────────────────────

// UpdateUnitMembership persists changes to an existing unit membership and returns the updated row.
func (r *PostgresUnitRepository) UpdateUnitMembership(ctx context.Context, m *UnitMembership) (*UnitMembership, error) {
	const q = `
		UPDATE unit_memberships SET
			relationship = $1,
			is_voter     = $2,
			notes        = $3,
			updated_at   = now()
		WHERE id = $4 AND ended_at IS NULL
		RETURNING id, unit_id, user_id, relationship, is_voter,
		          started_at, ended_at, notes, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		m.Relationship,
		m.IsVoter,
		m.Notes,
		m.ID,
	)

	result, err := scanUnitMembership(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("unit: UpdateUnitMembership: membership %s not found or already ended", m.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("unit: UpdateUnitMembership: %w", err)
	}
	return result, nil
}

// ─── EndUnitMembership ───────────────────────────────────────────────────────

// EndUnitMembership sets ended_at to now() for the given membership.
func (r *PostgresUnitRepository) EndUnitMembership(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE unit_memberships SET ended_at = now()
		WHERE id = $1 AND ended_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("unit: EndUnitMembership: %w", err)
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// scanUnit reads a single unit row.
func scanUnit(row pgx.Row) (*Unit, error) {
	var u Unit
	var metadataRaw []byte
	err := row.Scan(
		&u.ID,
		&u.OrgID,
		&u.Label,
		&u.UnitType,
		&u.AddressLine1,
		&u.AddressLine2,
		&u.City,
		&u.State,
		&u.Zip,
		&u.Status,
		&u.LotSizeSqft,
		&u.VotingWeight,
		&metadataRaw,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &u.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if u.Metadata == nil {
		u.Metadata = map[string]any{}
	}
	return &u, nil
}

// collectUnits drains a pgx.Rows into a slice of Unit values.
func collectUnits(rows pgx.Rows, op string) ([]Unit, error) {
	var units []Unit
	for rows.Next() {
		var metadataRaw []byte
		var u Unit
		if err := rows.Scan(
			&u.ID,
			&u.OrgID,
			&u.Label,
			&u.UnitType,
			&u.AddressLine1,
			&u.AddressLine2,
			&u.City,
			&u.State,
			&u.Zip,
			&u.Status,
			&u.LotSizeSqft,
			&u.VotingWeight,
			&metadataRaw,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("unit: %s scan: %w", op, err)
		}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &u.Metadata); err != nil {
				return nil, fmt.Errorf("unit: %s unmarshal metadata: %w", op, err)
			}
		}
		if u.Metadata == nil {
			u.Metadata = map[string]any{}
		}
		units = append(units, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unit: %s rows: %w", op, err)
	}
	return units, nil
}

// scanProperty reads a single property row.
func scanProperty(row pgx.Row) (*Property, error) {
	var p Property
	var metadataRaw []byte
	err := row.Scan(
		&p.ID,
		&p.UnitID,
		&p.ParcelNumber,
		&p.SquareFeet,
		&p.Bedrooms,
		&p.Bathrooms,
		&p.YearBuilt,
		&metadataRaw,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &p.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal property metadata: %w", err)
		}
	}
	if p.Metadata == nil {
		p.Metadata = map[string]any{}
	}
	return &p, nil
}

// scanUnitMembership reads a single unit_membership row.
func scanUnitMembership(row pgx.Row) (*UnitMembership, error) {
	var m UnitMembership
	err := row.Scan(
		&m.ID,
		&m.UnitID,
		&m.UserID,
		&m.Relationship,
		&m.IsVoter,
		&m.StartedAt,
		&m.EndedAt,
		&m.Notes,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// collectUnitMemberships drains a pgx.Rows into a slice of UnitMembership values.
func collectUnitMemberships(rows pgx.Rows, op string) ([]UnitMembership, error) {
	var memberships []UnitMembership
	for rows.Next() {
		var m UnitMembership
		if err := rows.Scan(
			&m.ID,
			&m.UnitID,
			&m.UserID,
			&m.Relationship,
			&m.IsVoter,
			&m.StartedAt,
			&m.EndedAt,
			&m.Notes,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("unit: %s scan: %w", op, err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unit: %s rows: %w", op, err)
	}
	return memberships, nil
}
