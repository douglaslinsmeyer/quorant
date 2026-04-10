package estoppel_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flEstoppelRules returns a minimal EstoppelRules representative of Florida's
// statutory parameters, matching the seed data in migration 20260409000027.
func flEstoppelRules() *estoppel.EstoppelRules {
	rushDays := 3
	effectiveDays := 30
	return &estoppel.EstoppelRules{
		StandardTurnaroundBusinessDays: 10,
		StandardFeeCents:               29900,
		RushTurnaroundBusinessDays:     &rushDays,
		RushFeeCents:                   11900,
		DelinquentSurchargeCents:       17900,
		EffectivePeriodDays:            &effectiveDays,
		ElectronicDeliveryRequired:     true,
		StatutoryFormRequired:          true,
		StatutoryFormID:                "fl_720_30851",
		FreeAmendmentOnError:           true,
		StatuteRef:                     "§720.30851/§718.116(8)",
	}
}

// ---------------------------------------------------------------------------
// mockJurisdictionRulesRepo tests
// ---------------------------------------------------------------------------

func TestMockJurisdictionRulesRepo_KnownJurisdiction(t *testing.T) {
	fl := flEstoppelRules()
	repo := estoppel.NewMockJurisdictionRulesRepo(map[string]*estoppel.EstoppelRules{
		"FL": fl,
	})

	var jr estoppel.JurisdictionRulesRepository = repo
	got, err := jr.GetEstoppelRules(context.Background(), "FL")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, fl.StandardFeeCents, got.StandardFeeCents)
	assert.Equal(t, fl.StandardTurnaroundBusinessDays, got.StandardTurnaroundBusinessDays)
	assert.Equal(t, fl.RushFeeCents, got.RushFeeCents)
	assert.Equal(t, fl.DelinquentSurchargeCents, got.DelinquentSurchargeCents)
}

func TestMockJurisdictionRulesRepo_UnknownJurisdiction_ReturnsNil(t *testing.T) {
	repo := estoppel.NewMockJurisdictionRulesRepo(map[string]*estoppel.EstoppelRules{
		"FL": flEstoppelRules(),
	})

	var jr estoppel.JurisdictionRulesRepository = repo
	got, err := jr.GetEstoppelRules(context.Background(), "ZZ")

	require.NoError(t, err)
	assert.Nil(t, got, "expected nil rules for unknown jurisdiction")
}

func TestMockJurisdictionRulesRepo_EmptyMap_ReturnsNil(t *testing.T) {
	repo := estoppel.NewMockJurisdictionRulesRepo(nil)

	var jr estoppel.JurisdictionRulesRepository = repo
	got, err := jr.GetEstoppelRules(context.Background(), "FL")

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMockJurisdictionRulesRepo_MultipleJurisdictions(t *testing.T) {
	txDays := 10
	repo := estoppel.NewMockJurisdictionRulesRepo(map[string]*estoppel.EstoppelRules{
		"FL": flEstoppelRules(),
		"TX": {
			StandardTurnaroundBusinessDays: txDays,
			StandardFeeCents:               37500,
			StatuteRef:                     "Property Code §207.003",
		},
	})

	var jr estoppel.JurisdictionRulesRepository = repo

	fl, err := jr.GetEstoppelRules(context.Background(), "FL")
	require.NoError(t, err)
	require.NotNil(t, fl)
	assert.Equal(t, int64(29900), fl.StandardFeeCents)

	tx, err := jr.GetEstoppelRules(context.Background(), "TX")
	require.NoError(t, err)
	require.NotNil(t, tx)
	assert.Equal(t, int64(37500), tx.StandardFeeCents)
}

// ---------------------------------------------------------------------------
// Interface compliance check
// ---------------------------------------------------------------------------

var _ estoppel.JurisdictionRulesRepository = (*estoppel.MockJurisdictionRulesRepo)(nil)

// ---------------------------------------------------------------------------
// ResolveRules integration tests (uses mockJurisdictionRulesRepo + service)
// ---------------------------------------------------------------------------

func TestResolveRules_KnownJurisdiction(t *testing.T) {
	fl := flEstoppelRules()
	repo := estoppel.NewMockJurisdictionRulesRepo(map[string]*estoppel.EstoppelRules{"FL": fl})

	svc := newTestServiceWithJurisdiction(repo, "FL")

	orgID := uuid.New()
	unitID := uuid.New()
	rules, err := svc.ResolveRules(context.Background(), orgID, unitID)

	require.NoError(t, err)
	require.NotNil(t, rules)
	assert.Equal(t, fl.StandardFeeCents, rules.StandardFeeCents)
}

func TestResolveRules_UnknownJurisdiction_ReturnsError(t *testing.T) {
	// Repo has no rules for the state the property returns.
	repo := estoppel.NewMockJurisdictionRulesRepo(nil)
	svc := newTestServiceWithJurisdiction(repo, "ZZ")

	orgID := uuid.New()
	unitID := uuid.New()
	rules, err := svc.ResolveRules(context.Background(), orgID, unitID)

	require.Error(t, err)
	assert.Nil(t, rules)
}

func TestResolveRules_EmptyOrgState_ReturnsError(t *testing.T) {
	// Property provider returns empty OrgState.
	repo := estoppel.NewMockJurisdictionRulesRepo(map[string]*estoppel.EstoppelRules{"FL": flEstoppelRules()})
	svc := newTestServiceWithJurisdiction(repo, "") // empty state

	orgID := uuid.New()
	unitID := uuid.New()
	rules, err := svc.ResolveRules(context.Background(), orgID, unitID)

	require.Error(t, err)
	assert.Nil(t, rules)
}
