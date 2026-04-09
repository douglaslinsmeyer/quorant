//go:build integration

package gov_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/gov"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestBallot builds a minimal Ballot for insertion.
func newTestBallot(f govTestFixture) *gov.Ballot {
	now := time.Now().UTC()
	return &gov.Ballot{
		OrgID:        f.orgID,
		Title:        "Board Member Election",
		Description:  "Annual election for open board seats",
		BallotType:   "election",
		Status:       "draft",
		Options:      json.RawMessage(`[{"id":"a","label":"Candidate A"},{"id":"b","label":"Candidate B"}]`),
		EligibleRole: "homeowner",
		OpensAt:      now.Add(24 * time.Hour),
		ClosesAt:     now.Add(7 * 24 * time.Hour),
		VotesCast:    0,
		WeightMethod: "equal",
		CreatedBy:    f.userID,
	}
}

// ─── TestCreateBallot + FindByID + ListByOrg ──────────────────────────────────

func TestCreateBallot(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	input := newTestBallot(f)
	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "Board Member Election", got.Title)
	assert.Equal(t, "election", got.BallotType)
	assert.Equal(t, "draft", got.Status)
	assert.Equal(t, "homeowner", got.EligibleRole)
	assert.Equal(t, "equal", got.WeightMethod)
	assert.Equal(t, 0, got.VotesCast)
	assert.Equal(t, f.userID, got.CreatedBy)
	assert.NotNil(t, got.Options)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
	assert.Nil(t, got.DeletedAt)
}

func TestFindBallotByID_Found(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Board Member Election", got.Title)
}

func TestFindBallotByID_NotFound(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown ID")
}

func TestListBallotsByOrg(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)
	_, err = repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	list, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, b := range list {
		assert.Equal(t, f.orgID, b.OrgID)
	}
}

func TestListBallotsByOrg_EmptyForUnknown(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	list, err := repo.ListByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestUpdateBallot_ChangeStatusToOpen(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	created.Status = "open"
	updated, err := repo.Update(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "open", updated.Status)
}

func TestSoftDeleteBallot(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted ballot should not be returned by FindByID")
}

// ─── TestCastVote + ListVotes ─────────────────────────────────────────────────

func TestCastVote(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	ballot, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	vote := &gov.BallotVote{
		BallotID:   ballot.ID,
		VoterID:    f.userID,
		UnitID:     f.unitID,
		Selection:  json.RawMessage(`{"choice":"a"}`),
		VoteWeight: 1.0,
	}

	got, err := repo.CastVote(ctx, vote)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, ballot.ID, got.BallotID)
	assert.Equal(t, f.userID, got.VoterID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.NotNil(t, got.Selection)
	assert.Equal(t, 1.0, got.VoteWeight)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListVotesByBallot(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	ballot, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	// Create a second unit so we can cast two votes.
	var unit2ID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, unit_type, status)
		 VALUES ($1, $2, 'single_family', 'occupied') RETURNING id`,
		f.orgID,
		"Unit-BV2-"+uuid.New().String(),
	).Scan(&unit2ID)
	require.NoError(t, err)

	var voter2ID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3) RETURNING id`,
		"test-idp-ballot-voter2-"+uuid.New().String(),
		"ballot-voter2-"+uuid.New().String()+"@example.com",
		"Ballot Voter Two",
	).Scan(&voter2ID)
	require.NoError(t, err)

	_, err = repo.CastVote(ctx, &gov.BallotVote{
		BallotID:   ballot.ID,
		VoterID:    f.userID,
		UnitID:     f.unitID,
		Selection:  json.RawMessage(`{"choice":"a"}`),
		VoteWeight: 1.0,
	})
	require.NoError(t, err)

	_, err = repo.CastVote(ctx, &gov.BallotVote{
		BallotID:   ballot.ID,
		VoterID:    voter2ID,
		UnitID:     unit2ID,
		Selection:  json.RawMessage(`{"choice":"b"}`),
		VoteWeight: 1.0,
	})
	require.NoError(t, err)

	votes, err := repo.ListVotesByBallot(ctx, ballot.ID)

	require.NoError(t, err)
	require.Len(t, votes, 2)
	for _, v := range votes {
		assert.Equal(t, ballot.ID, v.BallotID)
	}
}

// ─── TestCastVote_DuplicateUnit ───────────────────────────────────────────────

