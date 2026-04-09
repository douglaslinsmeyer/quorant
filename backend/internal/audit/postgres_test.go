//go:build integration

package audit_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM audit_log")
		pool.Close()
	})

	return pool
}

func newEntry(orgID, actorID, resourceID uuid.UUID) audit.AuditEntry {
	return audit.AuditEntry{
		OrgID:        orgID,
		ActorID:      actorID,
		Action:       "assessment.created",
		ResourceType: "assessment",
		ResourceID:   resourceID,
		Module:       "fin",
		OccurredAt:   time.Now().UTC().Truncate(time.Millisecond),
	}
}

// ─── TestRecord_InsertsAuditEntry ────────────────────────────────────────────

func TestRecord_InsertsAuditEntry(t *testing.T) {
	pool := setupTestDB(t)
	auditor := audit.NewPostgresAuditor(pool)
	ctx := context.Background()

	orgID := uuid.New()
	actorID := uuid.New()
	resourceID := uuid.New()

	entry := newEntry(orgID, actorID, resourceID)
	err := auditor.Record(ctx, entry)
	require.NoError(t, err)

	results, err := auditor.Query(ctx, audit.AuditQuery{OrgID: &orgID})
	require.NoError(t, err)
	require.Len(t, results, 1)

	got := results[0]
	assert.Equal(t, orgID, got.OrgID)
	assert.Equal(t, actorID, got.ActorID)
	assert.Equal(t, "assessment.created", got.Action)
	assert.Equal(t, "assessment", got.ResourceType)
	assert.Equal(t, resourceID, got.ResourceID)
	assert.Equal(t, "fin", got.Module)
	assert.Nil(t, got.ImpersonatorID)
	assert.NotEqual(t, uuid.Nil, got.EventID, "event_id should be auto-generated")
	assert.Greater(t, got.ID, int64(0), "id should be a positive auto-generated identity")
}

// ─── TestRecord_WithImpersonator ─────────────────────────────────────────────

func TestRecord_WithImpersonator(t *testing.T) {
	pool := setupTestDB(t)
	auditor := audit.NewPostgresAuditor(pool)
	ctx := context.Background()

	orgID := uuid.New()
	actorID := uuid.New()
	impersonatorID := uuid.New()
	resourceID := uuid.New()

	entry := newEntry(orgID, actorID, resourceID)
	entry.ImpersonatorID = &impersonatorID

	err := auditor.Record(ctx, entry)
	require.NoError(t, err)

	results, err := auditor.Query(ctx, audit.AuditQuery{OrgID: &orgID})
	require.NoError(t, err)
	require.Len(t, results, 1)

	got := results[0]
	require.NotNil(t, got.ImpersonatorID)
	assert.Equal(t, impersonatorID, *got.ImpersonatorID)
}

// ─── TestRecord_WithBeforeAfterState ─────────────────────────────────────────

func TestRecord_WithBeforeAfterState(t *testing.T) {
	pool := setupTestDB(t)
	auditor := audit.NewPostgresAuditor(pool)
	ctx := context.Background()

	orgID := uuid.New()
	actorID := uuid.New()
	resourceID := uuid.New()

	beforeState := json.RawMessage(`{"status":"draft","amount":100}`)
	afterState := json.RawMessage(`{"status":"approved","amount":200}`)

	entry := newEntry(orgID, actorID, resourceID)
	entry.BeforeState = beforeState
	entry.AfterState = afterState

	err := auditor.Record(ctx, entry)
	require.NoError(t, err)

	results, err := auditor.Query(ctx, audit.AuditQuery{OrgID: &orgID})
	require.NoError(t, err)
	require.Len(t, results, 1)

	got := results[0]

	// Compare as unmarshalled maps to avoid whitespace/ordering differences in JSONB round-trips.
	var wantBefore, gotBefore, wantAfter, gotAfter map[string]any
	require.NoError(t, json.Unmarshal(beforeState, &wantBefore))
	require.NoError(t, json.Unmarshal(got.BeforeState, &gotBefore))
	require.NoError(t, json.Unmarshal(afterState, &wantAfter))
	require.NoError(t, json.Unmarshal(got.AfterState, &gotAfter))

	assert.Equal(t, wantBefore, gotBefore)
	assert.Equal(t, wantAfter, gotAfter)
}

// ─── TestQuery_FiltersByOrg ───────────────────────────────────────────────────

func TestQuery_FiltersByOrg(t *testing.T) {
	pool := setupTestDB(t)
	auditor := audit.NewPostgresAuditor(pool)
	ctx := context.Background()

	orgA := uuid.New()
	orgB := uuid.New()
	actorID := uuid.New()

	// Insert two entries for orgA and one for orgB.
	for i := 0; i < 2; i++ {
		err := auditor.Record(ctx, newEntry(orgA, actorID, uuid.New()))
		require.NoError(t, err)
	}
	err := auditor.Record(ctx, newEntry(orgB, actorID, uuid.New()))
	require.NoError(t, err)

	results, err := auditor.Query(ctx, audit.AuditQuery{OrgID: &orgA})
	require.NoError(t, err)
	assert.Len(t, results, 2, "should return only entries for orgA")
	for _, r := range results {
		assert.Equal(t, orgA, r.OrgID)
	}
}

// ─── TestQuery_FiltersByResourceType ─────────────────────────────────────────

func TestQuery_FiltersByResourceType(t *testing.T) {
	pool := setupTestDB(t)
	auditor := audit.NewPostgresAuditor(pool)
	ctx := context.Background()

	orgID := uuid.New()
	actorID := uuid.New()

	// Insert one assessment and one invoice entry.
	assessmentEntry := newEntry(orgID, actorID, uuid.New())
	assessmentEntry.ResourceType = "assessment"
	assessmentEntry.Action = "assessment.created"
	require.NoError(t, auditor.Record(ctx, assessmentEntry))

	invoiceEntry := newEntry(orgID, actorID, uuid.New())
	invoiceEntry.ResourceType = "invoice"
	invoiceEntry.Action = "invoice.created"
	require.NoError(t, auditor.Record(ctx, invoiceEntry))

	resType := "assessment"
	results, err := auditor.Query(ctx, audit.AuditQuery{
		OrgID:        &orgID,
		ResourceType: &resType,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "assessment", results[0].ResourceType)
}

// ─── TestQuery_Pagination ─────────────────────────────────────────────────────

func TestQuery_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	auditor := audit.NewPostgresAuditor(pool)
	ctx := context.Background()

	orgID := uuid.New()
	actorID := uuid.New()

	// Insert 5 entries with small time gaps to ensure stable ordering.
	for i := 0; i < 5; i++ {
		entry := newEntry(orgID, actorID, uuid.New())
		entry.OccurredAt = time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		require.NoError(t, auditor.Record(ctx, entry))
	}

	// First page: 2 entries.
	page1, err := auditor.Query(ctx, audit.AuditQuery{OrgID: &orgID, Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	// Second page: next 2 entries.
	page2, err := auditor.Query(ctx, audit.AuditQuery{OrgID: &orgID, Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	// Third page: last 1 entry.
	page3, err := auditor.Query(ctx, audit.AuditQuery{OrgID: &orgID, Limit: 2, Offset: 4})
	require.NoError(t, err)
	assert.Len(t, page3, 1)

	// All IDs across pages should be distinct.
	allIDs := map[int64]bool{}
	for _, e := range append(append(page1, page2...), page3...) {
		assert.False(t, allIDs[e.ID], "duplicate entry id %d across pages", e.ID)
		allIDs[e.ID] = true
	}
	assert.Len(t, allIDs, 5)
}
