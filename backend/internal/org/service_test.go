package org_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock repositories ───────────────────────────────────────────────────────

// mockOrgRepo is an in-memory OrgRepository for unit tests.
type mockOrgRepo struct {
	orgs        map[uuid.UUID]*org.Organization
	management  []*org.OrgManagement
	createErr   error
	findErr     error
	updateErr   error
	deleteErr   error
}

func newMockOrgRepo() *mockOrgRepo {
	return &mockOrgRepo{
		orgs:       make(map[uuid.UUID]*org.Organization),
		management: []*org.OrgManagement{},
	}
}

func (m *mockOrgRepo) Create(_ context.Context, o *org.Organization) (*org.Organization, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	o.CreatedAt = time.Now()
	o.UpdatedAt = time.Now()
	if o.Settings == nil {
		o.Settings = map[string]any{}
	}
	copy := *o
	m.orgs[o.ID] = &copy
	return &copy, nil
}

func (m *mockOrgRepo) FindByID(_ context.Context, id uuid.UUID) (*org.Organization, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	o, ok := m.orgs[id]
	if !ok {
		return nil, nil
	}
	copy := *o
	return &copy, nil
}

func (m *mockOrgRepo) FindBySlug(_ context.Context, slug string) (*org.Organization, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	for _, o := range m.orgs {
		if o.Slug == slug {
			copy := *o
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *mockOrgRepo) ListByUserAccess(_ context.Context, _ uuid.UUID, limit int, afterID *uuid.UUID) ([]org.Organization, bool, error) {
	if m.findErr != nil {
		return nil, false, m.findErr
	}
	result := make([]org.Organization, 0, len(m.orgs))
	for _, o := range m.orgs {
		result = append(result, *o)
	}
	hasMore := limit > 0 && len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	return result, hasMore, nil
}

func (m *mockOrgRepo) Update(_ context.Context, o *org.Organization) (*org.Organization, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	o.UpdatedAt = time.Now()
	copy := *o
	m.orgs[o.ID] = &copy
	return &copy, nil
}

func (m *mockOrgRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.orgs, id)
	return nil
}

func (m *mockOrgRepo) ListChildren(_ context.Context, parentID uuid.UUID) ([]org.Organization, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []org.Organization{}
	for _, o := range m.orgs {
		if o.ParentID != nil && *o.ParentID == parentID {
			result = append(result, *o)
		}
	}
	return result, nil
}

func (m *mockOrgRepo) ConnectManagement(_ context.Context, firmOrgID, hoaOrgID uuid.UUID) (*org.OrgManagement, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	mgmt := &org.OrgManagement{
		ID:        uuid.New(),
		FirmOrgID: firmOrgID,
		HOAOrgID:  hoaOrgID,
		StartedAt: time.Now(),
		CreatedAt: time.Now(),
	}
	m.management = append(m.management, mgmt)
	return mgmt, nil
}

func (m *mockOrgRepo) DisconnectManagement(_ context.Context, hoaOrgID uuid.UUID) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	now := time.Now()
	for _, mgmt := range m.management {
		if mgmt.HOAOrgID == hoaOrgID && mgmt.EndedAt == nil {
			mgmt.EndedAt = &now
		}
	}
	return nil
}

func (m *mockOrgRepo) ListManagementHistory(_ context.Context, hoaOrgID uuid.UUID) ([]org.OrgManagement, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []org.OrgManagement{}
	for _, mgmt := range m.management {
		if mgmt.HOAOrgID == hoaOrgID {
			result = append(result, *mgmt)
		}
	}
	return result, nil
}

func (m *mockOrgRepo) FindActiveManagement(_ context.Context, hoaOrgID uuid.UUID) (*org.OrgManagement, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	for _, mgmt := range m.management {
		if mgmt.HOAOrgID == hoaOrgID && mgmt.EndedAt == nil {
			copy := *mgmt
			return &copy, nil
		}
	}
	return nil, nil
}

// mockMembershipRepo is an in-memory MembershipRepository for unit tests.
type mockMembershipRepo struct {
	memberships map[uuid.UUID]*iam.Membership
	createErr   error
	findErr     error
	updateErr   error
	deleteErr   error
}

