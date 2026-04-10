package org

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/queue"
)

// OrgService provides business logic for organization operations.
type OrgService struct {
	orgRepo          OrgRepository
	membershipRepo   MembershipRepository
	unitRepo         UnitRepository
	amenityRepo      AmenityRepository
	vendorRepo       VendorRepository
	registrationRepo RegistrationRepository
	userRepo         UserFinder
	auditor          audit.Auditor
	publisher        queue.Publisher
	logger           *slog.Logger
}

// NewOrgService constructs an OrgService backed by the given repositories.
func NewOrgService(
	orgRepo OrgRepository,
	membershipRepo MembershipRepository,
	unitRepo UnitRepository,
	userRepo UserFinder,
	auditor audit.Auditor,
	publisher queue.Publisher,
	logger *slog.Logger,
) *OrgService {
	return &OrgService{
		orgRepo:        orgRepo,
		membershipRepo: membershipRepo,
		unitRepo:       unitRepo,
		userRepo:       userRepo,
		auditor:        auditor,
		publisher:      publisher,
		logger:         logger,
	}
}

// WithAmenityRepo sets the amenity repository on the service (optional).
func (s *OrgService) WithAmenityRepo(repo AmenityRepository) *OrgService {
	s.amenityRepo = repo
	return s
}

// WithVendorRepo sets the vendor repository on the service (optional).
func (s *OrgService) WithVendorRepo(repo VendorRepository) *OrgService {
	s.vendorRepo = repo
	return s
}

// WithRegistrationRepo sets the registration repository on the service (optional).
func (s *OrgService) WithRegistrationRepo(repo RegistrationRepository) *OrgService {
	s.registrationRepo = repo
	return s
}

// ─── Organization operations ─────────────────────────────────────────────────

// CreateOrganization validates the request and creates a new organization.
func (s *OrgService) CreateOrganization(ctx context.Context, req CreateOrgRequest) (*Organization, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	org := &Organization{
		Type:         req.Type,
		Name:         req.Name,
		ParentID:     req.ParentID,
		AddressLine1: req.AddressLine1,
		AddressLine2: req.AddressLine2,
		City:         req.City,
		State:        req.State,
		Jurisdiction: req.Jurisdiction,
		Zip:          req.Zip,
		Phone:        req.Phone,
		Email:        req.Email,
		Website:      req.Website,
		Locale:       req.Locale,
		Timezone:     req.Timezone,
		CurrencyCode: req.CurrencyCode,
		Country:      req.Country,
		Settings:     req.Settings,
	}
	if org.Settings == nil {
		org.Settings = map[string]any{}
	}

	created, err := s.orgRepo.Create(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateOrganization: %w", err)
	}

	s.logger.InfoContext(ctx, "organization created", "org_id", created.ID, "type", created.Type)
	return created, nil
}

// GetOrganization returns an organization by ID, or a NotFoundError if it does not exist.
func (s *OrgService) GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error) {
	org, err := s.orgRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: GetOrganization: %w", err)
	}
	if org == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "organization"), api.P("id", id.String()))
	}
	return org, nil
}

// ListOrganizations returns the organizations accessible by the authenticated user.
// It extracts the user from the JWT claims stored in the context.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *OrgService) ListOrganizations(ctx context.Context, limit int, afterID *uuid.UUID) ([]Organization, bool, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, false, api.NewUnauthenticatedError("auth.missing_claims")
	}

	user, err := s.userRepo.FindByIDPUserID(ctx, claims.Subject)
	if err != nil {
		return nil, false, fmt.Errorf("org service: ListOrganizations find user: %w", err)
	}
	if user == nil {
		return nil, false, api.NewNotFoundError("resource.not_found", api.P("resource", "user"))
	}

	orgs, hasMore, err := s.orgRepo.ListByUserAccess(ctx, user.ID, limit, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("org service: ListOrganizations: %w", err)
	}
	return orgs, hasMore, nil
}

