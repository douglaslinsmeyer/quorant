//go:build integration

package gov_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/gov"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestARBRequest builds a minimal ARBRequest for insertion.
func newTestARBRequest(f govTestFixture) *gov.ARBRequest {
	return &gov.ARBRequest{
		OrgID:       f.orgID,
		UnitID:      f.unitID,
		SubmittedBy: f.userID,
		Title:       "Deck addition",
		Description: "Adding a 200 sqft deck to the rear of the home",
		Category:    "structural",
		Status:      "submitted",
		Conditions:  json.RawMessage(`[]`),
	}
}

// ─── TestCreateARBRequest + FindByID ─────────────────────────────────────────

func TestCreateARBRequest(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	input := newTestARBRequest(f)
	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, f.userID, got.SubmittedBy)
	assert.Equal(t, "Deck addition", got.Title)
	assert.Equal(t, "structural", got.Category)
	assert.Equal(t, "submitted", got.Status)
	assert.False(t, got.AutoApproved)
	assert.NotNil(t, got.Conditions)
	assert.NotNil(t, got.Metadata)
	assert.NotNil(t, got.SupportingDocIDs)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
	assert.Nil(t, got.DeletedAt)
}

func TestFindARBRequestByID_Found(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestARBRequest(f))
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Deck addition", got.Title)
}

func TestFindARBRequestByID_NotFound(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown ID")
}

// ─── TestListARBRequestsByOrg ─────────────────────────────────────────────────

func TestListARBRequestsByOrg(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, newTestARBRequest(f))
	require.NoError(t, err)
	_, err = repo.Create(ctx, newTestARBRequest(f))
	require.NoError(t, err)

	list, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, r := range list {
		assert.Equal(t, f.orgID, r.OrgID)
	}
}

func TestListARBRequestsByOrg_EmptyForUnknown(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	list, err := repo.ListByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.Empty(t, list)
}

// ─── TestUpdateARBRequest ─────────────────────────────────────────────────────

func TestUpdateARBRequest_ChangeStatusToApproved(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestARBRequest(f))
	require.NoError(t, err)

	notes := "Approved with standard conditions"
	created.Status = "approved"
	created.ReviewedBy = &f.userID
	created.DecisionNotes = &notes

	updated, err := repo.Update(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "approved", updated.Status)
	assert.Equal(t, &f.userID, updated.ReviewedBy)
	assert.Equal(t, &notes, updated.DecisionNotes)
	assert.True(t, updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt))
}

func TestSoftDeleteARBRequest(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestARBRequest(f))
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted ARB request should not be returned by FindByID")
}

// ─── TestCreateARBVote + ListVotes ────────────────────────────────────────────

func TestCreateARBVote(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	request, err := repo.Create(ctx, newTestARBRequest(f))
	require.NoError(t, err)

	notes := "Looks good to me"
	vote := &gov.ARBVote{
		ARBRequestID: request.ID,
		VoterID:      f.userID,
		Vote:         "approve",
		Notes:        &notes,
	}

	got, err := repo.CreateVote(ctx, vote)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, request.ID, got.ARBRequestID)
	assert.Equal(t, f.userID, got.VoterID)
	assert.Equal(t, "approve", got.Vote)
	assert.Equal(t, &notes, got.Notes)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListVotesByRequest(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	request, err := repo.Create(ctx, newTestARBRequest(f))
	require.NoError(t, err)

	// Create a second voter.
	var voter2ID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3) RETURNING id`,
		"test-idp-voter2-"+uuid.New().String(),
		"voter2-"+uuid.New().String()+"@example.com",
		"Voter Two",
	).Scan(&voter2ID)
	require.NoError(t, err)

	_, err = repo.CreateVote(ctx, &gov.ARBVote{
		ARBRequestID: request.ID,
		VoterID:      f.userID,
		Vote:         "approve",
	})
	require.NoError(t, err)

	_, err = repo.CreateVote(ctx, &gov.ARBVote{
		ARBRequestID: request.ID,
		VoterID:      voter2ID,
		Vote:         "deny",
	})
	require.NoError(t, err)

	votes, err := repo.ListVotesByRequest(ctx, request.ID)

	require.NoError(t, err)
	require.Len(t, votes, 2)
	assert.Equal(t, request.ID, votes[0].ARBRequestID)
	assert.Equal(t, request.ID, votes[1].ARBRequestID)
}

// ─── TestARBVote_UniqueConstraint ─────────────────────────────────────────────

func TestARBVote_UniqueConstraint(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresARBRepository(f.pool)
	ctx := context.Background()

	request, err := repo.Create(ctx, newTestARBRequest(f))
	require.NoError(t, err)

	// First vote — should succeed.
	_, err = repo.CreateVote(ctx, &gov.ARBVote{
		ARBRequestID: request.ID,
		VoterID:      f.userID,
		Vote:         "approve",
	})
	require.NoError(t, err, "first vote should succeed")

	// Second vote from the same voter — should be rejected by unique constraint.
	_, err = repo.CreateVote(ctx, &gov.ARBVote{
		ARBRequestID: request.ID,
		VoterID:      f.userID,
		Vote:         "deny",
	})
	assert.Error(t, err, "duplicate vote should be rejected by unique constraint (arb_request_id, voter_id)")
}