func newMockMembershipRepo() *mockMembershipRepo {
	return &mockMembershipRepo{
		memberships: make(map[uuid.UUID]*iam.Membership),
	}
}

func (m *mockMembershipRepo) Create(_ context.Context, ms *iam.Membership) (*iam.Membership, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if ms.ID == uuid.Nil {
		ms.ID = uuid.New()
	}
	ms.CreatedAt = time.Now()
	ms.UpdatedAt = time.Now()
	copy := *ms
	m.memberships[ms.ID] = &copy
	return &copy, nil
}

func (m *mockMembershipRepo) FindByID(_ context.Context, id uuid.UUID) (*iam.Membership, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	ms, ok := m.memberships[id]
	if !ok {
		return nil, nil
	}
	copy := *ms
	return &copy, nil
}

func (m *mockMembershipRepo) ListByOrg(_ context.Context, orgID uuid.UUID) ([]iam.Membership, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []iam.Membership{}
	for _, ms := range m.memberships {
		if ms.OrgID == orgID && ms.DeletedAt == nil {
			result = append(result, *ms)
		}
	}
	return result, nil
}

func (m *mockMembershipRepo) Update(_ context.Context, ms *iam.Membership) (*iam.Membership, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	ms.UpdatedAt = time.Now()
	copy := *ms
	m.memberships[ms.ID] = &copy
	return &copy, nil
}

func (m *mockMembershipRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.memberships, id)
	return nil
}

// mockUnitRepo is an in-memory UnitRepository for unit tests.
type mockUnitRepo struct {
	units            map[uuid.UUID]*org.Unit
	properties       map[uuid.UUID]*org.Property
	unitMemberships  map[uuid.UUID]*org.UnitMembership
	ownershipHistory []org.UnitOwnershipHistory
	createErr        error
	findErr          error
	updateErr        error
	deleteErr        error
}

func newMockUnitRepo() *mockUnitRepo {
	return &mockUnitRepo{
		units:           make(map[uuid.UUID]*org.Unit),
		properties:      make(map[uuid.UUID]*org.Property),
		unitMemberships: make(map[uuid.UUID]*org.UnitMembership),
	}
}

func (m *mockUnitRepo) CreateUnit(_ context.Context, u *org.Unit) (*org.Unit, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	if u.Metadata == nil {
		u.Metadata = map[string]any{}
	}
	copy := *u
	m.units[u.ID] = &copy
	return &copy, nil
}

func (m *mockUnitRepo) FindUnitByID(_ context.Context, id uuid.UUID) (*org.Unit, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	u, ok := m.units[id]
	if !ok {
		return nil, nil
	}
	copy := *u
	return &copy, nil
}

func (m *mockUnitRepo) ListUnitsByOrg(_ context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]org.Unit, bool, error) {
	if m.findErr != nil {
		return nil, false, m.findErr
	}
	result := []org.Unit{}
	for _, u := range m.units {
		if u.OrgID == orgID && u.DeletedAt == nil {
			result = append(result, *u)
		}
	}
	hasMore := limit > 0 && len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	return result, hasMore, nil
}

func (m *mockUnitRepo) UpdateUnit(_ context.Context, u *org.Unit) (*org.Unit, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	u.UpdatedAt = time.Now()
	copy := *u
	m.units[u.ID] = &copy
	return &copy, nil
}

func (m *mockUnitRepo) SoftDeleteUnit(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.units, id)
	return nil
}

func (m *mockUnitRepo) GetProperty(_ context.Context, unitID uuid.UUID) (*org.Property, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	prop, ok := m.properties[unitID]
	if !ok {
		return nil, nil
	}
	copy := *prop
	return &copy, nil
}

func (m *mockUnitRepo) UpsertProperty(_ context.Context, prop *org.Property) (*org.Property, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	if prop.ID == uuid.Nil {
		prop.ID = uuid.New()
	}
	prop.CreatedAt = time.Now()
	prop.UpdatedAt = time.Now()
	copy := *prop
	m.properties[prop.UnitID] = &copy
	return &copy, nil
}

func (m *mockUnitRepo) CreateUnitMembership(_ context.Context, um *org.UnitMembership) (*org.UnitMembership, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if um.ID == uuid.Nil {
		um.ID = uuid.New()
	}
	um.CreatedAt = time.Now()
	um.UpdatedAt = time.Now()
	copy := *um
	m.unitMemberships[um.ID] = &copy
	return &copy, nil
}

