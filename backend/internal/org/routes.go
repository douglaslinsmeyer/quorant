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
	amenityHandler *AmenityHandler,
	vendorHandler *VendorHandler,
	registrationHandler *RegistrationHandler,
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
	mux.Handle("POST /api/v1/organizations/{org_id}/units/{unit_id}/transfer", permMw("org.unit.update", unitHandler.TransferOwnership))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/ownership-history", permMw("org.unit.read", unitHandler.GetOwnershipHistory))

	// Amenity endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/amenities", permMw("org.amenity.manage", amenityHandler.CreateAmenity))
	mux.Handle("GET /api/v1/organizations/{org_id}/amenities", permMw("org.amenity.read", amenityHandler.ListAmenities))
	mux.Handle("GET /api/v1/organizations/{org_id}/amenities/{amenity_id}", permMw("org.amenity.read", amenityHandler.GetAmenity))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/amenities/{amenity_id}", permMw("org.amenity.manage", amenityHandler.UpdateAmenity))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/amenities/{amenity_id}", permMw("org.amenity.manage", amenityHandler.DeleteAmenity))

	// Reservation endpoints under amenities
	mux.Handle("POST /api/v1/organizations/{org_id}/amenities/{amenity_id}/reservations", permMw("org.reservation.create", amenityHandler.CreateReservation))
	mux.Handle("GET /api/v1/organizations/{org_id}/amenities/{amenity_id}/reservations", permMw("org.reservation.read", amenityHandler.ListAmenityReservations))

	// Reservation endpoints at org level
	mux.Handle("GET /api/v1/organizations/{org_id}/reservations", permMw("org.reservation.read", amenityHandler.ListOrgReservations))
	mux.Handle("GET /api/v1/organizations/{org_id}/reservations/{reservation_id}", permMw("org.reservation.read", amenityHandler.GetReservation))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/reservations/{reservation_id}", permMw("org.reservation.manage", amenityHandler.UpdateReservation))

	// Registration type endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/registration-types", permMw("org.registration_type.manage", registrationHandler.CreateRegistrationType))
	mux.Handle("GET /api/v1/organizations/{org_id}/registration-types", permMw("org.registration_type.manage", registrationHandler.ListRegistrationTypes))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/registration-types/{id}", permMw("org.registration_type.manage", registrationHandler.UpdateRegistrationType))

	// Registration endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/units/{unit_id}/registrations", permMw("org.registration.create", registrationHandler.CreateRegistration))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/registrations", permMw("org.registration.read", registrationHandler.ListRegistrations))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/registrations/{id}", permMw("org.registration.manage", registrationHandler.UpdateRegistration))
	mux.Handle("POST /api/v1/organizations/{org_id}/registrations/{id}/approve", permMw("org.registration.manage", registrationHandler.ApproveRegistration))
	mux.Handle("POST /api/v1/organizations/{org_id}/registrations/{id}/revoke", permMw("org.registration.manage", registrationHandler.RevokeRegistration))

	// Vendor endpoints (top-level)
	mux.Handle("POST /api/v1/vendors", permMw("org.vendor.manage", vendorHandler.CreateVendor))
	mux.Handle("GET /api/v1/vendors", permMw("org.vendor.read", vendorHandler.ListVendors))
	mux.Handle("GET /api/v1/vendors/{vendor_id}", permMw("org.vendor.read", vendorHandler.GetVendor))
	mux.Handle("PATCH /api/v1/vendors/{vendor_id}", permMw("org.vendor.manage", vendorHandler.UpdateVendor))
	mux.Handle("DELETE /api/v1/vendors/{vendor_id}", permMw("org.vendor.manage", vendorHandler.DeleteVendor))

	// Vendor assignment endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/vendor-assignments", permMw("org.vendor.manage", vendorHandler.CreateVendorAssignment))
	mux.Handle("GET /api/v1/organizations/{org_id}/vendor-assignments", permMw("org.vendor.read", vendorHandler.ListVendorAssignments))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/vendor-assignments/{id}", permMw("org.vendor.manage", vendorHandler.DeleteVendorAssignment))
}