// UpdateOrganization validates the request, applies changes, and persists the update.
func (s *OrgService) UpdateOrganization(ctx context.Context, id uuid.UUID, req UpdateOrgRequest) (*Organization, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	org, err := s.GetOrganization(ctx, id)
	if err != nil {
		return nil, err
	}

	// Apply partial updates.
	if req.Name != nil {
		org.Name = *req.Name
	}
	if req.AddressLine1 != nil {
		org.AddressLine1 = req.AddressLine1
	}
	if req.AddressLine2 != nil {
		org.AddressLine2 = req.AddressLine2
	}
	if req.City != nil {
		org.City = req.City
	}
	if req.State != nil {
		org.State = req.State
	}
	if req.Jurisdiction != nil {
		org.Jurisdiction = req.Jurisdiction
	}
	if req.Zip != nil {
		org.Zip = req.Zip
	}
	if req.Phone != nil {
		org.Phone = req.Phone
	}
	if req.Email != nil {
		org.Email = req.Email
	}
	if req.Website != nil {
		org.Website = req.Website
	}
	if req.LogoURL != nil {
		org.LogoURL = req.LogoURL
	}
	if req.Settings != nil {
		org.Settings = req.Settings
	}
	if req.Locale != nil {
		org.Locale = *req.Locale
	}
	if req.Timezone != nil {
		org.Timezone = *req.Timezone
	}
	if req.CurrencyCode != nil {
		org.CurrencyCode = *req.CurrencyCode
	}
	if req.Country != nil {
		org.Country = *req.Country
	}

	updated, err := s.orgRepo.Update(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateOrganization: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"org_id": updated.ID,
		"name":   updated.Name,
	})
	evt := queue.NewBaseEvent("quorant.org.OrganizationUpdated", "organization", updated.ID, updated.ID, payload)
	if err := s.publisher.Publish(ctx, evt); err != nil {
		s.logger.Error("failed to publish OrganizationUpdated", "org_id", updated.ID, "error", err)
	}

	s.logger.InfoContext(ctx, "organization updated", "org_id", updated.ID)
	return updated, nil
}

// DeleteOrganization soft-deletes an organization by ID.
func (s *OrgService) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	if err := s.orgRepo.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("org service: DeleteOrganization: %w", err)
	}
	s.logger.InfoContext(ctx, "organization deleted", "org_id", id)
	return nil
}

// ListChildren returns the direct child organizations of the given parent.
func (s *OrgService) ListChildren(ctx context.Context, parentID uuid.UUID) ([]Organization, error) {
	orgs, err := s.orgRepo.ListChildren(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("org service: ListChildren: %w", err)
	}
	return orgs, nil
}

// ─── Management operations ───────────────────────────────────────────────────

// ConnectManagement links a management firm to an HOA after validating both org types.
func (s *OrgService) ConnectManagement(ctx context.Context, hoaOrgID uuid.UUID, req ConnectManagementRequest) (*OrgManagement, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Validate HOA exists and is type "hoa".
	hoa, err := s.GetOrganization(ctx, hoaOrgID)
	if err != nil {
		return nil, err
	}
	if hoa.Type != "hoa" {
		return nil, api.NewValidationError(
			"validation.constraint", "hoa_org_id",
			api.P("field", "hoa_org_id"), api.P("constraint", "must be of type hoa"),
		)
	}

	// Validate firm exists and is type "firm".
	firm, err := s.GetOrganization(ctx, req.FirmOrgID)
	if err != nil {
		return nil, err
	}
	if firm.Type != "firm" {
		return nil, api.NewValidationError(
			"validation.constraint", "firm_org_id",
			api.P("field", "firm_org_id"), api.P("constraint", "must be of type firm"),
		)
	}

	mgmt, err := s.orgRepo.ConnectManagement(ctx, req.FirmOrgID, hoaOrgID)
	if err != nil {
		return nil, fmt.Errorf("org service: ConnectManagement: %w", err)
	}

	s.logger.InfoContext(ctx, "management connected", "hoa_org_id", hoaOrgID, "firm_org_id", req.FirmOrgID)
	return mgmt, nil
}

// DisconnectManagement ends the active management relationship for an HOA.
func (s *OrgService) DisconnectManagement(ctx context.Context, hoaOrgID uuid.UUID) error {
	if err := s.orgRepo.DisconnectManagement(ctx, hoaOrgID); err != nil {
		return fmt.Errorf("org service: DisconnectManagement: %w", err)
	}
	s.logger.InfoContext(ctx, "management disconnected", "hoa_org_id", hoaOrgID)
	return nil
}

// GetManagementHistory returns the full management history for an HOA.
func (s *OrgService) GetManagementHistory(ctx context.Context, hoaOrgID uuid.UUID) ([]OrgManagement, error) {
	history, err := s.orgRepo.ListManagementHistory(ctx, hoaOrgID)
	if err != nil {
		return nil, fmt.Errorf("org service: GetManagementHistory: %w", err)
	}
	return history, nil
}

// ─── Membership operations ───────────────────────────────────────────────────

// CreateMembership validates the request and creates a new org membership.
func (s *OrgService) CreateMembership(ctx context.Context, orgID uuid.UUID, req CreateMembershipRequest) (*iam.Membership, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	m := &iam.Membership{
		OrgID:  orgID,
		UserID: req.UserID,
		RoleID: req.RoleID,
		Status: "invited",
	}

	created, err := s.membershipRepo.Create(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateMembership: %w", err)
	}

	s.logger.InfoContext(ctx, "membership created", "membership_id", created.ID, "org_id", orgID, "user_id", req.UserID)
	return created, nil
}

