package org

import (
	"context"
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
	orgRepo        OrgRepository
	membershipRepo MembershipRepository
	unitRepo       UnitRepository
	userRepo UserFinder
	auditor        audit.Auditor
	publisher      queue.Publisher
	logger         *slog.Logger
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
		Zip:          req.Zip,
		Phone:        req.Phone,
		Email:        req.Email,
		Website:      req.Website,
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
		return nil, api.NewNotFoundError(fmt.Sprintf("organization %s not found", id))
	}
	return org, nil
}

// ListOrganizations returns the organizations accessible by the authenticated user.
// It extracts the user from the JWT claims stored in the context.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *OrgService) ListOrganizations(ctx context.Context, limit int, afterID *uuid.UUID) ([]Organization, bool, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, false, api.NewUnauthenticatedError("no claims in context")
	}

	user, err := s.userRepo.FindByIDPUserID(ctx, claims.Subject)
	if err != nil {
		return nil, false, fmt.Errorf("org service: ListOrganizations find user: %w", err)
	}
	if user == nil {
		return nil, false, api.NewNotFoundError("user not found")
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

	updated, err := s.orgRepo.Update(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("org service: UpdateOrganization: %w", err)
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
			fmt.Sprintf("organization %s is not of type 'hoa'", hoaOrgID),
			"hoa_org_id",
		)
	}

	// Validate firm exists and is type "firm".
	firm, err := s.GetOrganization(ctx, req.FirmOrgID)
	if err != nil {
		return nil, err
	}
	if firm.Type != "firm" {
		return nil, api.NewValidationError(
			fmt.Sprintf("organization %s is not of type 'firm'", req.FirmOrgID),
			"firm_org_id",
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
		return nil, api.NewNotFoundError(fmt.Sprintf("membership %s not found", id))
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
		return nil, api.NewNotFoundError(fmt.Sprintf("membership %s not found", id))
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
		return nil, api.NewNotFoundError(fmt.Sprintf("unit %s not found", id))
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
	if err := s.unitRepo.SoftDeleteUnit(ctx, id); err != nil {
		return fmt.Errorf("org service: DeleteUnit: %w", err)
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
		return nil, api.NewNotFoundError(fmt.Sprintf("property for unit %s not found", unitID))
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
		return nil, api.NewNotFoundError(fmt.Sprintf("unit membership %s not found", id))
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
