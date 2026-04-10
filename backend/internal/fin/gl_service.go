package fin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
)

// GLService provides business logic for the double-entry general ledger.
type GLService struct {
	gl      GLRepository
	auditor audit.Auditor
	logger  *slog.Logger
}

// NewGLService creates a new GLService.
func NewGLService(gl GLRepository, auditor audit.Auditor, logger *slog.Logger) *GLService {
	return &GLService{gl: gl, auditor: auditor, logger: logger}
}

// WithTx returns a copy of the GLService whose underlying repository runs
// against the given transaction.
func (s *GLService) WithTx(tx pgx.Tx) *GLService {
	return &GLService{
		gl:      s.gl.WithTx(tx),
		auditor: s.auditor,
		logger:  s.logger,
	}
}

// CreateAccount validates the request and persists a new GL account.
func (s *GLService) CreateAccount(ctx context.Context, orgID uuid.UUID, req CreateGLAccountRequest) (*GLAccount, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	a := &GLAccount{
		OrgID:         orgID,
		ParentID:      req.ParentID,
		FundID:        req.FundID,
		AccountNumber: req.AccountNumber,
		Name:          req.Name,
		AccountType:   GLAccountType(req.AccountType),
		IsHeader:      req.IsHeader,
		Description:   req.Description,
	}
	return s.gl.CreateAccount(ctx, a)
}

// GetAccount returns the account with the given id, or a 404 error if not found.
func (s *GLService) GetAccount(ctx context.Context, id uuid.UUID) (*GLAccount, error) {
	a, err := s.gl.FindAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "gl_account"), api.P("id", id.String()))
	}
	return a, nil
}

// ListAccounts returns all non-deleted GL accounts for the given org.
func (s *GLService) ListAccounts(ctx context.Context, orgID uuid.UUID) ([]GLAccount, error) {
	return s.gl.ListAccountsByOrg(ctx, orgID)
}

// UpdateAccount applies partial updates to an existing GL account.
func (s *GLService) UpdateAccount(ctx context.Context, id uuid.UUID, req UpdateGLAccountRequest) (*GLAccount, error) {
	existing, err := s.GetAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.IsSystem && req.Name != nil {
		return nil, api.NewUnprocessableError("gl.cannot_modify_system_account")
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.FundID != nil {
		existing.FundID = req.FundID
	}
	return s.gl.UpdateAccount(ctx, existing)
}

// DeleteAccount soft-deletes a GL account after verifying it is safe to do so.
func (s *GLService) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	existing, err := s.GetAccount(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return api.NewUnprocessableError("gl.cannot_delete_system_account")
	}
	hasLines, err := s.gl.HasPostedLines(ctx, id)
	if err != nil {
		return err
	}
	if hasLines {
		return api.NewUnprocessableError("gl.cannot_delete_account_with_entries")
	}
	return s.gl.SoftDeleteAccount(ctx, id)
}

// PostJournalEntry validates the request and posts a manual journal entry.
func (s *GLService) PostJournalEntry(ctx context.Context, orgID uuid.UUID, postedBy uuid.UUID, req CreateJournalEntryRequest) (*GLJournalEntry, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	lines := make([]GLJournalLine, len(req.Lines))
	for i, l := range req.Lines {
		lines[i] = GLJournalLine{
			AccountID:   l.AccountID,
			DebitCents:  l.DebitCents,
			CreditCents: l.CreditCents,
			Memo:        l.Memo,
		}
	}

	sourceType := GLSourceTypeManual
	entry := &GLJournalEntry{
		OrgID:      orgID,
		EntryDate:  req.EntryDate,
		Memo:       req.Memo,
		SourceType: &sourceType,
		PostedBy:   postedBy,
		Lines:      lines,
	}
	return s.gl.PostJournalEntry(ctx, entry)
}

// PostSystemJournalEntry is the internal method called by FinService for
// automated postings. It enforces the balance invariant and minimum line count.
func (s *GLService) PostSystemJournalEntry(
	ctx context.Context,
	orgID uuid.UUID,
	postedBy uuid.UUID,
	entryDate time.Time,
	memo string,
	sourceType *GLSourceType,
	sourceID *uuid.UUID,
	unitID *uuid.UUID,
	lines []GLJournalLine,
) (*GLJournalEntry, error) {
	if len(lines) < 2 {
		return nil, fmt.Errorf("gl: journal entry requires at least 2 lines")
	}

	var totalDebits, totalCredits int64
	for _, line := range lines {
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	if totalDebits != totalCredits {
		return nil, fmt.Errorf("gl: unbalanced journal entry: debits=%d credits=%d", totalDebits, totalCredits)
	}

	entry := &GLJournalEntry{
		OrgID:      orgID,
		EntryDate:  entryDate,
		Memo:       memo,
		SourceType: sourceType,
		SourceID:   sourceID,
		UnitID:     unitID,
		PostedBy:   postedBy,
		Lines:      lines,
	}
	return s.gl.PostJournalEntry(ctx, entry)
}

// FindAccountByOrgAndNumber returns the account with the given org and number.
func (s *GLService) FindAccountByOrgAndNumber(ctx context.Context, orgID uuid.UUID, number int) (*GLAccount, error) {
	return s.gl.FindAccountByOrgAndNumber(ctx, orgID, number)
}

// GetJournalEntry returns the journal entry with the given id, or a 404 error.
func (s *GLService) GetJournalEntry(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error) {
	entry, err := s.gl.FindJournalEntryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "journal_entry"), api.P("id", id.String()))
	}
	return entry, nil
}

// ListJournalEntries returns all journal entries for the given org.
func (s *GLService) ListJournalEntries(ctx context.Context, orgID uuid.UUID) ([]GLJournalEntry, error) {
	return s.gl.ListJournalEntriesByOrg(ctx, orgID)
}

// GetTrialBalance returns the trial balance for the given org as of the given date.
func (s *GLService) GetTrialBalance(ctx context.Context, orgID uuid.UUID, asOfDate time.Time) ([]TrialBalanceRow, error) {
	return s.gl.GetTrialBalance(ctx, orgID, asOfDate)
}

// GetAccountBalances returns account balances for the given org in the date range.
func (s *GLService) GetAccountBalances(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]AccountBalance, error) {
	return s.gl.GetAccountBalances(ctx, orgID, from, to)
}

// SeedDefaultAccounts creates the default chart of accounts for a new org.
func (s *GLService) SeedDefaultAccounts(ctx context.Context, orgID uuid.UUID, operatingFundID uuid.UUID, reserveFundID uuid.UUID, engine AccountingEngine) error {
	fundMap := map[string]*uuid.UUID{
		"operating": &operatingFundID,
		"reserve":   &reserveFundID,
	}

	seeds := engine.ChartOfAccounts()
	numToID := make(map[int]uuid.UUID)

	for _, seed := range seeds {
		a := &GLAccount{
			OrgID:         orgID,
			AccountNumber: seed.Number,
			Name:          seed.Name,
			AccountType:   seed.Type,
			IsHeader:      seed.IsHeader,
			IsSystem:      seed.IsSystem,
			FundID:        fundMap[seed.FundKey],
		}
		if seed.ParentNum != 0 {
			parentID := numToID[seed.ParentNum]
			a.ParentID = &parentID
		}
		created, err := s.gl.CreateAccount(ctx, a)
		if err != nil {
			return fmt.Errorf("seed account %d %s: %w", seed.Number, seed.Name, err)
		}
		numToID[seed.Number] = created.ID
	}

	return nil
}
