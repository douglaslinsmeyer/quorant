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

// PostgresAmenityRepository implements AmenityRepository using a pgxpool.
type PostgresAmenityRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAmenityRepository creates a new PostgresAmenityRepository backed by pool.
func NewPostgresAmenityRepository(pool *pgxpool.Pool) *PostgresAmenityRepository {
	return &PostgresAmenityRepository{pool: pool}
}

// ─── CreateAmenity ───────────────────────────────────────────────────────────

// CreateAmenity inserts a new amenity and returns the fully-populated row.
func (r *PostgresAmenityRepository) CreateAmenity(ctx context.Context, a *Amenity) (*Amenity, error) {
	if a.ReservationRules == nil {
		a.ReservationRules = map[string]any{}
	}
	if a.Hours == nil {
		a.Hours = map[string]any{}
	}
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}

	rulesJSON, err := json.Marshal(a.ReservationRules)
	if err != nil {
		return nil, fmt.Errorf("amenity: CreateAmenity marshal reservation_rules: %w", err)
	}
	hoursJSON, err := json.Marshal(a.Hours)
	if err != nil {
		return nil, fmt.Errorf("amenity: CreateAmenity marshal hours: %w", err)
	}
	metadataJSON, err := json.Marshal(a.Metadata)
	if err != nil {
		return nil, fmt.Errorf("amenity: CreateAmenity marshal metadata: %w", err)
	}

	status := a.Status
	if status == "" {
		status = "open"
	}

	const q = `
		INSERT INTO amenities (
			org_id, name, amenity_type, description, location,
			capacity, is_reservable, reservation_rules, fee_cents,
			hours, status, metadata
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12
		)
		RETURNING id, org_id, name, amenity_type, description, location,
		          capacity, is_reservable, reservation_rules, fee_cents,
		          hours, status, metadata, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		a.OrgID,
		a.Name,
		a.AmenityType,
		a.Description,
		a.Location,
		a.Capacity,
		a.IsReservable,
		rulesJSON,
		a.FeeCents,
		hoursJSON,
		status,
		metadataJSON,
	)

	result, err := scanAmenity(row)
	if err != nil {
		return nil, fmt.Errorf("amenity: CreateAmenity: %w", err)
	}
	return result, nil
}

// ─── FindAmenityByID ─────────────────────────────────────────────────────────

// FindAmenityByID returns the amenity with the given ID, or nil,nil if not found or soft-deleted.
func (r *PostgresAmenityRepository) FindAmenityByID(ctx context.Context, id uuid.UUID) (*Amenity, error) {
	const q = `
		SELECT id, org_id, name, amenity_type, description, location,
		       capacity, is_reservable, reservation_rules, fee_cents,
		       hours, status, metadata, created_at, updated_at, deleted_at
		FROM amenities
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanAmenity(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("amenity: FindAmenityByID: %w", err)
	}
	return result, nil
}

// ─── ListAmenitiesByOrg ──────────────────────────────────────────────────────