// ListMemberships returns all memberships for the given org.
func (s *OrgService) ListMemberships(ctx context.Context, orgID uuid.UUID) ([]iam.Membership, error) {
	memberships, err := s.membershipRepo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("org service: ListMemberships: %w", err)
	}
	return memberships, nil
}

// UpdateMembership updates the role and/or status of an existing membership.
func (s *OrgService) UpdateMembership(ctx context.Context, id uuid.UUID, roleID *uuid.UUID, status *string) (*iam.Membership, error) {
	m, err := s.membershipRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateMembership find: %w", err)
	}
	if m == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "membership"), api.P("id", id.String()))
	}

	if roleID != nil {
		m.RoleID = *roleID
	}
	if status != nil {
		m.Status = *status
	}

	updated, err := s.membershipRepo.Update(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateMembership: %w", err)
	}

	s.logger.InfoContext(ctx, "membership updated", "membership_id", id)
	return updated, nil
}

// FindMembership returns a membership by ID, or a NotFoundError if it does not exist.
func (s *OrgService) FindMembership(ctx context.Context, id uuid.UUID) (*iam.Membership, error) {
	m, err := s.membershipRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: FindMembership: %w", err)
	}
	if m == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "membership"), api.P("id", id.String()))
	}
	return m, nil
}

// DeleteMembership soft-deletes a membership by ID.
func (s *OrgService) DeleteMembership(ctx context.Context, id uuid.UUID) error {
	if err := s.membershipRepo.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("org service: DeleteMembership: %w", err)
	}
	s.logger.InfoContext(ctx, "membership deleted", "membership_id", id)
	return nil
}

// ─── Unit operations ─────────────────────────────────────────────────────────

// CreateUnit validates the request and creates a new unit within an org.
func (s *OrgService) CreateUnit(ctx context.Context, orgID uuid.UUID, req CreateUnitRequest) (*Unit, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	votingWeight := 1.0
	if req.VotingWeight != nil {
		votingWeight = *req.VotingWeight
	}

	unit := &Unit{
		OrgID:        orgID,
		Label:        req.Label,
		UnitType:     req.UnitType,
		AddressLine1: req.AddressLine1,
		AddressLine2: req.AddressLine2,
		City:         req.City,
		State:        req.State,
		Zip:          req.Zip,
		Country:      req.Country,
		LotSizeSqft:  req.LotSizeSqft,
		VotingWeight: votingWeight,
		Status:       "active",
		Metadata:     req.Metadata,
	}
	if unit.Metadata == nil {
		unit.Metadata = map[string]any{}
	}

	created, err := s.unitRepo.CreateUnit(ctx, unit)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateUnit: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"org_id":  created.OrgID,
		"unit_id": created.ID,
	})
	evt := queue.NewBaseEvent("quorant.org.UnitCreated", "unit", created.ID, created.OrgID, payload)
	if err := s.publisher.Publish(ctx, evt); err != nil {
		s.logger.Error("failed to publish UnitCreated", "unit_id", created.ID, "error", err)
	}

	s.logger.InfoContext(ctx, "unit created", "unit_id", created.ID, "org_id", orgID)
	return created, nil
}

// GetUnit returns a unit by ID, or a NotFoundError if it does not exist.
func (s *OrgService) GetUnit(ctx context.Context, id uuid.UUID) (*Unit, error) {
	unit, err := s.unitRepo.FindUnitByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: GetUnit: %w", err)
	}
	if unit == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "unit"), api.P("id", id.String()))
	}
	return unit, nil
}

// ListUnits returns units belonging to the given org, supporting cursor-based pagination.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *OrgService) ListUnits(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Unit, bool, error) {
	units, hasMore, err := s.unitRepo.ListUnitsByOrg(ctx, orgID, limit, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("org service: ListUnits: %w", err)
	}
	return units, hasMore, nil
}

// UpdateUnit applies partial updates to a unit.
func (s *OrgService) UpdateUnit(ctx context.Context, id uuid.UUID, req UpdateUnitRequest) (*Unit, error) {
	label, unitType := req.Label, req.UnitType
	addressLine1, addressLine2 := req.AddressLine1, req.AddressLine2
	city, state, zip := req.City, req.State, req.Zip
	lotSizeSqft, votingWeight, status, metadata := req.LotSizeSqft, req.VotingWeight, req.Status, req.Metadata
	_ = metadata // used below
	unit, err := s.GetUnit(ctx, id)
	if err != nil {
		return nil, err
	}

	if label != nil {
		unit.Label = *label
	}
	if unitType != nil {
		unit.UnitType = unitType
	}
	if addressLine1 != nil {
		unit.AddressLine1 = addressLine1
	}
	if addressLine2 != nil {
		unit.AddressLine2 = addressLine2
	}
	if city != nil {
		unit.City = city
	}
	if state != nil {
		unit.State = state
	}
	if zip != nil {
		unit.Zip = zip
	}
	if lotSizeSqft != nil {
		unit.LotSizeSqft = lotSizeSqft
	}
	if votingWeight != nil {
		unit.VotingWeight = *votingWeight
	}
	if status != nil {
		unit.Status = *status
	}
	if metadata != nil {
		unit.Metadata = metadata
	}

	updated, err := s.unitRepo.UpdateUnit(ctx, unit)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateUnit: %w", err)
	}

	s.logger.InfoContext(ctx, "unit updated", "unit_id", id)
	return updated, nil
}

