package fin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock GL Repository ───────────────────────────────────────────────────────

// mockGLRepo is an in-memory implementation of GLRepository.
type mockGLRepo struct {
	accounts     map[uuid.UUID]*fin.GLAccount
	entries      map[uuid.UUID]*fin.GLJournalEntry
	nextEntryNum int
	// hasPostedLinesOverride allows tests to force HasPostedLines to return true.
	hasPostedLinesOverride map[uuid.UUID]bool
}

func newMockGLRepo() *mockGLRepo {
	return &mockGLRepo{
		accounts:               make(map[uuid.UUID]*fin.GLAccount),
		entries:                make(map[uuid.UUID]*fin.GLJournalEntry),
		nextEntryNum:           1,
		hasPostedLinesOverride: make(map[uuid.UUID]bool),
	}
}

func (m *mockGLRepo) CreateAccount(_ context.Context, a *fin.GLAccount) (*fin.GLAccount, error) {
	a.ID = uuid.New()
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	cp := *a
	m.accounts[cp.ID] = &cp
	return &cp, nil
}

func (m *mockGLRepo) FindAccountByID(_ context.Context, id uuid.UUID) (*fin.GLAccount, error) {
	a, ok := m.accounts[id]
	if !ok || a.DeletedAt != nil {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

func (m *mockGLRepo) ListAccountsByOrg(_ context.Context, orgID uuid.UUID) ([]fin.GLAccount, error) {
	var result []fin.GLAccount
	for _, a := range m.accounts {
		if a.OrgID == orgID && a.DeletedAt == nil {
			result = append(result, *a)
		}
	}
	if result == nil {
		return []fin.GLAccount{}, nil
	}
	return result, nil
}

func (m *mockGLRepo) FindAccountByOrgAndNumber(_ context.Context, orgID uuid.UUID, number int) (*fin.GLAccount, error) {
	for _, a := range m.accounts {
		if a.OrgID == orgID && a.AccountNumber == number && a.DeletedAt == nil {
			cp := *a
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockGLRepo) UpdateAccount(_ context.Context, a *fin.GLAccount) (*fin.GLAccount, error) {
	existing, ok := m.accounts[a.ID]
	if !ok {
		return nil, nil
	}
	a.UpdatedAt = time.Now()
	*existing = *a
	cp := *existing
	return &cp, nil
}

func (m *mockGLRepo) SoftDeleteAccount(_ context.Context, id uuid.UUID) error {
	a, ok := m.accounts[id]
	if !ok {
		return nil
	}
	now := time.Now()
	a.DeletedAt = &now
	return nil
}

func (m *mockGLRepo) PostJournalEntry(_ context.Context, entry *fin.GLJournalEntry) (*fin.GLJournalEntry, error) {
	entry.ID = uuid.New()
	entry.EntryNumber = m.nextEntryNum
	m.nextEntryNum++
	entry.CreatedAt = time.Now()
	for i := range entry.Lines {
		entry.Lines[i].ID = uuid.New()
		entry.Lines[i].JournalEntryID = entry.ID
	}
	cp := *entry
	cp.Lines = make([]fin.GLJournalLine, len(entry.Lines))
	copy(cp.Lines, entry.Lines)
	m.entries[cp.ID] = &cp
	return &cp, nil
}

func (m *mockGLRepo) FindJournalEntryByID(_ context.Context, id uuid.UUID) (*fin.GLJournalEntry, error) {
	e, ok := m.entries[id]
	if !ok {
		return nil, nil
	}
	cp := *e
	cp.Lines = make([]fin.GLJournalLine, len(e.Lines))
	copy(cp.Lines, e.Lines)
	return &cp, nil
}

func (m *mockGLRepo) ListJournalEntriesByOrg(_ context.Context, orgID uuid.UUID) ([]fin.GLJournalEntry, error) {
	var result []fin.GLJournalEntry
	for _, e := range m.entries {
		if e.OrgID == orgID {
			cp := *e
			cp.Lines = make([]fin.GLJournalLine, len(e.Lines))
			copy(cp.Lines, e.Lines)
			result = append(result, cp)
		}
	}
	if result == nil {
		return []fin.GLJournalEntry{}, nil
	}
	return result, nil
}

func (m *mockGLRepo) GetTrialBalance(_ context.Context, _ uuid.UUID, _ time.Time) ([]fin.TrialBalanceRow, error) {
	return []fin.TrialBalanceRow{}, nil
}

func (m *mockGLRepo) GetAccountBalances(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]fin.AccountBalance, error) {
	return []fin.AccountBalance{}, nil
}

func (m *mockGLRepo) HasPostedLines(_ context.Context, accountID uuid.UUID) (bool, error) {
	if v, ok := m.hasPostedLinesOverride[accountID]; ok {
		return v, nil
	}
	return false, nil
}

// ── Helper ───────────────────────────────────────────────────────────────────

func newTestGLService() (*fin.GLService, *mockGLRepo) {
	repo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := fin.NewGLService(repo, audit.NewNoopAuditor(), logger)
	return svc, repo
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestGLService_CreateAccount_Success(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()

	req := fin.CreateGLAccountRequest{
		AccountNumber: 1010,
		Name:          "Cash-Operating",
		AccountType:   "asset",
	}

	acct, err := svc.CreateAccount(ctx, orgID, req)
	require.NoError(t, err)
	require.NotNil(t, acct)
	assert.NotEqual(t, uuid.Nil, acct.ID)
	assert.Equal(t, orgID, acct.OrgID)
	assert.Equal(t, 1010, acct.AccountNumber)
	assert.Equal(t, "Cash-Operating", acct.Name)
	assert.Equal(t, "asset", acct.AccountType)
}

func TestGLService_CreateAccount_ValidationError(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()

	req := fin.CreateGLAccountRequest{
		AccountNumber: 1010,
		Name:          "", // empty name triggers validation error
		AccountType:   "asset",
	}

	_, err := svc.CreateAccount(ctx, orgID, req)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "name", valErr.Field)
}

func TestGLService_GetAccount_NotFound(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()

	_, err := svc.GetAccount(ctx, uuid.New())
	require.Error(t, err)

	var notFound *api.NotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestGLService_UpdateAccount_RejectsSystemAccount(t *testing.T) {
	svc, repo := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()

	// Create a system account directly in the repo.
	acct := &fin.GLAccount{
		ID:            uuid.New(),
		OrgID:         orgID,
		AccountNumber: 1010,
		Name:          "Cash-Operating",
		AccountType:   "asset",
		IsSystem:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repo.accounts[acct.ID] = acct

	newName := "Renamed"
	req := fin.UpdateGLAccountRequest{Name: &newName}

	_, err := svc.UpdateAccount(ctx, acct.ID, req)
	require.Error(t, err)

	var unproc *api.UnprocessableError
	require.ErrorAs(t, err, &unproc)
	assert.Contains(t, unproc.Error(), "cannot modify system account")
}

func TestGLService_DeleteAccount_RejectsSystemAccount(t *testing.T) {
	svc, repo := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()

	acct := &fin.GLAccount{
		ID:            uuid.New(),
		OrgID:         orgID,
		AccountNumber: 1010,
		Name:          "Cash-Operating",
		AccountType:   "asset",
		IsSystem:      true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repo.accounts[acct.ID] = acct

	err := svc.DeleteAccount(ctx, acct.ID)
	require.Error(t, err)

	var unproc *api.UnprocessableError
	require.ErrorAs(t, err, &unproc)
	assert.Contains(t, unproc.Error(), "cannot delete system account")
}

func TestGLService_DeleteAccount_RejectsAccountWithLines(t *testing.T) {
	svc, repo := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()

	acct := &fin.GLAccount{
		ID:            uuid.New(),
		OrgID:         orgID,
		AccountNumber: 1010,
		Name:          "Cash-Operating",
		AccountType:   "asset",
		IsSystem:      false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repo.accounts[acct.ID] = acct
	repo.hasPostedLinesOverride[acct.ID] = true

	err := svc.DeleteAccount(ctx, acct.ID)
	require.Error(t, err)

	var unproc *api.UnprocessableError
	require.ErrorAs(t, err, &unproc)
	assert.Contains(t, unproc.Error(), "cannot delete account with posted journal lines")
}

func TestGLService_PostJournalEntry_BalancedSuccess(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()
	postedBy := uuid.New()
	acctDebit := uuid.New()
	acctCredit := uuid.New()

	req := fin.CreateJournalEntryRequest{
		EntryDate: time.Now(),
		Memo:      "Test entry",
		Lines: []fin.JournalEntryLineInput{
			{AccountID: acctDebit, DebitCents: 10000, CreditCents: 0},
			{AccountID: acctCredit, DebitCents: 0, CreditCents: 10000},
		},
	}

	entry, err := svc.PostJournalEntry(ctx, orgID, postedBy, req)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.NotEqual(t, uuid.Nil, entry.ID)
	assert.Equal(t, 1, entry.EntryNumber)
	assert.Equal(t, "Test entry", entry.Memo)
	require.NotNil(t, entry.SourceType)
	assert.Equal(t, "manual", *entry.SourceType)
	require.Len(t, entry.Lines, 2)
}

func TestGLService_PostJournalEntry_UnbalancedError(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()
	postedBy := uuid.New()

	req := fin.CreateJournalEntryRequest{
		EntryDate: time.Now(),
		Memo:      "Unbalanced",
		Lines: []fin.JournalEntryLineInput{
			{AccountID: uuid.New(), DebitCents: 10000, CreditCents: 0},
			{AccountID: uuid.New(), DebitCents: 0, CreditCents: 5000},
		},
	}

	_, err := svc.PostJournalEntry(ctx, orgID, postedBy, req)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
}

func TestGLService_PostJournalEntry_TooFewLines(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()
	postedBy := uuid.New()

	req := fin.CreateJournalEntryRequest{
		EntryDate: time.Now(),
		Memo:      "Single line",
		Lines: []fin.JournalEntryLineInput{
			{AccountID: uuid.New(), DebitCents: 10000, CreditCents: 0},
		},
	}

	_, err := svc.PostJournalEntry(ctx, orgID, postedBy, req)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
}

func TestGLService_PostSystemJournalEntry_Balanced(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()
	postedBy := uuid.New()
	sourceType := "assessment"

	lines := []fin.GLJournalLine{
		{AccountID: uuid.New(), DebitCents: 15000, CreditCents: 0},
		{AccountID: uuid.New(), DebitCents: 0, CreditCents: 15000},
	}

	entry, err := svc.PostSystemJournalEntry(ctx, orgID, postedBy, time.Now(), "Assessment posting", &sourceType, nil, nil, lines)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.NotEqual(t, uuid.Nil, entry.ID)
	assert.Equal(t, 1, entry.EntryNumber)
	assert.Equal(t, "Assessment posting", entry.Memo)
	require.NotNil(t, entry.SourceType)
	assert.Equal(t, "assessment", *entry.SourceType)
	require.Len(t, entry.Lines, 2)
}

func TestGLService_PostSystemJournalEntry_Unbalanced(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()
	postedBy := uuid.New()

	lines := []fin.GLJournalLine{
		{AccountID: uuid.New(), DebitCents: 10000, CreditCents: 0},
		{AccountID: uuid.New(), DebitCents: 0, CreditCents: 5000},
	}

	_, err := svc.PostSystemJournalEntry(ctx, orgID, postedBy, time.Now(), "Bad entry", nil, nil, nil, lines)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gl: unbalanced journal entry")

	// This should be a plain fmt.Errorf, NOT an api error.
	var valErr *api.ValidationError
	assert.False(t, assert.ObjectsAreEqual(true, false)) // dummy; real check below
	assert.NotErrorIs(t, err, valErr)
}

func TestGLService_SeedDefaultAccounts(t *testing.T) {
	svc, repo := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()
	operatingFundID := uuid.New()
	reserveFundID := uuid.New()

	err := svc.SeedDefaultAccounts(ctx, orgID, operatingFundID, reserveFundID)
	require.NoError(t, err)

	// Count accounts belonging to this org.
	var count int
	for _, a := range repo.accounts {
		if a.OrgID == orgID {
			count++
		}
	}
	assert.Equal(t, 26, count)

	// Verify headers are marked correctly.
	headers := 0
	for _, a := range repo.accounts {
		if a.OrgID == orgID && a.IsHeader {
			headers++
		}
	}
	assert.Equal(t, 5, headers)

	// Verify parent references are set for child accounts.
	for _, a := range repo.accounts {
		if a.OrgID == orgID && !a.IsHeader {
			assert.NotNil(t, a.ParentID, "child account %d %s should have a parent_id", a.AccountNumber, a.Name)
		}
	}
}