// ListAmenitiesByOrg returns non-deleted amenities for the given org, supporting cursor-based
// pagination ordered by id. afterID is the ID of the last item from the previous page.
func (r *PostgresAmenityRepository) ListAmenitiesByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Amenity, bool, error) {
	const q = `
		SELECT id, org_id, name, amenity_type, description, location,
		       capacity, is_reservable, reservation_rules, fee_cents,
		       hours, status, metadata, created_at, updated_at, deleted_at
		FROM amenities
		WHERE org_id = $1 AND deleted_at IS NULL
		  AND ($3::uuid IS NULL OR id > $3)
		ORDER BY id
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, orgID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("amenity: ListAmenitiesByOrg: %w", err)
	}
	defer rows.Close()

	amenities, err := collectAmenities(rows, "ListAmenitiesByOrg")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(amenities) > limit
	if hasMore {
		amenities = amenities[:limit]
	}
	return amenities, hasMore, nil
}

// ─── UpdateAmenity ───────────────────────────────────────────────────────────

// UpdateAmenity persists changes to an existing amenity and returns the updated row.
func (r *PostgresAmenityRepository) UpdateAmenity(ctx context.Context, a *Amenity) (*Amenity, error) {
	if a.ReservationRules == nil {
		a.ReservationRules = map[string]any{}
	}
	if a.Hours == nil {
		a.Hours = map[string]any{}
	}
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}

	rulesJSON, err := json.Marshal(a.ReservationRules)
	if err != nil {
		return nil, fmt.Errorf("amenity: UpdateAmenity marshal reservation_rules: %w", err)
	}
	hoursJSON, err := json.Marshal(a.Hours)
	if err != nil {
		return nil, fmt.Errorf("amenity: UpdateAmenity marshal hours: %w", err)
	}
	metadataJSON, err := json.Marshal(a.Metadata)
	if err != nil {
		return nil, fmt.Errorf("amenity: UpdateAmenity marshal metadata: %w", err)
	}

	const q = `
		UPDATE amenities SET
			name              = $1,
			amenity_type      = $2,
			description       = $3,
			location          = $4,
			capacity          = $5,
			is_reservable     = $6,
			reservation_rules = $7,
			fee_cents         = $8,
			hours             = $9,
			status            = $10,
			metadata          = $11,
			updated_at        = now()
		WHERE id = $12 AND deleted_at IS NULL
		RETURNING id, org_id, name, amenity_type, description, location,
		          capacity, is_reservable, reservation_rules, fee_cents,
		          hours, status, metadata, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		a.Name,
		a.AmenityType,
		a.Description,
		a.Location,
		a.Capacity,
		a.IsReservable,
		rulesJSON,
		a.FeeCents,
		hoursJSON,
		a.Status,
		metadataJSON,
		a.ID,
	)

	result, err := scanAmenity(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("amenity: UpdateAmenity: amenity %s not found or already deleted", a.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("amenity: UpdateAmenity: %w", err)
	}
	return result, nil
}

// ─── SoftDeleteAmenity ───────────────────────────────────────────────────────

// SoftDeleteAmenity marks an amenity as deleted without removing the row.
func (r *PostgresAmenityRepository) SoftDeleteAmenity(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE amenities SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("amenity: SoftDeleteAmenity: %w", err)
	}
	return nil
}

// ─── CreateReservation ───────────────────────────────────────────────────────

// CreateReservation inserts a new amenity reservation and returns the fully-populated row.
func (r *PostgresAmenityRepository) CreateReservation(ctx context.Context, res *AmenityReservation) (*AmenityReservation, error) {
	status := res.Status
	if status == "" {
		status = "pending"
	}

	const q = `
		INSERT INTO amenity_reservations (
			amenity_id, org_id, user_id, unit_id, status,
			starts_at, ends_at, guest_count, fee_cents, deposit_cents,
			deposit_refunded, notes
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12
		)
		RETURNING id, amenity_id, org_id, user_id, unit_id, status,
		          starts_at, ends_at, guest_count, fee_cents, deposit_cents,
		          deposit_refunded, notes, cancelled_at, cancelled_by, cancellation_reason,
		          created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		res.AmenityID,
		res.OrgID,
		res.UserID,
		res.UnitID,
		status,
		res.StartsAt,
		res.EndsAt,
		res.GuestCount,
		res.FeeCents,
		res.DepositCents,
		res.DepositRefunded,
		res.Notes,
	)

	result, err := scanReservation(row)
	if err != nil {
		return nil, fmt.Errorf("amenity: CreateReservation: %w", err)
	}
	return result, nil
}

// ─── ListReservationsByAmenity ───────────────────────────────────────────────

// ListReservationsByAmenity returns all reservations for the given amenity, ordered by starts_at DESC.
func (r *PostgresAmenityRepository) ListReservationsByAmenity(ctx context.Context, amenityID uuid.UUID) ([]AmenityReservation, error) {
	const q = `
		SELECT id, amenity_id, org_id, user_id, unit_id, status,
		       starts_at, ends_at, guest_count, fee_cents, deposit_cents,
		       deposit_refunded, notes, cancelled_at, cancelled_by, cancellation_reason,
		       created_at, updated_at
		FROM amenity_reservations
		WHERE amenity_id = $1
		ORDER BY starts_at DESC`

	rows, err := r.pool.Query(ctx, q, amenityID)
	if err != nil {
		return nil, fmt.Errorf("amenity: ListReservationsByAmenity: %w", err)
	}
	defer rows.Close()

	return collectReservations(rows, "ListReservationsByAmenity")
}

// ─── ListReservationsByUser ──────────────────────────────────────────────────

// ListReservationsByUser returns all reservations for the given user within an org, ordered by starts_at DESC.
func (r *PostgresAmenityRepository) ListReservationsByUser(ctx context.Context, orgID, userID uuid.UUID) ([]AmenityReservation, error) {
	const q = `
		SELECT id, amenity_id, org_id, user_id, unit_id, status,
		       starts_at, ends_at, guest_count, fee_cents, deposit_cents,
		       deposit_refunded, notes, cancelled_at, cancelled_by, cancellation_reason,
		       created_at, updated_at
		FROM amenity_reservations
		WHERE org_id = $1 AND user_id = $2
		ORDER BY starts_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("amenity: ListReservationsByUser: %w", err)
	}
	defer rows.Close()

	return collectReservations(rows, "ListReservationsByUser")
}