// DeleteUnit soft-deletes a unit by ID.
func (s *OrgService) DeleteUnit(ctx context.Context, id uuid.UUID) error {
	unit, err := s.unitRepo.FindUnitByID(ctx, id)
	if err != nil {
		return fmt.Errorf("org service: DeleteUnit: %w", err)
	}

	if err := s.unitRepo.SoftDeleteUnit(ctx, id); err != nil {
		return fmt.Errorf("org service: DeleteUnit: %w", err)
	}

	if unit != nil {
		payload, _ := json.Marshal(map[string]any{
			"org_id":  unit.OrgID,
			"unit_id": id,
		})
		evt := queue.NewBaseEvent("quorant.org.UnitDeleted", "unit", id, unit.OrgID, payload)
		if err := s.publisher.Publish(ctx, evt); err != nil {
			s.logger.Error("failed to publish UnitDeleted", "unit_id", id, "error", err)
		}
	}

	s.logger.InfoContext(ctx, "unit deleted", "unit_id", id)
	return nil
}

// ─── Property operations ─────────────────────────────────────────────────────

// GetProperty returns the property record for a unit, or a NotFoundError if none exists.
func (s *OrgService) GetProperty(ctx context.Context, unitID uuid.UUID) (*Property, error) {
	prop, err := s.unitRepo.GetProperty(ctx, unitID)
	if err != nil {
		return nil, fmt.Errorf("org service: GetProperty: %w", err)
	}
	if prop == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "property"), api.P("id", unitID.String()))
	}
	return prop, nil
}

// SetProperty upserts (creates or updates) the property record for a unit.
func (s *OrgService) SetProperty(ctx context.Context, unitID uuid.UUID, prop *Property) (*Property, error) {
	prop.UnitID = unitID

	saved, err := s.unitRepo.UpsertProperty(ctx, prop)
	if err != nil {
		return nil, fmt.Errorf("org service: SetProperty: %w", err)
	}

	s.logger.InfoContext(ctx, "property set", "unit_id", unitID)
	return saved, nil
}

// ─── Unit Membership operations ──────────────────────────────────────────────

// CreateUnitMembership validates the request and creates a new unit membership.
func (s *OrgService) CreateUnitMembership(ctx context.Context, unitID uuid.UUID, req CreateUnitMembershipRequest) (*UnitMembership, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	um := &UnitMembership{
		UnitID:       unitID,
		UserID:       req.UserID,
		Relationship: req.Relationship,
		IsVoter:      req.IsVoter,
		Notes:        req.Notes,
	}

	created, err := s.unitRepo.CreateUnitMembership(ctx, um)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateUnitMembership: %w", err)
	}

	s.logger.InfoContext(ctx, "unit membership created", "unit_id", unitID, "user_id", req.UserID)
	return created, nil
}

// ListUnitMemberships returns all memberships for the given unit.
func (s *OrgService) ListUnitMemberships(ctx context.Context, unitID uuid.UUID) ([]UnitMembership, error) {
	memberships, err := s.unitRepo.ListUnitMemberships(ctx, unitID)
	if err != nil {
		return nil, fmt.Errorf("org service: ListUnitMemberships: %w", err)
	}
	return memberships, nil
}

// UpdateUnitMembership applies partial updates to a unit membership.
func (s *OrgService) UpdateUnitMembership(ctx context.Context, id uuid.UUID, req UpdateUnitMembershipRequest) (*UnitMembership, error) {
	um, err := s.unitRepo.FindUnitMembershipByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateUnitMembership find: %w", err)
	}
	if um == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "unit_membership"), api.P("id", id.String()))
	}

	if req.Relationship != nil {
		um.Relationship = *req.Relationship
	}
	if req.IsVoter != nil {
		um.IsVoter = *req.IsVoter
	}
	if req.Notes != nil {
		um.Notes = req.Notes
	}

	updated, err := s.unitRepo.UpdateUnitMembership(ctx, um)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateUnitMembership: %w", err)
	}

	s.logger.InfoContext(ctx, "unit membership updated", "unit_membership_id", id)
	return updated, nil
}