func (m *mockUnitRepo) FindUnitMembershipByID(_ context.Context, id uuid.UUID) (*org.UnitMembership, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	um, ok := m.unitMemberships[id]
	if !ok {
		return nil, nil
	}
	copy := *um
	return &copy, nil
}

func (m *mockUnitRepo) ListUnitMemberships(_ context.Context, unitID uuid.UUID) ([]org.UnitMembership, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []org.UnitMembership{}
	for _, um := range m.unitMemberships {
		if um.UnitID == unitID && um.EndedAt == nil {
			result = append(result, *um)
		}
	}
	return result, nil
}

func (m *mockUnitRepo) UpdateUnitMembership(_ context.Context, um *org.UnitMembership) (*org.UnitMembership, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	um.UpdatedAt = time.Now()
	copy := *um
	m.unitMemberships[um.ID] = &copy
	return &copy, nil
}

func (m *mockUnitRepo) EndUnitMembership(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if um, ok := m.unitMemberships[id]; ok {
		now := time.Now()
		um.EndedAt = &now
	}
	return nil
}

func (m *mockUnitRepo) CreateOwnershipHistory(_ context.Context, h *org.UnitOwnershipHistory) (*org.UnitOwnershipHistory, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	h.CreatedAt = time.Now()
	copy := *h
	m.ownershipHistory = append(m.ownershipHistory, copy)
	return &copy, nil
}

func (m *mockUnitRepo) ListOwnershipHistoryByUnit(_ context.Context, unitID uuid.UUID) ([]org.UnitOwnershipHistory, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []org.UnitOwnershipHistory
	for _, h := range m.ownershipHistory {
		if h.UnitID == unitID {
			result = append(result, h)
		}
	}
	if result == nil {
		return []org.UnitOwnershipHistory{}, nil
	}
	return result, nil
}

// mockUserRepo is a minimal iam.UserRepository for unit tests.
type mockUserRepo struct {
	users   map[string]*iam.User // keyed by idp_user_id
	findErr error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*iam.User),
	}
}

