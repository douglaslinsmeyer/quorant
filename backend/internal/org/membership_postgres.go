package org

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/iam"
)

// PostgresMembershipRepository implements MembershipRepository using a pgxpool.
type PostgresMembershipRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresMembershipRepository creates a new PostgresMembershipRepository backed by pool.
func NewPostgresMembershipRepository(pool *pgxpool.Pool) *PostgresMembershipRepository {
	return &PostgresMembershipRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new membership and returns the fully-populated row with role_name.
func (r *PostgresMembershipRepository) Create(ctx context.Context, m *iam.Membership) (*iam.Membership, error) {
	const q = `
		INSERT INTO memberships (user_id, org_id, role_id, status, invited_by, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, org_id, role_id, status, invited_by, joined_at,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		m.UserID,
		m.OrgID,
		m.RoleID,
		m.Status,
		m.InvitedBy,
		m.JoinedAt,
	)

	result, err := scanMembership(row)
	if err != nil {
		return nil, fmt.Errorf("membership: Create: %w", err)
	}

	// Fetch role name separately to populate the denormalized field.
	roleName, err := r.fetchRoleName(ctx, result.RoleID)
	if err != nil {
		return nil, fmt.Errorf("membership: Create fetch role: %w", err)
	}
	result.RoleName = roleName

	return result, nil
}

// ─── FindByID ────────────────────────────────────────────────────────────────

// FindByID returns the membership with the given ID, or nil if not found or soft-deleted.
func (r *PostgresMembershipRepository) FindByID(ctx context.Context, id uuid.UUID) (*iam.Membership, error) {
	const q = `
		SELECT m.id, m.user_id, m.org_id, m.role_id, ro.name AS role_name,
		       m.status, m.invited_by, m.joined_at,
		       m.created_at, m.updated_at, m.deleted_at
		FROM memberships m
		JOIN roles ro ON ro.id = m.role_id
		WHERE m.id = $1 AND m.deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanMembershipWithRole(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("membership: FindByID: %w", err)
	}
	return result, nil
}

// ─── ListByOrg ───────────────────────────────────────────────────────────────

// ListByOrg returns all non-deleted memberships for the given org.
func (r *PostgresMembershipRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]iam.Membership, error) {
	const q = `
		SELECT m.id, m.user_id, m.org_id, m.role_id, ro.name AS role_name,
		       m.status, m.invited_by, m.joined_at,
		       m.created_at, m.updated_at, m.deleted_at
		FROM memberships m
		JOIN roles ro ON ro.id = m.role_id
		WHERE m.org_id = $1 AND m.deleted_at IS NULL
		ORDER BY m.created_at`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("membership: ListByOrg: %w", err)
	}
	defer rows.Close()

	return collectMemberships(rows, "ListByOrg")
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists changes to role_id and status and returns the updated row with role_name.
func (r *PostgresMembershipRepository) Update(ctx context.Context, m *iam.Membership) (*iam.Membership, error) {
	const q = `
		UPDATE memberships SET
			role_id    = $1,
			status     = $2,
			updated_at = now()
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, user_id, org_id, role_id, status, invited_by, joined_at,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q, m.RoleID, m.Status, m.ID)
	result, err := scanMembership(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("membership: Update: membership %s not found or already deleted", m.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("membership: Update: %w", err)
	}

	roleName, err := r.fetchRoleName(ctx, result.RoleID)
	if err != nil {
		return nil, fmt.Errorf("membership: Update fetch role: %w", err)
	}
	result.RoleName = roleName

	return result, nil
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

// SoftDelete marks a membership as deleted without removing the row.
func (r *PostgresMembershipRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE memberships SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("membership: SoftDelete: %w", err)
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// fetchRoleName retrieves the name of a role by its ID.
func (r *PostgresMembershipRepository) fetchRoleName(ctx context.Context, roleID uuid.UUID) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT name FROM roles WHERE id = $1`, roleID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("fetch role name: %w", err)
	}
	return name, nil
}

// scanMembership reads a membership row without the role_name column.
func scanMembership(row pgx.Row) (*iam.Membership, error) {
	var m iam.Membership
	err := row.Scan(
		&m.ID,
		&m.UserID,
		&m.OrgID,
		&m.RoleID,
		&m.Status,
		&m.InvitedBy,
		&m.JoinedAt,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// scanMembershipWithRole reads a membership row that includes the role_name JOIN column.
func scanMembershipWithRole(row pgx.Row) (*iam.Membership, error) {
	var m iam.Membership
	err := row.Scan(
		&m.ID,
		&m.UserID,
		&m.OrgID,
		&m.RoleID,
		&m.RoleName,
		&m.Status,
		&m.InvitedBy,
		&m.JoinedAt,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// collectMemberships drains a pgx.Rows into a slice of Membership values (with role_name).
func collectMemberships(rows pgx.Rows, op string) ([]iam.Membership, error) {
	var memberships []iam.Membership
	for rows.Next() {
		var m iam.Membership
		if err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.OrgID,
			&m.RoleID,
			&m.RoleName,
			&m.Status,
			&m.InvitedBy,
			&m.JoinedAt,
			&m.CreatedAt,
			&m.UpdatedAt,
			&m.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("membership: %s scan: %w", op, err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("membership: %s rows: %w", op, err)
	}
	return memberships, nil
}