// EndUnitMembership ends a unit membership by setting its ended_at timestamp.
func (s *OrgService) EndUnitMembership(ctx context.Context, id uuid.UUID) error {
	if err := s.unitRepo.EndUnitMembership(ctx, id); err != nil {
		return fmt.Errorf("org service: EndUnitMembership: %w", err)
	}
	s.logger.InfoContext(ctx, "unit membership ended", "unit_membership_id", id)
	return nil
}

// ─── Ownership History operations ────────────────────────────────────────────

// TransferOwnership validates the request and records an ownership transfer for a unit.
func (s *OrgService) TransferOwnership(ctx context.Context, orgID uuid.UUID, unitID uuid.UUID, recordedBy uuid.UUID, req TransferOwnershipRequest) (*UnitOwnershipHistory, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	h := &UnitOwnershipHistory{
		UnitID:                  unitID,
		OrgID:                   orgID,
		FromUserID:              req.FromUserID,
		ToUserID:                req.ToUserID,
		TransferType:            req.TransferType,
		TransferDate:            req.TransferDate,
		SalePriceCents:          req.SalePriceCents,
		OutstandingBalanceCents: req.OutstandingBalanceCents,
		BalanceSettled:          req.BalanceSettled,
		RecordingRef:            req.RecordingRef,
		Notes:                   req.Notes,
		RecordedBy:              recordedBy,
	}

	created, err := s.unitRepo.CreateOwnershipHistory(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("org service: TransferOwnership: %w", err)
	}

	s.logger.InfoContext(ctx, "ownership transferred", "unit_id", unitID, "to_user_id", req.ToUserID)
	return created, nil
}

// GetOwnershipHistory returns all ownership history records for the given unit.
func (s *OrgService) GetOwnershipHistory(ctx context.Context, unitID uuid.UUID) ([]UnitOwnershipHistory, error) {
	history, err := s.unitRepo.ListOwnershipHistoryByUnit(ctx, unitID)
	if err != nil {
		return nil, fmt.Errorf("org service: GetOwnershipHistory: %w", err)
	}
	return history, nil
}

// ─── Amenity operations ──────────────────────────────────────────────────────

// CreateAmenity validates the request and creates a new amenity within an org.
func (s *OrgService) CreateAmenity(ctx context.Context, orgID uuid.UUID, req CreateAmenityRequest) (*Amenity, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	a := &Amenity{
		OrgID:            orgID,
		Name:             req.Name,
		AmenityType:      req.AmenityType,
		Description:      req.Description,
		Location:         req.Location,
		Capacity:         req.Capacity,
		IsReservable:     req.IsReservable,
		ReservationRules: req.ReservationRules,
		FeeCents:         req.FeeCents,
		Hours:            req.Hours,
		Status:           "open",
		Metadata:         req.Metadata,
	}
	if a.ReservationRules == nil {
		a.ReservationRules = map[string]any{}
	}
	if a.Hours == nil {
		a.Hours = map[string]any{}
	}
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}

	created, err := s.amenityRepo.CreateAmenity(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateAmenity: %w", err)
	}

	s.logger.InfoContext(ctx, "amenity created", "amenity_id", created.ID, "org_id", orgID)
	return created, nil
}

// GetAmenity returns an amenity by ID, or a NotFoundError if it does not exist.
func (s *OrgService) GetAmenity(ctx context.Context, id uuid.UUID) (*Amenity, error) {
	a, err := s.amenityRepo.FindAmenityByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: GetAmenity: %w", err)
	}
	if a == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "amenity"), api.P("id", id.String()))
	}
	return a, nil
}

// ListAmenities returns amenities for the given org, supporting cursor-based pagination.
func (s *OrgService) ListAmenities(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Amenity, bool, error) {
	amenities, hasMore, err := s.amenityRepo.ListAmenitiesByOrg(ctx, orgID, limit, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("org service: ListAmenities: %w", err)
	}
	return amenities, hasMore, nil
}

