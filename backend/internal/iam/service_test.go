package iam_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUserRepository implements UserRepository for unit testing.
type mockUserRepository struct {
	users       map[string]*iam.User     // keyed by idp_user_id
	memberships map[uuid.UUID][]iam.Membership
	upsertErr   error
	findErr     error
}

func newMockRepo() *mockUserRepository {
	return &mockUserRepository{
		users:       make(map[string]*iam.User),
		memberships: make(map[uuid.UUID][]iam.Membership),
	}
}

func (m *mockUserRepository) FindByIDPUserID(ctx context.Context, idpUserID string) (*iam.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	user, ok := m.users[idpUserID]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (m *mockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*iam.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepository) Upsert(ctx context.Context, user *iam.User) (*iam.User, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	// Assign a new UUID if not present.
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	m.users[user.IDPUserID] = user
	return user, nil
}

func (m *mockUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockUserRepository) FindMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]iam.Membership, error) {
	return m.memberships[userID], nil
}

// --- Tests ---

func TestGetOrCreateUser_FindsExistingUser(t *testing.T) {
	repo := newMockRepo()
	existingID := uuid.New()
	repo.users["idp-user-123"] = &iam.User{
		ID:          existingID,
		IDPUserID:   "idp-user-123",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		IsActive:    true,
	}
	// Track upsert calls via upsertErr being unused; verify by checking repo state.
	initialCount := len(repo.users)

	svc := iam.NewUserService(repo)
	claims := &auth.Claims{
		Subject: "idp-user-123",
		Email:   "alice@example.com",
		Name:    "Alice",
	}

	user, err := svc.GetOrCreateUser(context.Background(), claims)

	require.NoError(t, err)
	assert.Equal(t, existingID, user.ID)
	assert.Equal(t, "alice@example.com", user.Email)
	// Upsert should NOT have been called: repo still has same count.
	assert.Equal(t, initialCount, len(repo.users), "Upsert should not have been called for existing user")
}

func TestGetOrCreateUser_CreatesNewUser(t *testing.T) {
	repo := newMockRepo()

	svc := iam.NewUserService(repo)
	claims := &auth.Claims{
		Subject: "idp-new-999",
		Email:   "bob@example.com",
		Name:    "Bob",
	}

	user, err := svc.GetOrCreateUser(context.Background(), claims)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID, "new user should have an assigned UUID")
	assert.Equal(t, "idp-new-999", user.IDPUserID)
	assert.Equal(t, "bob@example.com", user.Email)
	assert.Equal(t, "Bob", user.DisplayName)
	assert.True(t, user.IsActive)
	// Upsert was called: user now lives in repo.
	_, exists := repo.users["idp-new-999"]
	assert.True(t, exists, "user should have been persisted via Upsert")
}

func TestGetOrCreateUser_FindError_ReturnsError(t *testing.T) {
	repo := newMockRepo()
	repo.findErr = errors.New("db connection lost")

	svc := iam.NewUserService(repo)
	claims := &auth.Claims{Subject: "idp-xyz", Email: "x@y.com", Name: "X"}

	_, err := svc.GetOrCreateUser(context.Background(), claims)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "finding user")
}

func TestGetCurrentUser_ReturnsProfileWithMemberships(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	orgID := uuid.New()
	roleID := uuid.New()
	repo.users["idp-user-42"] = &iam.User{
		ID:          userID,
		IDPUserID:   "idp-user-42",
		Email:       "carol@example.com",
		DisplayName: "Carol",
		IsActive:    true,
	}
	repo.memberships[userID] = []iam.Membership{
		{
			ID:       uuid.New(),
			UserID:   userID,
			OrgID:    orgID,
			RoleID:   roleID,
			RoleName: "admin",
			Status:   "active",
		},
	}

	svc := iam.NewUserService(repo)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "idp-user-42",
		Email:   "carol@example.com",
		Name:    "Carol",
	})

	profile, err := svc.GetCurrentUser(ctx)

	require.NoError(t, err)
	assert.Equal(t, userID, profile.ID)
	assert.Equal(t, "carol@example.com", profile.Email)
	assert.Equal(t, "Carol", profile.DisplayName)
	require.Len(t, profile.Memberships, 1)
	assert.Equal(t, "admin", profile.Memberships[0].RoleName)
}

func TestGetCurrentUser_NoClaimsInContext(t *testing.T) {
	repo := newMockRepo()
	svc := iam.NewUserService(repo)

	_, err := svc.GetCurrentUser(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no claims in context")
}

func TestGetCurrentUser_UserNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := iam.NewUserService(repo)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "idp-does-not-exist",
		Email:   "ghost@example.com",
		Name:    "Ghost",
	})

	_, err := svc.GetCurrentUser(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestUpdateProfile_UpdatesDisplayName(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	repo.users["idp-user-77"] = &iam.User{
		ID:          userID,
		IDPUserID:   "idp-user-77",
		Email:       "dave@example.com",
		DisplayName: "Dave",
		IsActive:    true,
	}

	svc := iam.NewUserService(repo)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "idp-user-77",
		Email:   "dave@example.com",
		Name:    "Dave",
	})

	newName := "David"
	profile, err := svc.UpdateProfile(ctx, iam.UpdateProfileRequest{
		DisplayName: &newName,
	})

	require.NoError(t, err)
	assert.Equal(t, "David", profile.DisplayName)
	// Verify the change was persisted in the mock.
	assert.Equal(t, "David", repo.users["idp-user-77"].DisplayName)
}

func TestUpdateProfile_NoClaimsInContext(t *testing.T) {
	repo := newMockRepo()
	svc := iam.NewUserService(repo)

	newName := "Nobody"
	_, err := svc.UpdateProfile(context.Background(), iam.UpdateProfileRequest{
		DisplayName: &newName,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no claims in context")
}

func TestUpdateProfile_UserNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := iam.NewUserService(repo)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "idp-nobody",
		Email:   "nobody@example.com",
		Name:    "Nobody",
	})

	newName := "Test"
	_, err := svc.UpdateProfile(ctx, iam.UpdateProfileRequest{DisplayName: &newName})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}
