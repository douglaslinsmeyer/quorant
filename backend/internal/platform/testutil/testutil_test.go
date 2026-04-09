package testutil_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscardLogger(t *testing.T) {
	logger := testutil.DiscardLogger()
	require.NotNil(t, logger)
	// Should not panic
	logger.Info("test message", "key", "value")
}

func TestNoopAuditor(t *testing.T) {
	auditor := testutil.NoopAuditor()
	require.NotNil(t, auditor)
}

func TestInMemoryPublisher(t *testing.T) {
	pub := testutil.InMemoryPublisher()
	require.NotNil(t, pub)
	assert.Empty(t, pub.Events())
}

func TestTestUserID(t *testing.T) {
	id := testutil.TestUserID()
	assert.NotEqual(t, uuid.Nil, id)
}

func TestAuthContext(t *testing.T) {
	userID := testutil.TestUserID()
	ctx := testutil.AuthContext(userID)

	claims, ok := auth.ClaimsFromContext(ctx)
	require.True(t, ok)
	assert.Contains(t, claims.Subject, userID.String())

	resolvedID := middleware.UserIDFromContext(ctx)
	assert.Equal(t, userID, resolvedID)
}

func TestAuthContextWithOrg(t *testing.T) {
	userID := testutil.TestUserID()
	orgID := testutil.TestOrgID()
	ctx := testutil.AuthContextWithOrg(userID, orgID)

	resolvedOrg := middleware.OrgIDFromContext(ctx)
	assert.Equal(t, orgID, resolvedOrg)

	resolvedUser := middleware.UserIDFromContext(ctx)
	assert.Equal(t, userID, resolvedUser)
}