// UpdateAmenity applies partial updates to an amenity.
func (s *OrgService) UpdateAmenity(ctx context.Context, id uuid.UUID, req UpdateAmenityRequest) (*Amenity, error) {
	a, err := s.GetAmenity(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.AmenityType != nil {
		a.AmenityType = *req.AmenityType
	}
	if req.Description != nil {
		a.Description = req.Description
	}
	if req.Location != nil {
		a.Location = req.Location
	}
	if req.Capacity != nil {
		a.Capacity = req.Capacity
	}
	if req.IsReservable != nil {
		a.IsReservable = *req.IsReservable
	}
	if req.ReservationRules != nil {
		a.ReservationRules = req.ReservationRules
	}
	if req.FeeCents != nil {
		a.FeeCents = req.FeeCents
	}
	if req.Hours != nil {
		a.Hours = req.Hours
	}
	if req.Status != nil {
		a.Status = *req.Status
	}
	if req.Metadata != nil {
		a.Metadata = req.Metadata
	}

	updated, err := s.amenityRepo.UpdateAmenity(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateAmenity: %w", err)
	}

	s.logger.InfoContext(ctx, "amenity updated", "amenity_id", id)
	return updated, nil
}

// DeleteAmenity soft-deletes an amenity by ID.
func (s *OrgService) DeleteAmenity(ctx context.Context, id uuid.UUID) error {
	if err := s.amenityRepo.SoftDeleteAmenity(ctx, id); err != nil {
		return fmt.Errorf("org service: DeleteAmenity: %w", err)
	}
	s.logger.InfoContext(ctx, "amenity deleted", "amenity_id", id)
	return nil
}

// CreateReservation validates the request and creates a new amenity reservation.
func (s *OrgService) CreateReservation(ctx context.Context, orgID, amenityID uuid.UUID, req CreateReservationRequest) (*AmenityReservation, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	res := &AmenityReservation{
		AmenityID:  amenityID,
		OrgID:      orgID,
		UserID:     req.UserID,
		UnitID:     req.UnitID,
		StartsAt:   req.StartsAt,
		EndsAt:     req.EndsAt,
		GuestCount: req.GuestCount,
		Notes:      req.Notes,
	}

	created, err := s.amenityRepo.CreateReservation(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateReservation: %w", err)
	}

	s.logger.InfoContext(ctx, "reservation created", "reservation_id", created.ID, "amenity_id", amenityID)
	return created, nil
}

// ListAmenityReservations returns all reservations for an amenity.
func (s *OrgService) ListAmenityReservations(ctx context.Context, amenityID uuid.UUID) ([]AmenityReservation, error) {
	reservations, err := s.amenityRepo.ListReservationsByAmenity(ctx, amenityID)
	if err != nil {
		return nil, fmt.Errorf("org service: ListAmenityReservations: %w", err)
	}
	return reservations, nil
}

// ListUserReservations returns all reservations for the current user within an org.
func (s *OrgService) ListUserReservations(ctx context.Context, orgID, userID uuid.UUID) ([]AmenityReservation, error) {
	reservations, err := s.amenityRepo.ListReservationsByUser(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("org service: ListUserReservations: %w", err)
	}
	return reservations, nil
}

// GetReservation returns a reservation by ID, or a NotFoundError if it does not exist.
func (s *OrgService) GetReservation(ctx context.Context, id uuid.UUID) (*AmenityReservation, error) {
	res, err := s.amenityRepo.FindReservationByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: GetReservation: %w", err)
	}
	if res == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "reservation"), api.P("id", id.String()))
	}
	return res, nil
}

// UpdateReservation applies partial updates to a reservation.
func (s *OrgService) UpdateReservation(ctx context.Context, id uuid.UUID, req UpdateReservationRequest) (*AmenityReservation, error) {
	res, err := s.GetReservation(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Status != nil {
		res.Status = *req.Status
	}
	if req.StartsAt != nil {
		res.StartsAt = *req.StartsAt
	}
	if req.EndsAt != nil {
		res.EndsAt = *req.EndsAt
	}
	if req.GuestCount != nil {
		res.GuestCount = req.GuestCount
	}
	if req.Notes != nil {
		res.Notes = req.Notes
	}

	updated, err := s.amenityRepo.UpdateReservation(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateReservation: %w", err)
	}

	s.logger.InfoContext(ctx, "reservation updated", "reservation_id", id)
	return updated, nil
}

// ─── Vendor operations ───────────────────────────────────────────────────────

// CreateVendor validates the request and creates a new vendor.
func (s *OrgService) CreateVendor(ctx context.Context, req CreateVendorRequest) (*Vendor, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	v := &Vendor{
		Name:            req.Name,
		ContactEmail:    req.ContactEmail,
		ContactPhone:    req.ContactPhone,
		ServiceTypes:    req.ServiceTypes,
		LicenseNumber:   req.LicenseNumber,
		InsuranceExpiry: req.InsuranceExpiry,
		Metadata:        req.Metadata,
	}
	if v.ServiceTypes == nil {
		v.ServiceTypes = []string{}
	}
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}

	created, err := s.vendorRepo.CreateVendor(ctx, v)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateVendor: %w", err)
	}

	s.logger.InfoContext(ctx, "vendor created", "vendor_id", created.ID)
	return created, nil
}

// GetVendor returns a vendor by ID, or a NotFoundError if it does not exist.
func (s *OrgService) GetVendor(ctx context.Context, id uuid.UUID) (*Vendor, error) {
	v, err := s.vendorRepo.FindVendorByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: GetVendor: %w", err)
	}
	if v == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "vendor"), api.P("id", id.String()))
	}
	return v, nil
}

