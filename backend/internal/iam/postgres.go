package iam

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresUserRepository implements UserRepository using a pgxpool.
type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresUserRepository creates a new PostgresUserRepository backed by pool.
func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

// FindByIDPUserID looks up a user by their identity provider ID.
// Returns nil, nil when no user exists with the given idpUserID.
func (r *PostgresUserRepository) FindByIDPUserID(ctx context.Context, idpUserID string) (*User, error) {
	const q = `
		SELECT id, idp_user_id, email, display_name, phone, avatar_url,
		       is_active, last_login_at, created_at, updated_at, deleted_at
		FROM users
		WHERE idp_user_id = $1`

	row := r.pool.QueryRow(ctx, q, idpUserID)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("iam: FindByIDPUserID: %w", err)
	}
	return user, nil
}

// FindByID looks up a user by their internal UUID.
// Returns nil, nil when no user exists with the given id.
func (r *PostgresUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `
		SELECT id, idp_user_id, email, display_name, phone, avatar_url,
		       is_active, last_login_at, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("iam: FindByID: %w", err)
	}
	return user, nil
}

// Upsert inserts a new user or updates an existing one matched by idp_user_id.
// On conflict, email, display_name, and updated_at are updated.
// Returns the full persisted user record.
func (r *PostgresUserRepository) Upsert(ctx context.Context, user *User) (*User, error) {
	const q = `
		INSERT INTO users (idp_user_id, email, display_name, phone, avatar_url, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (idp_user_id) DO UPDATE
			SET email        = EXCLUDED.email,
			    display_name = EXCLUDED.display_name,
			    updated_at   = now()
		RETURNING id, idp_user_id, email, display_name, phone, avatar_url,
		          is_active, last_login_at, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		user.IDPUserID,
		user.Email,
		user.DisplayName,
		user.Phone,
		user.AvatarURL,
		user.IsActive,
	)
	result, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("iam: Upsert: %w", err)
	}
	return result, nil
}

// UpdateLastLogin sets last_login_at to the current time for the given user ID.
func (r *PostgresUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET last_login_at = $1, updated_at = now() WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("iam: UpdateLastLogin: %w", err)
	}
	return nil
}

// FindMembershipsByUserID returns all active (non-deleted) memberships for the
// given user, with the role name joined from the roles table.
func (r *PostgresUserRepository) FindMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]Membership, error) {
	const q = `
		SELECT m.id, m.user_id, m.org_id, m.role_id, ro.name,
		       m.status, m.invited_by, m.joined_at, m.created_at, m.updated_at, m.deleted_at
		FROM memberships m
		JOIN roles ro ON ro.id = m.role_id
		WHERE m.user_id = $1
		  AND m.deleted_at IS NULL`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("iam: FindMembershipsByUserID: %w", err)
	}
	defer rows.Close()

	var memberships []Membership
	for rows.Next() {
		var m Membership
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
			return nil, fmt.Errorf("iam: FindMembershipsByUserID scan: %w", err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iam: FindMembershipsByUserID rows: %w", err)
	}
	return memberships, nil
}

// scanUser scans a single row into a User struct.
func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID,
		&u.IDPUserID,
		&u.Email,
		&u.DisplayName,
		&u.Phone,
		&u.AvatarURL,
		&u.IsActive,
		&u.LastLoginAt,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
