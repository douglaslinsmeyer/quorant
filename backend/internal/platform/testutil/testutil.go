// Package testutil provides shared test helpers to reduce boilerplate across module tests.
// Import this package ONLY in _test.go files.
package testutil

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
)

// DiscardLogger returns a slog.Logger that writes to io.Discard.
// Use in tests where log output is not needed.
func DiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// NoopAuditor returns an audit.Auditor that discards all entries.
func NoopAuditor() audit.Auditor {
	return audit.NewNoopAuditor()
}

// InMemoryPublisher returns a queue.Publisher that records events in memory.
func InMemoryPublisher() *queue.InMemoryPublisher {
	return queue.NewInMemoryPublisher()
}

// TestUserID returns a fixed UUID for use as a test user ID.
// Using a fixed ID makes test assertions predictable.
func TestUserID() uuid.UUID {
	return uuid.MustParse("00000000-0000-0000-0000-000000000099")
}

// TestOrgID returns a fixed UUID for use as a test org ID.
func TestOrgID() uuid.UUID {
	return uuid.MustParse("00000000-0000-0000-0000-000000000088")
}

// AuthContext returns a context with auth claims and user ID set,
// simulating what the Auth + RBAC middleware would provide.
func AuthContext(userID uuid.UUID) context.Context {
	ctx := context.Background()
	ctx = auth.WithClaims(ctx, &auth.Claims{
		Subject: "test-idp-" + userID.String(),
		Email:   "test@example.com",
		Name:    "Test User",
	})
	ctx = middleware.WithUserID(ctx, userID)
	return ctx
}

// AuthContextWithOrg returns a context with auth claims, user ID, and org ID set.
func AuthContextWithOrg(userID, orgID uuid.UUID) context.Context {
	ctx := AuthContext(userID)
	ctx = middleware.WithOrgID(ctx, orgID)
	return ctx
}

// IntegrationDB connects to the Docker Compose test database.
// Skips the test if the DB is not available (for -short mode).
// Returns a pool that is closed on test cleanup.
func IntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("integration DB not available: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("integration DB not reachable: %v", err)
	}

	t.Cleanup(func() { pool.Close() })

	require.NoError(t, err)
	return pool
}
