package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// EntitlementChecker verifies whether an org has access to a feature.
type EntitlementChecker interface {
	Check(ctx context.Context, orgID uuid.UUID, featureKey string) (allowed bool, remaining int, err error)
}

// RequireEntitlement creates middleware that checks if the org has the given feature entitlement.
// Returns 403 with a descriptive message if the feature is not available on the org's plan.
// If no org ID is present in context (user-scoped routes), the check is skipped.
func RequireEntitlement(checker EntitlementChecker, featureKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgID := OrgIDFromContext(r.Context())
			if orgID == uuid.Nil {
				// No org context — skip entitlement check (user-scoped routes)
				next.ServeHTTP(w, r)
				return
			}

			allowed, _, err := checker.Check(r.Context(), orgID, featureKey)
			if err != nil {
				api.WriteError(w, api.NewInternalError(err))
				return
			}
			if !allowed {
				api.WriteError(w, api.NewForbiddenError("feature '"+featureKey+"' is not available on your current plan"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