// ─── FindReservationByID ─────────────────────────────────────────────────────

// FindReservationByID returns a reservation by ID, or nil if not found.
func (r *PostgresAmenityRepository) FindReservationByID(ctx context.Context, id uuid.UUID) (*AmenityReservation, error) {
	const q = `
		SELECT id, amenity_id, org_id, user_id, unit_id, status,
		       starts_at, ends_at, guest_count, fee_cents, deposit_cents,
		       deposit_refunded, notes, cancelled_at, cancelled_by, cancellation_reason,
		       created_at, updated_at
		FROM amenity_reservations
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanReservation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("amenity: FindReservationByID: %w", err)
	}
	return result, nil
}

// ─── UpdateReservation ───────────────────────────────────────────────────────

// UpdateReservation persists changes to an existing reservation and returns the updated row.
func (r *PostgresAmenityRepository) UpdateReservation(ctx context.Context, res *AmenityReservation) (*AmenityReservation, error) {
	const q = `
		UPDATE amenity_reservations SET
			status              = $1,
			starts_at           = $2,
			ends_at             = $3,
			guest_count         = $4,
			fee_cents           = $5,
			deposit_cents       = $6,
			deposit_refunded    = $7,
			notes               = $8,
			cancelled_at        = $9,
			cancelled_by        = $10,
			cancellation_reason = $11,
			updated_at          = now()
		WHERE id = $12
		RETURNING id, amenity_id, org_id, user_id, unit_id, status,
		          starts_at, ends_at, guest_count, fee_cents, deposit_cents,
		          deposit_refunded, notes, cancelled_at, cancelled_by, cancellation_reason,
		          created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		res.Status,
		res.StartsAt,
		res.EndsAt,
		res.GuestCount,
		res.FeeCents,
		res.DepositCents,
		res.DepositRefunded,
		res.Notes,
		res.CancelledAt,
		res.CancelledBy,
		res.CancellationReason,
		res.ID,
	)

	result, err := scanReservation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("amenity: UpdateReservation: reservation %s not found", res.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("amenity: UpdateReservation: %w", err)
	}
	return result, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// scanAmenity reads a single amenity row.