func TestCastVote_DuplicateUnit(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	ballot, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	// First vote — should succeed.
	_, err = repo.CastVote(ctx, &gov.BallotVote{
		BallotID:   ballot.ID,
		VoterID:    f.userID,
		UnitID:     f.unitID,
		Selection:  json.RawMessage(`{"choice":"a"}`),
		VoteWeight: 1.0,
	})
	require.NoError(t, err, "first vote should succeed")

	// Second vote from the same unit — should fail due to UNIQUE (ballot_id, unit_id).
	var voter2ID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3) RETURNING id`,
		"test-idp-dup-"+uuid.New().String(),
		"dup-voter-"+uuid.New().String()+"@example.com",
		"Dup Voter",
	).Scan(&voter2ID)
	require.NoError(t, err)

	_, err = repo.CastVote(ctx, &gov.BallotVote{
		BallotID:   ballot.ID,
		VoterID:    voter2ID,
		UnitID:     f.unitID, // same unit — triggers unique constraint
		Selection:  json.RawMessage(`{"choice":"b"}`),
		VoteWeight: 1.0,
	})
	assert.Error(t, err, "duplicate unit vote should be rejected by unique constraint (ballot_id, unit_id)")
}

// ─── TestFileProxy + ListProxies ──────────────────────────────────────────────

func TestFileProxy(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	ballot, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	var proxyUserID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3) RETURNING id`,
		"test-idp-proxy-"+uuid.New().String(),
		"proxy-user-"+uuid.New().String()+"@example.com",
		"Proxy User",
	).Scan(&proxyUserID)
	require.NoError(t, err)

	proxy := &gov.ProxyAuthorization{
		BallotID:  ballot.ID,
		UnitID:    f.unitID,
		GrantorID: f.userID,
		ProxyID:   proxyUserID,
		FiledAt:   time.Now().UTC(),
	}

	got, err := repo.FileProxy(ctx, proxy)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, ballot.ID, got.BallotID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, f.userID, got.GrantorID)
	assert.Equal(t, proxyUserID, got.ProxyID)
	assert.Nil(t, got.RevokedAt)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListProxiesByBallot(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	ballot, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	// Create a second unit and proxy user.
	var unit2ID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, unit_type, status)
		 VALUES ($1, $2, 'single_family', 'occupied') RETURNING id`,
		f.orgID,
		"Unit-PX2-"+uuid.New().String(),
	).Scan(&unit2ID)
	require.NoError(t, err)

	var proxyUserID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3) RETURNING id`,
		"test-idp-pxlist-"+uuid.New().String(),
		"px-list-"+uuid.New().String()+"@example.com",
		"Proxy List User",
	).Scan(&proxyUserID)
	require.NoError(t, err)

	_, err = repo.FileProxy(ctx, &gov.ProxyAuthorization{
		BallotID:  ballot.ID,
		UnitID:    f.unitID,
		GrantorID: f.userID,
		ProxyID:   proxyUserID,
		FiledAt:   time.Now().UTC(),
	})
	require.NoError(t, err)

	_, err = repo.FileProxy(ctx, &gov.ProxyAuthorization{
		BallotID:  ballot.ID,
		UnitID:    unit2ID,
		GrantorID: f.userID,
		ProxyID:   proxyUserID,
		FiledAt:   time.Now().UTC(),
	})
	require.NoError(t, err)

	proxies, err := repo.ListProxiesByBallot(ctx, ballot.ID)

	require.NoError(t, err)
	require.Len(t, proxies, 2)
	for _, p := range proxies {
		assert.Equal(t, ballot.ID, p.BallotID)
	}
}

// ─── TestRevokeProxy ─────────────────────────────────────────────────────────

func TestRevokeProxy(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresBallotRepository(f.pool)
	ctx := context.Background()

	ballot, err := repo.Create(ctx, newTestBallot(f))
	require.NoError(t, err)

	var proxyUserID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3) RETURNING id`,
		"test-idp-revoke-"+uuid.New().String(),
		"revoke-user-"+uuid.New().String()+"@example.com",
		"Revoke User",
	).Scan(&proxyUserID)
	require.NoError(t, err)

	proxy, err := repo.FileProxy(ctx, &gov.ProxyAuthorization{
		BallotID:  ballot.ID,
		UnitID:    f.unitID,
		GrantorID: f.userID,
		ProxyID:   proxyUserID,
		FiledAt:   time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Nil(t, proxy.RevokedAt, "newly filed proxy should not be revoked")

	// Revoke it.
	err = repo.RevokeProxy(ctx, proxy.ID)
	require.NoError(t, err)

	// Verify revoked_at is now set by fetching via raw query.
	var revokedAt *time.Time
	err = f.pool.QueryRow(ctx,
		`SELECT revoked_at FROM proxy_authorizations WHERE id = $1`, proxy.ID,
	).Scan(&revokedAt)
	require.NoError(t, err)
	assert.NotNil(t, revokedAt, "revoked_at should be set after RevokeProxy")
}
