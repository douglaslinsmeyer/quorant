package org

import (
	"context"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/iam"
)

// Service defines the business operations for the org module.
// Handlers depend on this interface rather than the concrete OrgService struct.
type Service interface {
	// Organizations
	CreateOrganization(ctx context.Context, req CreateOrgRequest) (*Organization, error)
	GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error)
	ListOrganizations(ctx context.Context, limit int, afterID *uuid.UUID) ([]Organization, bool, error)
	UpdateOrganization(ctx context.Context, id uuid.UUID, req UpdateOrgRequest) (*Organization, error)
	DeleteOrganization(ctx context.Context, id uuid.UUID) error
	ListChildren(ctx context.Context, parentID uuid.UUID) ([]Organization, error)
	ConnectManagement(ctx context.Context, hoaOrgID uuid.UUID, req ConnectManagementRequest) (*OrgManagement, error)
	DisconnectManagement(ctx context.Context, hoaOrgID uuid.UUID) error
	GetManagementHistory(ctx context.Context, hoaOrgID uuid.UUID) ([]OrgManagement, error)

	// Memberships
	CreateMembership(ctx context.Context, orgID uuid.UUID, req CreateMembershipRequest) (*iam.Membership, error)
	ListMemberships(ctx context.Context, orgID uuid.UUID) ([]iam.Membership, error)
	UpdateMembership(ctx context.Context, id uuid.UUID, roleID *uuid.UUID, status *string) (*iam.Membership, error)
	FindMembership(ctx context.Context, id uuid.UUID) (*iam.Membership, error)
	DeleteMembership(ctx context.Context, id uuid.UUID) error

	// Units
	CreateUnit(ctx context.Context, orgID uuid.UUID, req CreateUnitRequest) (*Unit, error)
	GetUnit(ctx context.Context, id uuid.UUID) (*Unit, error)
	ListUnits(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Unit, bool, error)
	UpdateUnit(ctx context.Context, id uuid.UUID, req UpdateUnitRequest) (*Unit, error)
	DeleteUnit(ctx context.Context, id uuid.UUID) error
	GetProperty(ctx context.Context, unitID uuid.UUID) (*Property, error)
	SetProperty(ctx context.Context, unitID uuid.UUID, prop *Property) (*Property, error)
	CreateUnitMembership(ctx context.Context, unitID uuid.UUID, req CreateUnitMembershipRequest) (*UnitMembership, error)
	ListUnitMemberships(ctx context.Context, unitID uuid.UUID) ([]UnitMembership, error)
	UpdateUnitMembership(ctx context.Context, id uuid.UUID, req UpdateUnitMembershipRequest) (*UnitMembership, error)
	EndUnitMembership(ctx context.Context, id uuid.UUID) error
	TransferOwnership(ctx context.Context, orgID uuid.UUID, unitID uuid.UUID, recordedBy uuid.UUID, req TransferOwnershipRequest) (*UnitOwnershipHistory, error)
	GetOwnershipHistory(ctx context.Context, unitID uuid.UUID) ([]UnitOwnershipHistory, error)

	// Amenities
	CreateAmenity(ctx context.Context, orgID uuid.UUID, req CreateAmenityRequest) (*Amenity, error)
	GetAmenity(ctx context.Context, id uuid.UUID) (*Amenity, error)
	ListAmenities(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Amenity, bool, error)
	UpdateAmenity(ctx context.Context, id uuid.UUID, req UpdateAmenityRequest) (*Amenity, error)
	DeleteAmenity(ctx context.Context, id uuid.UUID) error
	CreateReservation(ctx context.Context, orgID, amenityID uuid.UUID, req CreateReservationRequest) (*AmenityReservation, error)
	ListAmenityReservations(ctx context.Context, amenityID uuid.UUID) ([]AmenityReservation, error)
	ListUserReservations(ctx context.Context, orgID, userID uuid.UUID) ([]AmenityReservation, error)
	GetReservation(ctx context.Context, id uuid.UUID) (*AmenityReservation, error)
	UpdateReservation(ctx context.Context, id uuid.UUID, req UpdateReservationRequest) (*AmenityReservation, error)

	// Vendors
	CreateVendor(ctx context.Context, req CreateVendorRequest) (*Vendor, error)
	GetVendor(ctx context.Context, id uuid.UUID) (*Vendor, error)
	ListVendors(ctx context.Context, limit int, afterID *uuid.UUID) ([]Vendor, bool, error)
	UpdateVendor(ctx context.Context, id uuid.UUID, req UpdateVendorRequest) (*Vendor, error)
	DeleteVendor(ctx context.Context, id uuid.UUID) error
	CreateVendorAssignment(ctx context.Context, orgID uuid.UUID, req CreateVendorAssignmentRequest) (*VendorAssignment, error)
	ListVendorAssignments(ctx context.Context, orgID uuid.UUID) ([]VendorAssignment, error)
	DeleteVendorAssignment(ctx context.Context, id uuid.UUID) error

	// Registrations
	CreateRegistrationType(ctx context.Context, orgID uuid.UUID, req CreateRegistrationTypeRequest) (*RegistrationType, error)
	ListRegistrationTypes(ctx context.Context, orgID uuid.UUID) ([]RegistrationType, error)
	UpdateRegistrationType(ctx context.Context, id uuid.UUID, req UpdateRegistrationTypeRequest) (*RegistrationType, error)
	CreateRegistration(ctx context.Context, orgID, unitID uuid.UUID, req CreateRegistrationRequest) (*Registration, error)
	ListRegistrations(ctx context.Context, unitID uuid.UUID) ([]Registration, error)
	GetRegistration(ctx context.Context, id uuid.UUID) (*Registration, error)
	UpdateRegistration(ctx context.Context, id uuid.UUID, req UpdateRegistrationRequest) (*Registration, error)
	ApproveRegistration(ctx context.Context, id, approverID uuid.UUID) (*Registration, error)
	RevokeRegistration(ctx context.Context, id uuid.UUID) error
}