func scanAmenity(row pgx.Row) (*Amenity, error) {
	var a Amenity
	var rulesRaw, hoursRaw, metadataRaw []byte
	err := row.Scan(
		&a.ID,
		&a.OrgID,
		&a.Name,
		&a.AmenityType,
		&a.Description,
		&a.Location,
		&a.Capacity,
		&a.IsReservable,
		&rulesRaw,
		&a.FeeCents,
		&hoursRaw,
		&a.Status,
		&metadataRaw,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(rulesRaw) > 0 {
		if err := json.Unmarshal(rulesRaw, &a.ReservationRules); err != nil {
			return nil, fmt.Errorf("unmarshal reservation_rules: %w", err)
		}
	}
	if a.ReservationRules == nil {
		a.ReservationRules = map[string]any{}
	}
	if len(hoursRaw) > 0 {
		if err := json.Unmarshal(hoursRaw, &a.Hours); err != nil {
			return nil, fmt.Errorf("unmarshal hours: %w", err)
		}
	}
	if a.Hours == nil {
		a.Hours = map[string]any{}
	}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &a.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal amenity metadata: %w", err)
		}
	}
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}
	return &a, nil
}

// collectAmenities drains a pgx.Rows into a slice of Amenity values.
func collectAmenities(rows pgx.Rows, op string) ([]Amenity, error) {
	var amenities []Amenity
	for rows.Next() {
		var a Amenity
		var rulesRaw, hoursRaw, metadataRaw []byte
		if err := rows.Scan(
			&a.ID,
			&a.OrgID,
			&a.Name,
			&a.AmenityType,
			&a.Description,
			&a.Location,
			&a.Capacity,
			&a.IsReservable,
			&rulesRaw,
			&a.FeeCents,
			&hoursRaw,
			&a.Status,
			&metadataRaw,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("amenity: %s scan: %w", op, err)
		}
		if len(rulesRaw) > 0 {
			if err := json.Unmarshal(rulesRaw, &a.ReservationRules); err != nil {
				return nil, fmt.Errorf("amenity: %s unmarshal reservation_rules: %w", op, err)
			}
		}
		if a.ReservationRules == nil {
			a.ReservationRules = map[string]any{}
		}
		if len(hoursRaw) > 0 {
			if err := json.Unmarshal(hoursRaw, &a.Hours); err != nil {
				return nil, fmt.Errorf("amenity: %s unmarshal hours: %w", op, err)
			}
		}
		if a.Hours == nil {
			a.Hours = map[string]any{}
		}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &a.Metadata); err != nil {
				return nil, fmt.Errorf("amenity: %s unmarshal metadata: %w", op, err)
			}
		}
		if a.Metadata == nil {
			a.Metadata = map[string]any{}
		}
		amenities = append(amenities, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("amenity: %s rows: %w", op, err)
	}
	return amenities, nil
}

// scanReservation reads a single amenity_reservation row.
func scanReservation(row pgx.Row) (*AmenityReservation, error) {
	var res AmenityReservation
	err := row.Scan(
		&res.ID,
		&res.AmenityID,
		&res.OrgID,
		&res.UserID,
		&res.UnitID,
		&res.Status,
		&res.StartsAt,
		&res.EndsAt,
		&res.GuestCount,
		&res.FeeCents,
		&res.DepositCents,
		&res.DepositRefunded,
		&res.Notes,
		&res.CancelledAt,
		&res.CancelledBy,
		&res.CancellationReason,
		&res.CreatedAt,
		&res.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// collectReservations drains a pgx.Rows into a slice of AmenityReservation values.
func collectReservations(rows pgx.Rows, op string) ([]AmenityReservation, error) {
	var reservations []AmenityReservation
	for rows.Next() {
		var res AmenityReservation
		if err := rows.Scan(
			&res.ID,
			&res.AmenityID,
			&res.OrgID,
			&res.UserID,
			&res.UnitID,
			&res.Status,
			&res.StartsAt,
			&res.EndsAt,
			&res.GuestCount,
			&res.FeeCents,
			&res.DepositCents,
			&res.DepositRefunded,
			&res.Notes,
			&res.CancelledAt,
			&res.CancelledBy,
			&res.CancellationReason,
			&res.CreatedAt,
			&res.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("amenity: %s scan: %w", op, err)
		}
		reservations = append(reservations, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("amenity: %s rows: %w", op, err)
	}
	return reservations, nil
}
