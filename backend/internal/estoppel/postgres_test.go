//go:build integration

package estoppel_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Setup ────────────────────────────────────────────────────────────────────

// estoppelTestFixture holds all resources needed by repository tests.
type estoppelTestFixture struct {
	repo       estoppel.EstoppelRepository
	orgID      uuid.UUID
	unitID     uuid.UUID
	userID     uuid.UUID
	documentID uuid.UUID
	pool       *pgxpool.Pool
}

// setupEstoppelDB connects to the local Docker postgres and registers cleanup.
func setupEstoppelDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM estoppel_certificates")
		pool.Exec(cleanCtx, "DELETE FROM estoppel_requests")
		pool.Exec(cleanCtx, "DELETE FROM documents")
		pool.Exec(cleanCtx, "DELETE FROM unit_memberships")
		pool.Exec(cleanCtx, "DELETE FROM units")
		pool.Exec(cleanCtx, "DELETE FROM organizations WHERE parent_id IS NOT NULL")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Exec(cleanCtx, "DELETE FROM users")
		pool.Close()
	})

	return pool
}

// setupEstoppelFixture creates a pool, test org, user, unit, and document.
func setupEstoppelFixture(t *testing.T) estoppelTestFixture {
	t.Helper()
	ctx := context.Background()
	pool := setupEstoppelDB(t)

	// Create a test organization.
	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', 'Estoppel Test HOA', $1, $2, '{}')
		 RETURNING id`,
		"estoppel-test-hoa-"+uuid.New().String(),
		"estoppel_test_"+uuid.New().String()[:8],
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	// Create a test user.
	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"estoppel-test-idp-"+uuid.New().String(),
		"estoppel-test-"+uuid.New().String()+"@example.com",
		"Estoppel Test User",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	// Create a test unit.
	var unitID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status)
		 VALUES ($1, 'Unit 101', 'occupied')
		 RETURNING id`,
		orgID,
	).Scan(&unitID)
	require.NoError(t, err, "create test unit")

	// Create a test document (needed for estoppel_certificates).
	var documentID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO documents (org_id, uploaded_by, title, file_name, content_type, size_bytes, storage_key)
		 VALUES ($1, $2, 'Test Estoppel Certificate', 'estoppel.pdf', 'application/pdf', 1024, $3)
		 RETURNING id`,
		orgID,
		userID,
		"estoppel/test/"+uuid.New().String()+".pdf",
	).Scan(&documentID)
	require.NoError(t, err, "create test document")

	repo := estoppel.NewPostgresRepository(pool)

	return estoppelTestFixture{
		repo:       repo,
		orgID:      orgID,
		unitID:     unitID,
		userID:     userID,
		documentID: documentID,
		pool:       pool,
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func minimalRequest(orgID, unitID, userID uuid.UUID) *estoppel.EstoppelRequest {
	return &estoppel.EstoppelRequest{
		OrgID:           orgID,
		UnitID:          unitID,
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "Jane Title",
		RequestorEmail:  "jane@titleco.com",
		PropertyAddress: "123 Oak St, Anytown, FL 32801",
		OwnerName:       "John Homeowner",
		RushRequested:   false,
		Status:          "submitted",
		FeeCents:        20000,
		TotalFeeCents:   20000,
		DeadlineAt:      time.Now().UTC().Add(10 * 24 * time.Hour),
		Metadata:        map[string]any{},
		CreatedBy:       userID,
	}
}

func minimalCertificate(requestID, orgID, unitID, documentID, userID uuid.UUID) *estoppel.EstoppelCertificate {
	dataSnap, _ := json.Marshal(map[string]any{"fee_cents": 20000})
	narratives, _ := json.Marshal(map[string]any{"assessment_summary": []any{}})
	return &estoppel.EstoppelCertificate{
		RequestID:         requestID,
		OrgID:             orgID,
		UnitID:            unitID,
		DocumentID:        documentID,
		Jurisdiction:      "FL",
		EffectiveDate:     time.Now().UTC().Truncate(24 * time.Hour),
		DataSnapshot:      dataSnap,
		NarrativeSections: narratives,
		SignedBy:          userID,
		SignedAt:          time.Now().UTC(),
		SignerTitle:       "Property Manager",
		TemplateVersion:   "v1.0",
	}
}

// ─── TestCreateRequest ────────────────────────────────────────────────────────

func TestCreateRequest(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	input := minimalRequest(f.orgID, f.unitID, f.userID)
	got, err := f.repo.CreateRequest(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, "estoppel_certificate", got.RequestType)
	assert.Equal(t, "title_company", got.RequestorType)
	assert.Equal(t, "Jane Title", got.RequestorName)
	assert.Equal(t, "submitted", got.Status)
	assert.Equal(t, int64(20000), got.FeeCents)
	assert.Equal(t, int64(20000), got.TotalFeeCents)
	assert.False(t, got.RushRequested)
	assert.Equal(t, f.userID, got.CreatedBy)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
	assert.Nil(t, got.DeletedAt)
	assert.NotNil(t, got.Metadata)
}

// ─── TestFindRequestByID ──────────────────────────────────────────────────────

func TestFindRequestByID(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	got, err := f.repo.FindRequestByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "estoppel_certificate", got.RequestType)
}

func TestFindRequestByID_ReturnsNilWhenNotFound(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	got, err := f.repo.FindRequestByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown id")
}

// ─── TestUpdateRequestStatus ──────────────────────────────────────────────────

func TestUpdateRequestStatus(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)
	require.Equal(t, "submitted", created.Status)

	updated, err := f.repo.UpdateRequestStatus(ctx, created.ID, "data_aggregation")

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "data_aggregation", updated.Status)
	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt),
		"updated_at should be >= created_at after status update")
}

func TestUpdateRequestStatus_ErrorWhenNotFound(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	_, err := f.repo.UpdateRequestStatus(ctx, uuid.New(), "data_aggregation")

	require.Error(t, err, "should return error for unknown id")
}

// ─── TestListRequestsByOrg ────────────────────────────────────────────────────

func TestListRequestsByOrg(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	r1 := minimalRequest(f.orgID, f.unitID, f.userID)
	r1.RequestorName = "Requestor Alpha"
	_, err := f.repo.CreateRequest(ctx, r1)
	require.NoError(t, err)

	r2 := minimalRequest(f.orgID, f.unitID, f.userID)
	r2.RequestorName = "Requestor Beta"
	_, err = f.repo.CreateRequest(ctx, r2)
	require.NoError(t, err)

	list, hasMore, err := f.repo.ListRequestsByOrg(ctx, f.orgID, nil, 10, nil)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both requests")
	assert.False(t, hasMore, "hasMore should be false when all items fit on page")

	names := []string{list[0].RequestorName, list[1].RequestorName}
	assert.Contains(t, names, "Requestor Alpha")
	assert.Contains(t, names, "Requestor Beta")
}

func TestListRequestsByOrg_StatusFilter(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	// Create two requests — leave one as submitted, advance the other.
	_, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	r2, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)
	_, err = f.repo.UpdateRequestStatus(ctx, r2.ID, "data_aggregation")
	require.NoError(t, err)

	statusFilter := "submitted"
	list, hasMore, err := f.repo.ListRequestsByOrg(ctx, f.orgID, &statusFilter, 10, nil)

	require.NoError(t, err)
	require.Len(t, list, 1, "status filter should return only submitted requests")
	assert.False(t, hasMore)
	assert.Equal(t, "submitted", list[0].Status)
}

func TestListRequestsByOrg_Pagination(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	// Create 3 requests.
	for i := 0; i < 3; i++ {
		_, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
		require.NoError(t, err)
	}

	// Page 1: limit 2.
	page1, hasMore, err := f.repo.ListRequestsByOrg(ctx, f.orgID, nil, 2, nil)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.True(t, hasMore, "should indicate more items exist")

	// Page 2: cursor from last item on page 1.
	cursor := page1[len(page1)-1].ID
	page2, hasMore2, err := f.repo.ListRequestsByOrg(ctx, f.orgID, nil, 2, &cursor)
	require.NoError(t, err)
	require.Len(t, page2, 1, "second page should have 1 remaining item")
	assert.False(t, hasMore2)
}

func TestListRequestsByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	list, hasMore, err := f.repo.ListRequestsByOrg(ctx, f.orgID, nil, 10, nil)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
	assert.False(t, hasMore)
}

// ─── TestCreateCertificate ────────────────────────────────────────────────────

func TestCreateCertificate(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	req, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	input := minimalCertificate(req.ID, f.orgID, f.unitID, f.documentID, f.userID)
	got, err := f.repo.CreateCertificate(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, req.ID, got.RequestID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, f.documentID, got.DocumentID)
	assert.Equal(t, "FL", got.Jurisdiction)
	assert.Equal(t, f.userID, got.SignedBy)
	assert.Equal(t, "Property Manager", got.SignerTitle)
	assert.Equal(t, "v1.0", got.TemplateVersion)
	assert.False(t, got.CreatedAt.IsZero())
	assert.Nil(t, got.AmendmentOf)
}

// ─── TestFindCertificateByRequestID ──────────────────────────────────────────

func TestFindCertificateByRequestID(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	req, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	cert, err := f.repo.CreateCertificate(ctx, minimalCertificate(req.ID, f.orgID, f.unitID, f.documentID, f.userID))
	require.NoError(t, err)

	got, err := f.repo.FindCertificateByRequestID(ctx, req.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, cert.ID, got.ID)
	assert.Equal(t, req.ID, got.RequestID)
}

func TestFindCertificateByRequestID_ReturnsNilWhenNone(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	got, err := f.repo.FindCertificateByRequestID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got)
}
