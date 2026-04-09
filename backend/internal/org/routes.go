package org

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all org module routes on the mux.
// All routes require authentication via the auth middleware.
// Routes with {org_id} also get tenant context middleware.
func RegisterRoutes(
	mux *http.ServeMux,
	orgHandler *OrgHandler,
	membershipHandler *MembershipHandler,
	unitHandler *UnitHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	// Helper to wrap a handler with auth middleware only
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(h))
	}

	// Helper to wrap with auth + tenant context + permission check
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	// Organization endpoints
	mux.Handle("POST /api/v1/organizations", permMw("org.organization.create", orgHandler.CreateOrg))
	mux.Handle("GET /api/v1/organizations", authMw(orgHandler.ListOrgs)) // auth only — lists user's own orgs
	mux.Handle("GET /api/v1/organizations/{org_id}", permMw("org.organization.read", orgHandler.GetOrg))
	mux.Handle("PATCH /api/v1/organizations/{org_id}", permMw("org.organization.update", orgHandler.UpdateOrg))
	mux.Handle("DELETE /api/v1/organizations/{org_id}", permMw("org.organization.delete", orgHandler.DeleteOrg))
	mux.Handle("GET /api/v1/organizations/{org_id}/children", permMw("org.organization.read", orgHandler.ListChildren))
	mux.Handle("POST /api/v1/organizations/{org_id}/management", permMw("org.organization.update", orgHandler.ConnectManagement))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/management", permMw("org.organization.update", orgHandler.DisconnectManagement))
	mux.Handle("GET /api/v1/organizations/{org_id}/management/history", permMw("org.organization.read", orgHandler.GetManagementHistory))

	// Membership endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/memberships", permMw("org.membership.manage", membershipHandler.CreateMembership))
	mux.Handle("GET /api/v1/organizations/{org_id}/memberships", permMw("org.membership.read", membershipHandler.ListMemberships))
	mux.Handle("GET /api/v1/organizations/{org_id}/memberships/{membership_id}", permMw("org.membership.read", membershipHandler.GetMembership))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/memberships/{membership_id}", permMw("org.membership.manage", membershipHandler.UpdateMembership))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/memberships/{membership_id}", permMw("org.membership.manage", membershipHandler.DeleteMembership))

	// Unit endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/units", permMw("org.unit.create", unitHandler.CreateUnit))
	mux.Handle("GET /api/v1/organizations/{org_id}/units", permMw("org.unit.read", unitHandler.ListUnits))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}", permMw("org.unit.read", unitHandler.GetUnit))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/units/{unit_id}", permMw("org.unit.update", unitHandler.UpdateUnit))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/units/{unit_id}", permMw("org.unit.update", unitHandler.DeleteUnit))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/property", permMw("org.unit.read", unitHandler.GetProperty))
	mux.Handle("PUT /api/v1/organizations/{org_id}/units/{unit_id}/property", permMw("org.unit.update", unitHandler.SetProperty))
	mux.Handle("POST /api/v1/organizations/{org_id}/units/{unit_id}/memberships", permMw("org.unit_membership.manage", unitHandler.CreateUnitMembership))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/memberships", permMw("org.unit_membership.read", unitHandler.ListUnitMemberships))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}", permMw("org.unit_membership.manage", unitHandler.UpdateUnitMembership))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}", permMw("org.unit_membership.manage", unitHandler.EndUnitMembership))
}