// ListVendors returns vendors, supporting cursor-based pagination.
func (s *OrgService) ListVendors(ctx context.Context, limit int, afterID *uuid.UUID) ([]Vendor, bool, error) {
	vendors, hasMore, err := s.vendorRepo.ListVendors(ctx, limit, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("org service: ListVendors: %w", err)
	}
	return vendors, hasMore, nil
}

// UpdateVendor applies partial updates to a vendor.
func (s *OrgService) UpdateVendor(ctx context.Context, id uuid.UUID, req UpdateVendorRequest) (*Vendor, error) {
	v, err := s.GetVendor(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		v.Name = *req.Name
	}
	if req.ContactEmail != nil {
		v.ContactEmail = req.ContactEmail
	}
	if req.ContactPhone != nil {
		v.ContactPhone = req.ContactPhone
	}
	if req.ServiceTypes != nil {
		v.ServiceTypes = req.ServiceTypes
	}
	if req.LicenseNumber != nil {
		v.LicenseNumber = req.LicenseNumber
	}
	if req.InsuranceExpiry != nil {
		v.InsuranceExpiry = req.InsuranceExpiry
	}
	if req.Metadata != nil {
		v.Metadata = req.Metadata
	}

	updated, err := s.vendorRepo.UpdateVendor(ctx, v)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateVendor: %w", err)
	}

	s.logger.InfoContext(ctx, "vendor updated", "vendor_id", id)
	return updated, nil
}

// DeleteVendor soft-deletes a vendor by ID.
func (s *OrgService) DeleteVendor(ctx context.Context, id uuid.UUID) error {
	if err := s.vendorRepo.SoftDeleteVendor(ctx, id); err != nil {
		return fmt.Errorf("org service: DeleteVendor: %w", err)
	}
	s.logger.InfoContext(ctx, "vendor deleted", "vendor_id", id)
	return nil
}

// CreateVendorAssignment validates the request and assigns a vendor to an org.
func (s *OrgService) CreateVendorAssignment(ctx context.Context, orgID uuid.UUID, req CreateVendorAssignmentRequest) (*VendorAssignment, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	a := &VendorAssignment{
		VendorID:     req.VendorID,
		OrgID:        orgID,
		ServiceScope: req.ServiceScope,
		ContractRef:  req.ContractRef,
	}

	created, err := s.vendorRepo.CreateAssignment(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateVendorAssignment: %w", err)
	}

	s.logger.InfoContext(ctx, "vendor assignment created", "assignment_id", created.ID, "org_id", orgID, "vendor_id", req.VendorID)
	return created, nil
}

// ListVendorAssignments returns all active vendor assignments for an org.
func (s *OrgService) ListVendorAssignments(ctx context.Context, orgID uuid.UUID) ([]VendorAssignment, error) {
	assignments, err := s.vendorRepo.ListAssignmentsByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("org service: ListVendorAssignments: %w", err)
	}
	return assignments, nil
}

// DeleteVendorAssignment ends a vendor assignment.
func (s *OrgService) DeleteVendorAssignment(ctx context.Context, id uuid.UUID) error {
	if err := s.vendorRepo.DeleteAssignment(ctx, id); err != nil {
		return fmt.Errorf("org service: DeleteVendorAssignment: %w", err)
	}
	s.logger.InfoContext(ctx, "vendor assignment deleted", "assignment_id", id)
	return nil
}

// ─── Registration operations ─────────────────────────────────────────────────

// CreateRegistrationType validates the request and creates a new registration type.
func (s *OrgService) CreateRegistrationType(ctx context.Context, orgID uuid.UUID, req CreateRegistrationTypeRequest) (*RegistrationType, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	rt := &RegistrationType{
		OrgID:            orgID,
		Name:             req.Name,
		Slug:             req.Slug,
		Schema:           req.Schema,
		MaxPerUnit:       req.MaxPerUnit,
		RequiresApproval: req.RequiresApproval,
		IsActive:         true,
	}
	if rt.Schema == nil {
		rt.Schema = map[string]any{}
	}

	created, err := s.registrationRepo.CreateRegistrationType(ctx, rt)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateRegistrationType: %w", err)
	}

	s.logger.InfoContext(ctx, "registration type created", "registration_type_id", created.ID, "org_id", orgID)
	return created, nil
}

// ListRegistrationTypes returns all registration types for an org.
func (s *OrgService) ListRegistrationTypes(ctx context.Context, orgID uuid.UUID) ([]RegistrationType, error) {
	types, err := s.registrationRepo.ListRegistrationTypesByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("org service: ListRegistrationTypes: %w", err)
	}
	return types, nil
}