func (m *mockUserRepo) FindByIDPUserID(_ context.Context, idpUserID string) (*iam.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	u, ok := m.users[idpUserID]
	if !ok {
		return nil, nil
	}
	copy := *u
	return &copy, nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id uuid.UUID) (*iam.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	for _, u := range m.users {
		if u.ID == id {
			copy := *u
			return &copy, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) Upsert(_ context.Context, u *iam.User) (*iam.User, error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	m.users[u.IDPUserID] = u
	return u, nil
}

func (m *mockUserRepo) UpdateLastLogin(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockUserRepo) FindMembershipsByUserID(_ context.Context, _ uuid.UUID) ([]iam.Membership, error) {
	return []iam.Membership{}, nil
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

func newTestService(orgRepo *mockOrgRepo, memberRepo *mockMembershipRepo, unitRepo *mockUnitRepo, userRepo *mockUserRepo) *org.OrgService {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return org.NewOrgService(orgRepo, memberRepo, unitRepo, userRepo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
}

func newDefaultTestService() (*org.OrgService, *mockOrgRepo, *mockMembershipRepo, *mockUnitRepo, *mockUserRepo) {
	orgRepo := newMockOrgRepo()
	memberRepo := newMockMembershipRepo()
	unitRepo := newMockUnitRepo()
	userRepo := newMockUserRepo()
	svc := newTestService(orgRepo, memberRepo, unitRepo, userRepo)
	return svc, orgRepo, memberRepo, unitRepo, userRepo
}

// ─── Organization tests ───────────────────────────────────────────────────────

func TestCreateOrganization_Success(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	req := org.CreateOrgRequest{
		Type: "hoa",
		Name: "Sunset Ridge HOA",
	}

	created, err := svc.CreateOrganization(context.Background(), req)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "Sunset Ridge HOA", created.Name)
	assert.Equal(t, "hoa", created.Type)
	assert.Len(t, orgRepo.orgs, 1, "org should be persisted in repository")
}

func TestCreateOrganization_ValidationFails_MissingName(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	req := org.CreateOrgRequest{
		Type: "hoa",
		// Name intentionally omitted
	}

	_, err := svc.CreateOrganization(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
	assert.Empty(t, orgRepo.orgs, "repo.Create should not be called on validation failure")
}

func TestCreateOrganization_ValidationFails_InvalidType(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	req := org.CreateOrgRequest{
		Type: "unknown",
		Name: "Some Org",
	}

	_, err := svc.CreateOrganization(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "type must be")
	assert.Empty(t, orgRepo.orgs)
}

func TestCreateOrganization_RepoError_ReturnsError(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()
	orgRepo.createErr = errors.New("db connection failed")

	req := org.CreateOrgRequest{Type: "firm", Name: "Apex Management"}

	_, err := svc.CreateOrganization(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db connection failed")
}

func TestGetOrganization_Success(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	id := uuid.New()
	orgRepo.orgs[id] = &org.Organization{
		ID:   id,
		Type: "firm",
		Name: "Apex Management",
	}

	result, err := svc.GetOrganization(context.Background(), id)

	require.NoError(t, err)
	assert.Equal(t, id, result.ID)
	assert.Equal(t, "Apex Management", result.Name)
}

func TestGetOrganization_NotFound(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	_, err := svc.GetOrganization(context.Background(), uuid.New())

	require.Error(t, err)
	var notFound *api.NotFoundError
	assert.True(t, errors.As(err, &notFound), "expected NotFoundError, got: %T: %v", err, err)
}

func TestListOrganizations_UsesClaimsFromContext(t *testing.T) {
	svc, orgRepo, _, _, userRepo := newDefaultTestService()

	userID := uuid.New()
	idpUserID := "idp-user-123"
	userRepo.users[idpUserID] = &iam.User{
		ID:        userID,
		IDPUserID: idpUserID,
		Email:     "alice@example.com",
	}

	// Seed an org in the repo (ListByUserAccess returns all orgs for the mock).
	orgRepo.orgs[uuid.New()] = &org.Organization{
		ID:   uuid.New(),
		Type: "hoa",
		Name: "Pine Valley HOA",
	}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: idpUserID,
		Email:   "alice@example.com",
		Name:    "Alice",
	})

	orgs, _, err := svc.ListOrganizations(ctx, 25, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, orgs)
}

func TestListOrganizations_NoClaimsInContext(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	_, _, err := svc.ListOrganizations(context.Background(), 25, nil)

	require.Error(t, err)
	var unauthErr *api.UnauthenticatedError
	assert.True(t, errors.As(err, &unauthErr))
}

func TestListOrganizations_UserNotFound(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "idp-unknown",
		Email:   "ghost@example.com",
		Name:    "Ghost",
	})

	_, _, err := svc.ListOrganizations(ctx, 25, nil)

	require.Error(t, err)
	var notFound *api.NotFoundError
	assert.True(t, errors.As(err, &notFound))
}

func TestUpdateOrganization_Success(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	id := uuid.New()
	orgRepo.orgs[id] = &org.Organization{
		ID:   id,
		Type: "hoa",
		Name: "Old Name",
	}

	newName := "New Name"
	updated, err := svc.UpdateOrganization(context.Background(), id, org.UpdateOrgRequest{
		Name: &newName,
	})

	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
}

func TestUpdateOrganization_ValidationFails(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	// UpdateOrgRequest with no fields set should fail validation.
	_, err := svc.UpdateOrganization(context.Background(), uuid.New(), org.UpdateOrgRequest{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one field must be provided")
}

func TestUpdateOrganization_NotFound(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	newName := "New Name"
	_, err := svc.UpdateOrganization(context.Background(), uuid.New(), org.UpdateOrgRequest{Name: &newName})

	require.Error(t, err)
	var notFound *api.NotFoundError
	assert.True(t, errors.As(err, &notFound))
}

func TestDeleteOrganization_Success(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	id := uuid.New()
	orgRepo.orgs[id] = &org.Organization{ID: id, Type: "hoa", Name: "Test HOA"}

	err := svc.DeleteOrganization(context.Background(), id)

	require.NoError(t, err)
	assert.NotContains(t, orgRepo.orgs, id)
}

func TestListChildren_ReturnsChildOrgs(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	parentID := uuid.New()
	childID := uuid.New()
	orgRepo.orgs[parentID] = &org.Organization{ID: parentID, Type: "firm", Name: "Parent Firm"}
	orgRepo.orgs[childID] = &org.Organization{ID: childID, Type: "firm", Name: "Child Firm", ParentID: &parentID}

	children, err := svc.ListChildren(context.Background(), parentID)

	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, childID, children[0].ID)
}

// ─── Management tests ─────────────────────────────────────────────────────────

func TestConnectManagement_Success(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	hoaID := uuid.New()
	firmID := uuid.New()
	orgRepo.orgs[hoaID] = &org.Organization{ID: hoaID, Type: "hoa", Name: "Sunset HOA"}
	orgRepo.orgs[firmID] = &org.Organization{ID: firmID, Type: "firm", Name: "Apex Management"}

	mgmt, err := svc.ConnectManagement(context.Background(), hoaID, org.ConnectManagementRequest{
		FirmOrgID: firmID,
	})

	require.NoError(t, err)
	assert.Equal(t, hoaID, mgmt.HOAOrgID)
	assert.Equal(t, firmID, mgmt.FirmOrgID)
}

func TestConnectManagement_ValidatesOrgTypes_HOANotHOA(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	// HOA org ID actually points to a "firm" type — should fail.
	firmAsHOAID := uuid.New()
	firmID := uuid.New()
	orgRepo.orgs[firmAsHOAID] = &org.Organization{ID: firmAsHOAID, Type: "firm", Name: "Not an HOA"}
	orgRepo.orgs[firmID] = &org.Organization{ID: firmID, Type: "firm", Name: "Apex Management"}

	_, err := svc.ConnectManagement(context.Background(), firmAsHOAID, org.ConnectManagementRequest{
		FirmOrgID: firmID,
	})

	require.Error(t, err)
	var valErr *api.ValidationError
	assert.True(t, errors.As(err, &valErr), "expected ValidationError, got: %T: %v", err, err)
	assert.Contains(t, err.Error(), "not of type 'hoa'")
}

func TestConnectManagement_ValidatesOrgTypes_FirmNotFirm(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	hoaID := uuid.New()
	hoaAsFirmID := uuid.New()
	orgRepo.orgs[hoaID] = &org.Organization{ID: hoaID, Type: "hoa", Name: "Sunset HOA"}
	orgRepo.orgs[hoaAsFirmID] = &org.Organization{ID: hoaAsFirmID, Type: "hoa", Name: "Not a Firm"}

	_, err := svc.ConnectManagement(context.Background(), hoaID, org.ConnectManagementRequest{
		FirmOrgID: hoaAsFirmID,
	})

	require.Error(t, err)
	var valErr *api.ValidationError
	assert.True(t, errors.As(err, &valErr))
	assert.Contains(t, err.Error(), "not of type 'firm'")
}

func TestConnectManagement_ValidationFails_ZeroFirmOrgID(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	hoaID := uuid.New()
	orgRepo.orgs[hoaID] = &org.Organization{ID: hoaID, Type: "hoa", Name: "Sunset HOA"}

	_, err := svc.ConnectManagement(context.Background(), hoaID, org.ConnectManagementRequest{
		FirmOrgID: uuid.UUID{}, // zero UUID
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "firm_org_id is required")
}

func TestDisconnectManagement_Success(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	hoaID := uuid.New()
	firmID := uuid.New()
	mgmt := &org.OrgManagement{ID: uuid.New(), HOAOrgID: hoaID, FirmOrgID: firmID}
	orgRepo.management = append(orgRepo.management, mgmt)

	err := svc.DisconnectManagement(context.Background(), hoaID)

	require.NoError(t, err)
	assert.NotNil(t, orgRepo.management[0].EndedAt)
}

func TestGetManagementHistory_ReturnsHistory(t *testing.T) {
	svc, orgRepo, _, _, _ := newDefaultTestService()

	hoaID := uuid.New()
	firmID := uuid.New()
	orgRepo.management = append(orgRepo.management, &org.OrgManagement{
		ID:        uuid.New(),
		HOAOrgID:  hoaID,
		FirmOrgID: firmID,
	})

	history, err := svc.GetManagementHistory(context.Background(), hoaID)

	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, hoaID, history[0].HOAOrgID)
}

// ─── Membership tests ─────────────────────────────────────────────────────────

func TestCreateMembership_Success(t *testing.T) {
	svc, _, memberRepo, _, _ := newDefaultTestService()

	orgID := uuid.New()
	req := org.CreateMembershipRequest{
		UserID: uuid.New(),
		RoleID: uuid.New(),
	}

	created, err := svc.CreateMembership(context.Background(), orgID, req)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, orgID, created.OrgID)
	assert.Equal(t, req.UserID, created.UserID)
	assert.Equal(t, "invited", created.Status)
	assert.Len(t, memberRepo.memberships, 1)
}

func TestCreateMembership_ValidationFails(t *testing.T) {
	svc, _, memberRepo, _, _ := newDefaultTestService()

	_, err := svc.CreateMembership(context.Background(), uuid.New(), org.CreateMembershipRequest{
		// UserID and RoleID both zero
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user_id is required")
	assert.Empty(t, memberRepo.memberships)
}

func TestListMemberships_ReturnsMembershipsForOrg(t *testing.T) {
	svc, _, memberRepo, _, _ := newDefaultTestService()

	orgID := uuid.New()
	otherOrgID := uuid.New()
	memberRepo.memberships[uuid.New()] = &iam.Membership{ID: uuid.New(), OrgID: orgID, Status: "active"}
	memberRepo.memberships[uuid.New()] = &iam.Membership{ID: uuid.New(), OrgID: otherOrgID, Status: "active"}

	memberships, err := svc.ListMemberships(context.Background(), orgID)

	require.NoError(t, err)
	require.Len(t, memberships, 1)
	assert.Equal(t, orgID, memberships[0].OrgID)
}

func TestUpdateMembership_Success(t *testing.T) {
	svc, _, memberRepo, _, _ := newDefaultTestService()

	id := uuid.New()
	roleID := uuid.New()
	memberRepo.memberships[id] = &iam.Membership{
		ID:     id,
		RoleID: roleID,
		Status: "invited",
	}

	newRoleID := uuid.New()
	newStatus := "active"
	updated, err := svc.UpdateMembership(context.Background(), id, &newRoleID, &newStatus)

	require.NoError(t, err)
	assert.Equal(t, newRoleID, updated.RoleID)
	assert.Equal(t, "active", updated.Status)
}

func TestUpdateMembership_NotFound(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	_, err := svc.UpdateMembership(context.Background(), uuid.New(), nil, nil)

	require.Error(t, err)
	var notFound *api.NotFoundError
	assert.True(t, errors.As(err, &notFound))
}

func TestDeleteMembership_Success(t *testing.T) {
	svc, _, memberRepo, _, _ := newDefaultTestService()

	id := uuid.New()
	memberRepo.memberships[id] = &iam.Membership{ID: id}

	err := svc.DeleteMembership(context.Background(), id)

	require.NoError(t, err)
	assert.NotContains(t, memberRepo.memberships, id)
}

// ─── Unit tests ───────────────────────────────────────────────────────────────

func TestCreateUnit_Success(t *testing.T) {
	svc, _, _, unitRepo, _ := newDefaultTestService()

	orgID := uuid.New()
	req := org.CreateUnitRequest{
		Label: "Unit 101",
	}

	created, err := svc.CreateUnit(context.Background(), orgID, req)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, orgID, created.OrgID)
	assert.Equal(t, "Unit 101", created.Label)
	assert.Equal(t, 1.0, created.VotingWeight, "default voting weight should be 1.0")
	assert.Equal(t, "active", created.Status)
	assert.Len(t, unitRepo.units, 1)
}

func TestCreateUnit_DefaultVotingWeight(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	req := org.CreateUnitRequest{Label: "Unit A"}
	created, err := svc.CreateUnit(context.Background(), uuid.New(), req)

	require.NoError(t, err)
	assert.Equal(t, 1.0, created.VotingWeight)
}

func TestCreateUnit_CustomVotingWeight(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	weight := 2.5
	req := org.CreateUnitRequest{Label: "Unit B", VotingWeight: &weight}
	created, err := svc.CreateUnit(context.Background(), uuid.New(), req)

	require.NoError(t, err)
	assert.Equal(t, 2.5, created.VotingWeight)
}

func TestCreateUnit_ValidationFails_MissingLabel(t *testing.T) {
	svc, _, _, unitRepo, _ := newDefaultTestService()

	_, err := svc.CreateUnit(context.Background(), uuid.New(), org.CreateUnitRequest{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "label is required")
	assert.Empty(t, unitRepo.units)
}

func TestGetUnit_Success(t *testing.T) {
	svc, _, _, unitRepo, _ := newDefaultTestService()

	id := uuid.New()
	unitRepo.units[id] = &org.Unit{ID: id, Label: "Unit 1"}

	result, err := svc.GetUnit(context.Background(), id)

	require.NoError(t, err)
	assert.Equal(t, id, result.ID)
}

func TestGetUnit_NotFound(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	_, err := svc.GetUnit(context.Background(), uuid.New())

	require.Error(t, err)
	var notFound *api.NotFoundError
	assert.True(t, errors.As(err, &notFound))
}

func TestListUnits_ReturnsUnitsForOrg(t *testing.T) {
	svc, _, _, unitRepo, _ := newDefaultTestService()

	orgID := uuid.New()
	otherOrgID := uuid.New()
	unitRepo.units[uuid.New()] = &org.Unit{ID: uuid.New(), OrgID: orgID, Label: "Unit A"}
	unitRepo.units[uuid.New()] = &org.Unit{ID: uuid.New(), OrgID: otherOrgID, Label: "Unit B"}

	units, _, err := svc.ListUnits(context.Background(), orgID, 25, nil)

	require.NoError(t, err)
	require.Len(t, units, 1)
	assert.Equal(t, orgID, units[0].OrgID)
}

func TestUpdateUnit_Success(t *testing.T) {
	svc, _, _, unitRepo, _ := newDefaultTestService()

	id := uuid.New()
	unitRepo.units[id] = &org.Unit{ID: id, Label: "Old Label", VotingWeight: 1.0}

	newLabel := "New Label"
	updated, err := svc.UpdateUnit(context.Background(), id, org.UpdateUnitRequest{Label: &newLabel})

	require.NoError(t, err)
	assert.Equal(t, "New Label", updated.Label)
}

func TestUpdateUnit_NotFound(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	newLabel := "Anything"
	_, err := svc.UpdateUnit(context.Background(), uuid.New(), org.UpdateUnitRequest{Label: &newLabel})

	require.Error(t, err)
	var notFound *api.NotFoundError
	assert.True(t, errors.As(err, &notFound))
}

func TestDeleteUnit_Success(t *testing.T) {
	svc, _, _, unitRepo, _ := newDefaultTestService()

	id := uuid.New()
	unitRepo.units[id] = &org.Unit{ID: id, Label: "Unit X"}

	err := svc.DeleteUnit(context.Background(), id)

	require.NoError(t, err)
	assert.NotContains(t, unitRepo.units, id)
}

// ─── Property tests ───────────────────────────────────────────────────────────

func TestGetProperty_Success(t *testing.T) {
	svc, _, _, unitRepo, _ := newDefaultTestService()

	unitID := uuid.New()
	unitRepo.properties[unitID] = &org.Property{ID: uuid.New(), UnitID: unitID}

	prop, err := svc.GetProperty(context.Background(), unitID)

	require.NoError(t, err)
	assert.Equal(t, unitID, prop.UnitID)
}

func TestGetProperty_NotFound(t *testing.T) {
	svc, _, _, _, _ := newDefaultTestService()

	_, err := svc.GetProperty(context.Background(), uuid.New())

	require.Error(t, err)
	var notFound *api.NotFoundError
	assert.True(t, errors.As(err, &notFound))
}

func TestSetProperty_Success(t *testing.T) {
	svc, _, _, unitRepo, _ := newDefaultTestService()

	unitID := uuid.New()
	sqft := 1200
	prop := &org.Property{
		SquareFeet: &sqft,
	}

	saved, err := svc.SetProperty(context.Background(), unitID, prop)

	require.NoError(t, err)
	assert.Equal(t, unitID, saved.UnitID)
	assert.Equal(t, &sqft, saved.SquareFeet)
	assert.Contains(t, unitRepo.properties, unitID)
}
