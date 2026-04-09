package org

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all org module routes on the mux.
// All routes require authentication via the auth middleware.
// Routes with {org_id} also get tenant context middleware.
func RegisterRoutes(mux *http.ServeMux, orgHandler *OrgHandler, membershipHandler *MembershipHandler, unitHandler *UnitHandler, validator auth.TokenValidator) {
	// Helper to wrap a handler with auth middleware
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(h))
	}

	// Helper to wrap with auth + tenant context
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}

	// Organization endpoints
	mux.Handle("POST /api/v1/organizations", authMw(orgHandler.CreateOrg))
	mux.Handle("GET /api/v1/organizations", authMw(orgHandler.ListOrgs))
	mux.Handle("GET /api/v1/organizations/{org_id}", orgMw(orgHandler.GetOrg))
	mux.Handle("PATCH /api/v1/organizations/{org_id}", orgMw(orgHandler.UpdateOrg))
	mux.Handle("DELETE /api/v1/organizations/{org_id}", orgMw(orgHandler.DeleteOrg))
	mux.Handle("GET /api/v1/organizations/{org_id}/children", orgMw(orgHandler.ListChildren))
	mux.Handle("POST /api/v1/organizations/{org_id}/management", orgMw(orgHandler.ConnectManagement))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/management", orgMw(orgHandler.DisconnectManagement))
	mux.Handle("GET /api/v1/organizations/{org_id}/management/history", orgMw(orgHandler.GetManagementHistory))

	// Membership endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/memberships", orgMw(membershipHandler.CreateMembership))
	mux.Handle("GET /api/v1/organizations/{org_id}/memberships", orgMw(membershipHandler.ListMemberships))
	mux.Handle("GET /api/v1/organizations/{org_id}/memberships/{membership_id}", orgMw(membershipHandler.GetMembership))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/memberships/{membership_id}", orgMw(membershipHandler.UpdateMembership))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/memberships/{membership_id}", orgMw(membershipHandler.DeleteMembership))

	// Unit endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/units", orgMw(unitHandler.CreateUnit))
	mux.Handle("GET /api/v1/organizations/{org_id}/units", orgMw(unitHandler.ListUnits))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}", orgMw(unitHandler.GetUnit))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/units/{unit_id}", orgMw(unitHandler.UpdateUnit))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/units/{unit_id}", orgMw(unitHandler.DeleteUnit))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/property", orgMw(unitHandler.GetProperty))
	mux.Handle("PUT /api/v1/organizations/{org_id}/units/{unit_id}/property", orgMw(unitHandler.SetProperty))
	mux.Handle("POST /api/v1/organizations/{org_id}/units/{unit_id}/memberships", orgMw(unitHandler.CreateUnitMembership))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/memberships", orgMw(unitHandler.ListUnitMemberships))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}", orgMw(unitHandler.UpdateUnitMembership))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}", orgMw(unitHandler.EndUnitMembership))
}
