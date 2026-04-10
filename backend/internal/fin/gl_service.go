package fin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
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
		AccountType:   req.AccountType,
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
		return nil, api.NewNotFoundError(fmt.Sprintf("gl account %s not found", id))
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
		return nil, api.NewUnprocessableError("cannot modify system account")
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
		return api.NewUnprocessableError("cannot delete system account")
	}
	hasLines, err := s.gl.HasPostedLines(ctx, id)
	if err != nil {
		return err
	}
	if hasLines {
		return api.NewUnprocessableError("cannot delete account with posted journal lines")
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

	sourceType := "manual"
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
	sourceType *string,
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
		return nil, api.NewNotFoundError(fmt.Sprintf("journal entry %s not found", id))
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
func (s *GLService) SeedDefaultAccounts(ctx context.Context, orgID uuid.UUID, operatingFundID uuid.UUID, reserveFundID uuid.UUID) error {
	type acctDef struct {
		number      int
		name        string
		acctType    string
		isHeader    bool
		isSystem    bool
		fundID      *uuid.UUID
		parentNum   int // 0 means no parent
	}

	defs := []acctDef{
		// Headers
		{1000, "Assets", "asset", true, true, nil, 0},
		{2000, "Liabilities", "liability", true, true, nil, 0},
		{3000, "Fund Balances", "equity", true, true, nil, 0},
		{4000, "Revenue", "revenue", true, true, nil, 0},
		{5000, "Operating Expenses", "expense", true, true, nil, 0},

		// Under 1000 Assets
		{1010, "Cash-Operating", "asset", false, true, &operatingFundID, 1000},
		{1020, "Cash-Reserve", "asset", false, true, &reserveFundID, 1000},
		{1100, "AR-Assessments", "asset", false, true, &operatingFundID, 1000},
		{1110, "AR-Other", "asset", false, false, &operatingFundID, 1000},
		{1200, "Prepaid Expenses", "asset", false, false, &operatingFundID, 1000},

		// Under 2000 Liabilities
		{2100, "AP", "liability", false, true, &operatingFundID, 2000},
		{2200, "Prepaid Assessments", "liability", false, false, &operatingFundID, 2000},

		// Under 3000 Fund Balances
		{3010, "Operating Fund Balance", "equity", false, true, &operatingFundID, 3000},
		{3020, "Reserve Fund Balance", "equity", false, true, &reserveFundID, 3000},
		{3100, "Interfund Transfer Out", "equity", false, true, nil, 3000},
		{3110, "Interfund Transfer In", "equity", false, true, nil, 3000},

		// Under 4000 Revenue
		{4010, "Assessment Revenue-Operating", "revenue", false, true, &operatingFundID, 4000},
		{4020, "Assessment Revenue-Reserve", "revenue", false, true, &reserveFundID, 4000},
		{4100, "Late Fee Revenue", "revenue", false, true, &operatingFundID, 4000},
		{4200, "Interest Income", "revenue", false, false, nil, 4000},

		// Under 5000 Operating Expenses
		{5010, "Management Fee", "expense", false, false, &operatingFundID, 5000},
		{5020, "Insurance", "expense", false, false, &operatingFundID, 5000},
		{5030, "Utilities", "expense", false, false, &operatingFundID, 5000},
		{5040, "Landscaping", "expense", false, false, &operatingFundID, 5000},
		{5050, "Maintenance and Repairs", "expense", false, false, &operatingFundID, 5000},
		{5060, "Professional Services", "expense", false, false, &operatingFundID, 5000},
	}

	// Map account number to created account ID for parent references.
	numToID := make(map[int]uuid.UUID)

	for _, d := range defs {
		a := &GLAccount{
			OrgID:         orgID,
			AccountNumber: d.number,
			Name:          d.name,
			AccountType:   d.acctType,
			IsHeader:      d.isHeader,
			IsSystem:      d.isSystem,
			FundID:        d.fundID,
		}
		if d.parentNum != 0 {
			parentID := numToID[d.parentNum]
			a.ParentID = &parentID
		}
		created, err := s.gl.CreateAccount(ctx, a)
		if err != nil {
			return fmt.Errorf("seed account %d %s: %w", d.number, d.name, err)
		}
		numToID[d.number] = created.ID
	}

	return nil
}
