package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// TenantContext extracts {org_id} from the URL path and stores it in the request context.
// It also stores the user ID from auth claims for RLS session variables.
//
// This middleware does NOT set PG session variables directly (that happens at the
// repository/query layer within a transaction). It only prepares the context.
//
// The architecture doc specifies RLS variables are set via:
//
//	SET app.current_user_id = '...';
//	SET app.current_org_id = '...';
//
// This is done per-transaction by the data access layer, not by middleware.
func TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract org_id from URL path parameter.
		// Go 1.22+ ServeMux supports path parameters via r.PathValue("org_id").
		orgIDStr := r.PathValue("org_id")
		if orgIDStr == "" {
			// No org_id in this route — pass through (some routes don't have org context).
			next.ServeHTTP(w, r)
			return
		}

		orgID, err := uuid.Parse(orgIDStr)
		if err != nil {
			api.WriteError(w, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id")))
			return
		}

		// Store org ID in context using the helper from rbac.go.
		ctx := WithOrgID(r.Context(), orgID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