// UpdateRegistrationType applies partial updates to a registration type.
func (s *OrgService) UpdateRegistrationType(ctx context.Context, id uuid.UUID, req UpdateRegistrationTypeRequest) (*RegistrationType, error) {
	rt, err := s.registrationRepo.FindRegistrationTypeByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateRegistrationType find: %w", err)
	}
	if rt == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "registration_type"), api.P("id", id.String()))
	}

	if req.Name != nil {
		rt.Name = *req.Name
	}
	if req.Slug != nil {
		rt.Slug = *req.Slug
	}
	if req.Schema != nil {
		rt.Schema = req.Schema
	}
	if req.MaxPerUnit != nil {
		rt.MaxPerUnit = req.MaxPerUnit
	}
	if req.RequiresApproval != nil {
		rt.RequiresApproval = *req.RequiresApproval
	}
	if req.IsActive != nil {
		rt.IsActive = *req.IsActive
	}

	updated, err := s.registrationRepo.UpdateRegistrationType(ctx, rt)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateRegistrationType: %w", err)
	}

	s.logger.InfoContext(ctx, "registration type updated", "registration_type_id", id)
	return updated, nil
}

// CreateRegistration validates the request and creates a new registration.
func (s *OrgService) CreateRegistration(ctx context.Context, orgID, unitID uuid.UUID, req CreateRegistrationRequest) (*Registration, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	reg := &Registration{
		OrgID:              orgID,
		UnitID:             unitID,
		UserID:             req.UserID,
		RegistrationTypeID: req.RegistrationTypeID,
		Data:               req.Data,
		ExpiresAt:          req.ExpiresAt,
	}
	if reg.Data == nil {
		reg.Data = map[string]any{}
	}

	created, err := s.registrationRepo.CreateRegistration(ctx, reg)
	if err != nil {
		return nil, fmt.Errorf("org service: CreateRegistration: %w", err)
	}

	s.logger.InfoContext(ctx, "registration created", "registration_id", created.ID, "unit_id", unitID)
	return created, nil
}

// ListRegistrations returns all registrations for a unit.
func (s *OrgService) ListRegistrations(ctx context.Context, unitID uuid.UUID) ([]Registration, error) {
	registrations, err := s.registrationRepo.ListRegistrationsByUnit(ctx, unitID)
	if err != nil {
		return nil, fmt.Errorf("org service: ListRegistrations: %w", err)
	}
	return registrations, nil
}

// GetRegistration returns a registration by ID, or a NotFoundError if it does not exist.
func (s *OrgService) GetRegistration(ctx context.Context, id uuid.UUID) (*Registration, error) {
	reg, err := s.registrationRepo.FindRegistrationByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("org service: GetRegistration: %w", err)
	}
	if reg == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "registration"), api.P("id", id.String()))
	}
	return reg, nil
}

// UpdateRegistration applies partial updates to a registration.
func (s *OrgService) UpdateRegistration(ctx context.Context, id uuid.UUID, req UpdateRegistrationRequest) (*Registration, error) {
	reg, err := s.GetRegistration(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Data != nil {
		reg.Data = req.Data
	}
	if req.Status != nil {
		reg.Status = *req.Status
	}
	if req.ExpiresAt != nil {
		reg.ExpiresAt = req.ExpiresAt
	}

	updated, err := s.registrationRepo.UpdateRegistration(ctx, reg)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateRegistration: %w", err)
	}

	s.logger.InfoContext(ctx, "registration updated", "registration_id", id)
	return updated, nil
}

// ApproveRegistration sets a registration's status to approved.
func (s *OrgService) ApproveRegistration(ctx context.Context, id, approverID uuid.UUID) (*Registration, error) {
	reg, err := s.GetRegistration(ctx, id)
	if err != nil {
		return nil, err
	}

	now := reg.UpdatedAt
	_ = now
	reg.Status = "active"
	reg.ApprovedBy = &approverID

	updated, err := s.registrationRepo.UpdateRegistration(ctx, reg)
	if err != nil {
		return nil, fmt.Errorf("org service: ApproveRegistration: %w", err)
	}

	s.logger.InfoContext(ctx, "registration approved", "registration_id", id, "approved_by", approverID)
	return updated, nil
}

// RevokeRegistration soft-deletes a registration by marking it revoked.
func (s *OrgService) RevokeRegistration(ctx context.Context, id uuid.UUID) error {
	reg, err := s.GetRegistration(ctx, id)
	if err != nil {
		return err
	}

	reg.Status = "revoked"
	if _, err := s.registrationRepo.UpdateRegistration(ctx, reg); err != nil {
		return fmt.Errorf("org service: RevokeRegistration: %w", err)
	}

	s.logger.InfoContext(ctx, "registration revoked", "registration_id", id)
	return nil
}
