package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/platform/api"
)

// ─── Context helpers ─────────────────────────────────────────────────────────

type orgContextKey struct{}

// WithOrgID stores the org ID in context.
func WithOrgID(ctx context.Context, orgID uuid.UUID) context.Context {
	return context.WithValue(ctx, orgContextKey{}, orgID)
}

// OrgIDFromContext retrieves the org ID from context.
// Returns uuid.Nil if no org ID is present.
func OrgIDFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(orgContextKey{}).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// ─── PermissionChecker interface ─────────────────────────────────────────────

// PermissionChecker resolves whether a user has a permission for an org.
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID uuid.UUID, orgID uuid.UUID, permission string) (bool, error)
}

// ─── PostgresPermissionChecker ───────────────────────────────────────────────

// PostgresPermissionChecker resolves permissions from the database.
type PostgresPermissionChecker struct {
	pool *pgxpool.Pool
}

// NewPostgresPermissionChecker creates a new PostgresPermissionChecker.
func NewPostgresPermissionChecker(pool *pgxpool.Pool) *PostgresPermissionChecker {
	return &PostgresPermissionChecker{pool: pool}
}

// HasPermission checks if the user has the given permission for the target org.
//
// Resolution order:
//  1. Direct membership in target org with a role that has the permission
//  2. Membership in an ancestor firm org (via ltree) with a role that has the permission
//  3. Membership in a firm that manages the target org (via organizations_management)
//  4. Platform roles (platform_admin, platform_support, platform_finance) — global scope
func (c *PostgresPermissionChecker) HasPermission(ctx context.Context, userID uuid.UUID, orgID uuid.UUID, permission string) (bool, error) {
	const query = `
SELECT EXISTS(
  SELECT 1
  FROM memberships m
  JOIN role_permissions rp ON m.role_id = rp.role_id
  JOIN permissions p ON rp.permission_id = p.id
  LEFT JOIN organizations target ON target.id = $2
  LEFT JOIN organizations member_org ON member_org.id = m.org_id
  WHERE m.user_id = $1
    AND m.deleted_at IS NULL
    AND m.status = 'active'
    AND p.key = $3
    AND (
      -- 1. Direct membership in target org
      m.org_id = $2

      -- 2. Ancestor firm org via ltree (firm hierarchy only)
      OR (
        target.path <@ member_org.path
        AND member_org.type = 'firm'
        AND target.type = 'firm'
      )

      -- 3. Firm manages target HOA via organizations_management
      OR EXISTS (
        SELECT 1 FROM organizations_management om
        WHERE om.hoa_org_id = $2
          AND om.ended_at IS NULL
          AND (
            om.firm_org_id = m.org_id
            OR EXISTS (
              SELECT 1 FROM organizations firm
              WHERE firm.id = om.firm_org_id
                AND firm.path <@ member_org.path
            )
          )
      )

      -- 4. Platform roles — global scope (role name starts with 'platform_')
      OR EXISTS (
        SELECT 1 FROM roles r
        WHERE r.id = m.role_id
          AND r.name LIKE 'platform_%'
      )
    )
)`

	var allowed bool
	err := c.pool.QueryRow(ctx, query, userID, orgID, permission).Scan(&allowed)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

// ─── RequirePermission middleware ─────────────────────────────────────────────

// RequirePermission creates middleware that checks the given permission against
// the org ID stored in context.
//
// It expects:
//   - Org ID in context (from tenant middleware, via WithOrgID)
//   - A resolveUserID function that maps the request context to a user UUID
//     (typically derived from JWT claims → users lookup)
func RequirePermission(
	checker PermissionChecker,
	permission string,
	resolveUserID func(ctx context.Context) (uuid.UUID, error),
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgID := OrgIDFromContext(r.Context())
			if orgID == uuid.Nil {
				api.WriteError(w, api.NewForbiddenError("no organization context"))
				return
			}

			userID, err := resolveUserID(r.Context())
			if err != nil {
				api.WriteError(w, api.NewUnauthenticatedError("could not resolve user"))
				return
			}

			allowed, err := checker.HasPermission(r.Context(), userID, orgID, permission)
			if err != nil {
				api.WriteError(w, api.NewInternalError(err))
				return
			}

			if !allowed {
				api.WriteError(w, api.NewForbiddenError("insufficient permissions"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
