package fin

import (
	"context"
	"fmt"
	"time"

	"github.com/quorant/quorant/internal/platform/policy"
)

// GaapEngine implements AccountingEngine for US GAAP fund accounting.
type GaapEngine struct {
	resolver AccountResolver
	registry *policy.Registry
	config   EngineConfig
}

// NewGaapEngine returns a new GAAP accounting engine.
func NewGaapEngine(resolver AccountResolver, registry *policy.Registry, config EngineConfig) *GaapEngine {
	return &GaapEngine{
		resolver: resolver,
		registry: registry,
		config:   config,
	}
}

// Compile-time interface check.
var _ AccountingEngine = (*GaapEngine)(nil)

// Standard returns the accounting standard this engine implements.
func (e *GaapEngine) Standard() AccountingStandard {
	return AccountingStandardGAAP
}

// ChartOfAccounts returns the full GAAP chart of accounts for HOA fund accounting.
func (e *GaapEngine) ChartOfAccounts() []GLAccountSeed {
	return gaapChartOfAccounts
}

// RecordTransaction records a financial transaction and returns the resulting effects.
func (e *GaapEngine) RecordTransaction(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	if e.config.RecognitionBasis == RecognitionBasisModifiedAccrual {
		return nil, fmt.Errorf("record transaction: modified_accrual basis not yet implemented (Phase 1 supports cash and accrual)")
	}
	switch tx.Type {
	case TxTypeAssessment:
		return e.assessmentEffects(ctx, tx)
	case TxTypePayment:
		return e.paymentEffects(ctx, tx)
	case TxTypeFundTransfer:
		return e.fundTransferEffects(ctx, tx)
	case TxTypeLateFee:
		return e.lateFeeEffects(ctx, tx)
	case TxTypeInterestAccrual:
		return e.interestAccrualEffects(ctx, tx)
	default:
		return nil, fmt.Errorf("record transaction: unsupported type %q", tx.Type)
	}
}

// fundKeyToRevenueAccount maps fund key to revenue account number.
var fundKeyToRevenueAccount = map[string]int{
	"operating": 4010,
	"reserve":   4020,
	"capital":   4030,
	"special":   4040,
}

func revenueAccountForFundKey(key string) int {
	if num, ok := fundKeyToRevenueAccount[key]; ok {
		return num
	}
	return 4010 // default to operating
}

// fundKeyToCashAccount maps fund key to cash account number.
var fundKeyToCashAccount = map[string]int{
	"operating": 1010,
	"reserve":   1020,
	"capital":   1030,
	"special":   1040,
}

func cashAccountForFundKey(key string) int {
	if num, ok := fundKeyToCashAccount[key]; ok {
		return num
	}
	return 1010 // default to operating
}

func (e *GaapEngine) assessmentEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	// Ledger entry always created regardless of basis.
	if tx.UnitID != nil {
		desc := tx.Memo
		if desc == "" {
			desc = "Assessment charge"
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID: *tx.UnitID, Type: LedgerEntryTypeCharge,
			AmountCents: tx.AmountCents, Description: desc, SourceID: tx.SourceID,
		})
	}

	// Cash basis: no GL, no fund transactions.
	if e.config.RecognitionBasis == RecognitionBasisCash {
		return effects, nil
	}

	// Accrual: DR AR, CR Revenue per fund.
	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("assessment: resolve AR account 1100: %w", err)
	}
	effects.JournalLines = append(effects.JournalLines, GLJournalLine{
		AccountID: arAccount.ID, DebitCents: tx.AmountCents,
	})

	if len(tx.FundAllocations) == 0 {
		// No fund allocations: default to operating revenue (4010) for full amount.
		revenueAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 4010)
		if err != nil {
			return nil, fmt.Errorf("assessment: resolve revenue account 4010: %w", err)
		}
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID: revenueAccount.ID, CreditCents: tx.AmountCents,
		})
	} else {
		for _, alloc := range tx.FundAllocations {
			revenueNum := revenueAccountForFundKey(alloc.FundKey)
			revenueAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, revenueNum)
			if err != nil {
				return nil, fmt.Errorf("assessment: resolve revenue account %d: %w", revenueNum, err)
			}
			effects.JournalLines = append(effects.JournalLines, GLJournalLine{
				AccountID: revenueAccount.ID, CreditCents: alloc.AmountCents,
			})
			effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
				FundID: alloc.FundID, Type: "assessment",
				AmountCents: alloc.AmountCents, Description: tx.Memo,
			})
		}
	}

	return effects, nil
}

func (e *GaapEngine) paymentEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	// Ledger entry always created regardless of basis.
	if tx.UnitID != nil {
		desc := tx.Memo
		if desc == "" {
			desc = "Payment received"
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID: *tx.UnitID, Type: LedgerEntryTypePayment,
			AmountCents: tx.AmountCents, Description: desc, SourceID: tx.SourceID,
		})
	}

	if e.config.RecognitionBasis == RecognitionBasisCash {
		// Cash basis: DR Cash, CR Revenue per fund (revenue recognized at receipt).
		if len(tx.FundAllocations) == 0 {
			// No fund allocations: default to operating cash (1010) and revenue (4010).
			cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1010)
			if err != nil {
				return nil, fmt.Errorf("payment: resolve cash account 1010: %w", err)
			}
			revenueAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 4010)
			if err != nil {
				return nil, fmt.Errorf("payment: resolve revenue account 4010: %w", err)
			}
			effects.JournalLines = append(effects.JournalLines,
				GLJournalLine{AccountID: cashAccount.ID, DebitCents: tx.AmountCents},
				GLJournalLine{AccountID: revenueAccount.ID, CreditCents: tx.AmountCents},
			)
		} else {
			for _, alloc := range tx.FundAllocations {
				cashNum := cashAccountForFundKey(alloc.FundKey)
				cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, cashNum)
				if err != nil {
					return nil, fmt.Errorf("payment: resolve cash account %d: %w", cashNum, err)
				}
				effects.JournalLines = append(effects.JournalLines, GLJournalLine{
					AccountID: cashAccount.ID, DebitCents: alloc.AmountCents,
				})

				revenueNum := revenueAccountForFundKey(alloc.FundKey)
				revenueAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, revenueNum)
				if err != nil {
					return nil, fmt.Errorf("payment: resolve revenue account %d: %w", revenueNum, err)
				}
				effects.JournalLines = append(effects.JournalLines, GLJournalLine{
					AccountID: revenueAccount.ID, CreditCents: alloc.AmountCents,
				})

				effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
					FundID: alloc.FundID, Type: "revenue",
					AmountCents: alloc.AmountCents, Description: tx.Memo,
				})
			}
		}
		return effects, nil
	}

	// Accrual: DR Cash (fund-dependent), CR AR.
	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("payment: resolve AR account 1100: %w", err)
	}

	if len(tx.FundAllocations) == 0 {
		// No fund allocations: default to operating cash (1010) for full amount.
		cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1010)
		if err != nil {
			return nil, fmt.Errorf("payment: resolve cash account 1010: %w", err)
		}
		effects.JournalLines = append(effects.JournalLines,
			GLJournalLine{AccountID: cashAccount.ID, DebitCents: tx.AmountCents},
			GLJournalLine{AccountID: arAccount.ID, CreditCents: tx.AmountCents},
		)
	} else {
		for _, alloc := range tx.FundAllocations {
			cashNum := cashAccountForFundKey(alloc.FundKey)
			cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, cashNum)
			if err != nil {
				return nil, fmt.Errorf("payment: resolve cash account %d: %w", cashNum, err)
			}
			effects.JournalLines = append(effects.JournalLines, GLJournalLine{
				AccountID: cashAccount.ID, DebitCents: alloc.AmountCents,
			})

			effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
				FundID: alloc.FundID, Type: "payment",
				AmountCents: alloc.AmountCents, Description: tx.Memo,
			})
		}

		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID: arAccount.ID, CreditCents: tx.AmountCents,
		})
	}

	// Overpayment: produce a credit directive for the surplus.
	if overpayment, ok := metadataInt64(tx.Metadata, "overpayment_cents"); ok && overpayment > 0 && tx.UnitID != nil {
		effects.Credits = append(effects.Credits, CreditDirective{
			UnitID:      *tx.UnitID,
			AmountCents: overpayment,
			Type:        CreditTypeOnAccount,
		})
	}

	return effects, nil
}

// metadataInt64 extracts an int64 from a metadata map, handling both int64
// (programmatic callers) and float64 (JSON-unmarshaled values).
func metadataInt64(m map[string]any, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

// fundTypeToCashAccount maps fund type strings to their cash account numbers.
var fundTypeToCashAccount = map[string]int{
	"operating": 1010,
	"reserve":   1020,
	"capital":   1030,
	"special":   1040,
}

func cashAccountForFundType(fundType string) int {
	if num, ok := fundTypeToCashAccount[fundType]; ok {
		return num
	}
	return 1010
}

func (e *GaapEngine) fundTransferEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	// Resolve source and destination cash accounts.
	// When FundAllocations have FundKey set, use fund-key-based mapping.
	// Otherwise, fall back to Metadata fund type strings set by the service layer.
	var srcCashNum, dstCashNum int
	if len(tx.FundAllocations) >= 2 && tx.FundAllocations[0].FundKey != "" {
		srcCashNum = cashAccountForFundKey(tx.FundAllocations[0].FundKey)
		dstCashNum = cashAccountForFundKey(tx.FundAllocations[1].FundKey)
	} else {
		srcType, _ := tx.Metadata["from_fund_type"].(string)
		dstType, _ := tx.Metadata["to_fund_type"].(string)
		srcCashNum = cashAccountForFundType(srcType)
		dstCashNum = cashAccountForFundType(dstType)
	}

	transferOutAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 3100)
	if err != nil {
		return nil, fmt.Errorf("fund_transfer: resolve interfund transfer out account 3100: %w", err)
	}
	srcCashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, srcCashNum)
	if err != nil {
		return nil, fmt.Errorf("fund_transfer: resolve source cash account %d: %w", srcCashNum, err)
	}
	dstCashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, dstCashNum)
	if err != nil {
		return nil, fmt.Errorf("fund_transfer: resolve dest cash account %d: %w", dstCashNum, err)
	}
	transferInAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 3110)
	if err != nil {
		return nil, fmt.Errorf("fund_transfer: resolve interfund transfer in account 3110: %w", err)
	}

	// GL: DR 3100 (Interfund Out), CR source Cash, DR dest Cash, CR 3110 (Interfund In).
	effects.JournalLines = append(effects.JournalLines,
		GLJournalLine{AccountID: transferOutAccount.ID, DebitCents: tx.AmountCents},
		GLJournalLine{AccountID: srcCashAccount.ID, CreditCents: tx.AmountCents},
		GLJournalLine{AccountID: dstCashAccount.ID, DebitCents: tx.AmountCents},
		GLJournalLine{AccountID: transferInAccount.ID, CreditCents: tx.AmountCents},
	)

	// Fund directives: only when fund allocations are provided (engine-level tests).
	// The service layer handles fund transactions independently.
	if len(tx.FundAllocations) >= 2 {
		effects.FundTransactions = append(effects.FundTransactions,
			FundTransactionDirective{
				FundID: tx.FundAllocations[0].FundID, Type: FundTxTypeTransferOut,
				AmountCents: tx.AmountCents, Description: tx.Memo,
			},
			FundTransactionDirective{
				FundID: tx.FundAllocations[1].FundID, Type: FundTxTypeTransferIn,
				AmountCents: tx.AmountCents, Description: tx.Memo,
			},
		)
	}

	// No ledger entries — fund transfers don't affect unit balances.
	return effects, nil
}

func (e *GaapEngine) lateFeeEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	// Ledger entry always created regardless of basis.
	if tx.UnitID != nil {
		desc := tx.Memo
		if desc == "" {
			desc = "Late fee"
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID: *tx.UnitID, Type: LedgerEntryTypeLateFee,
			AmountCents: tx.AmountCents, Description: desc, SourceID: tx.SourceID,
		})
	}

	// Cash basis: ledger only, no GL.
	if e.config.RecognitionBasis == RecognitionBasisCash {
		return effects, nil
	}

	// Accrual: DR AR, CR Late Fee Revenue per fund.
	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("late_fee: resolve AR account 1100: %w", err)
	}
	effects.JournalLines = append(effects.JournalLines, GLJournalLine{
		AccountID: arAccount.ID, DebitCents: tx.AmountCents,
	})

	lateFeeAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 4100)
	if err != nil {
		return nil, fmt.Errorf("late_fee: resolve late fee revenue account 4100: %w", err)
	}
	effects.JournalLines = append(effects.JournalLines, GLJournalLine{
		AccountID: lateFeeAccount.ID, CreditCents: tx.AmountCents,
	})

	for _, alloc := range tx.FundAllocations {
		effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
			FundID: alloc.FundID, Type: "late_fee",
			AmountCents: alloc.AmountCents, Description: tx.Memo,
		})
	}

	return effects, nil
}

func (e *GaapEngine) interestAccrualEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	// Ledger entry always created regardless of basis.
	if tx.UnitID != nil {
		desc := tx.Memo
		if desc == "" {
			desc = "Interest accrual"
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID: *tx.UnitID, Type: LedgerEntryTypeCharge,
			AmountCents: tx.AmountCents, Description: desc, SourceID: tx.SourceID,
		})
	}

	// Cash basis: ledger only, no GL.
	if e.config.RecognitionBasis == RecognitionBasisCash {
		return effects, nil
	}

	// Accrual: DR AR, CR Interest Income per fund.
	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("interest_accrual: resolve AR account 1100: %w", err)
	}
	effects.JournalLines = append(effects.JournalLines, GLJournalLine{
		AccountID: arAccount.ID, DebitCents: tx.AmountCents,
	})

	interestAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 4200)
	if err != nil {
		return nil, fmt.Errorf("interest_accrual: resolve interest income account 4200: %w", err)
	}
	effects.JournalLines = append(effects.JournalLines, GLJournalLine{
		AccountID: interestAccount.ID, CreditCents: tx.AmountCents,
	})

	for _, alloc := range tx.FundAllocations {
		effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
			FundID: alloc.FundID, Type: "interest",
			AmountCents: alloc.AmountCents, Description: tx.Memo,
		})
	}

	return effects, nil
}

// ValidateTransaction validates a financial transaction against GAAP rules.
func (e *GaapEngine) ValidateTransaction(_ context.Context, tx FinancialTransaction) error {
	if e.config.RecognitionBasis == RecognitionBasisModifiedAccrual {
		return fmt.Errorf("validate transaction: modified_accrual basis not yet implemented (Phase 1 supports cash and accrual)")
	}
	if tx.Type != TxTypeAdjustingEntry && tx.AmountCents <= 0 {
		return fmt.Errorf("validate: amount_cents must be positive, got %d", tx.AmountCents)
	}
	switch tx.Type {
	case TxTypeAssessment, TxTypeLateFee, TxTypeInterestAccrual:
		if tx.UnitID == nil {
			return fmt.Errorf("validate: %s requires unit_id", tx.Type)
		}
	case TxTypeFundTransfer, TxTypeInterfundLoan:
		if len(tx.FundAllocations) < 2 {
			return fmt.Errorf("validate: %s requires at least 2 fund_allocations (source and destination)", tx.Type)
		}
	case TxTypePayment:
		if tx.UnitID == nil {
			return fmt.Errorf("validate: payment requires unit_id")
		}
	}
	return nil
}

// PaymentTerms computes payment terms for a payable.
// Not yet implemented; returns ErrNotImplemented.
func (e *GaapEngine) PaymentTerms(_ context.Context, _ PayableContext) (*PaymentTermsResult, error) {
	return nil, ErrNotImplemented
}

// PayableRecognitionDate determines when an expense should be recognized as a payable.
// Not yet implemented; returns ErrNotImplemented.
func (e *GaapEngine) PayableRecognitionDate(_ context.Context, _ ExpenseContext) (time.Time, error) {
	return time.Time{}, ErrNotImplemented
}

// RevenueRecognitionDate determines when revenue should be recognized.
// Not yet implemented; returns ErrNotImplemented.
func (e *GaapEngine) RevenueRecognitionDate(_ context.Context, _ FinancialTransaction) (time.Time, error) {
	return time.Time{}, ErrNotImplemented
}

// gaapChartOfAccounts defines the standard GAAP chart of accounts for HOA fund accounting.
// 5 headers + 51 detail accounts = 56 total.
var gaapChartOfAccounts = []GLAccountSeed{
	// ── Headers ──────────────────────────────────────────────────────
	{Number: 1000, Name: "Assets", Type: "asset", IsHeader: true, IsSystem: true},
	{Number: 2000, Name: "Liabilities", Type: "liability", IsHeader: true, IsSystem: true},
	{Number: 3000, Name: "Fund Balances", Type: "equity", IsHeader: true, IsSystem: true},
	{Number: 4000, Name: "Revenue", Type: "revenue", IsHeader: true, IsSystem: true},
	{Number: 5000, Name: "Operating Expenses", Type: "expense", IsHeader: true, IsSystem: true},

	// ── Assets (13) ─────────────────────────────────────────────────
	{Number: 1010, ParentNum: 1000, Name: "Cash-Operating", Type: "asset", IsSystem: true, FundKey: "operating"},
	{Number: 1020, ParentNum: 1000, Name: "Cash-Reserve", Type: "asset", IsSystem: true, FundKey: "reserve"},
	{Number: 1030, ParentNum: 1000, Name: "Cash-Capital", Type: "asset", IsSystem: true, FundKey: "capital"},
	{Number: 1040, ParentNum: 1000, Name: "Cash-Special", Type: "asset", IsSystem: true, FundKey: "special"},
	{Number: 1100, ParentNum: 1000, Name: "AR-Assessments", Type: "asset", IsSystem: true},
	{Number: 1105, ParentNum: 1000, Name: "Allowance for Doubtful Accounts", Type: "asset", IsSystem: true},
	{Number: 1110, ParentNum: 1000, Name: "AR-Other", Type: "asset", IsSystem: true},
	{Number: 1150, ParentNum: 1000, Name: "Accrued Interest Receivable", Type: "asset", IsSystem: true},
	{Number: 1200, ParentNum: 1000, Name: "Prepaid Expenses", Type: "asset", IsSystem: true},
	{Number: 1300, ParentNum: 1000, Name: "Due From Other Funds", Type: "asset", IsSystem: true},
	{Number: 1400, ParentNum: 1000, Name: "Fixed Assets", Type: "asset", IsSystem: true},
	{Number: 1405, ParentNum: 1000, Name: "Accumulated Depreciation", Type: "asset", IsSystem: true},
	{Number: 1500, ParentNum: 1000, Name: "Insurance Claim Receivable", Type: "asset", IsSystem: true},

	// ── Liabilities (7) ─────────────────────────────────────────────
	{Number: 2100, ParentNum: 2000, Name: "AP", Type: "liability", IsSystem: true},
	{Number: 2110, ParentNum: 2000, Name: "Accrued Expenses", Type: "liability", IsSystem: true},
	{Number: 2200, ParentNum: 2000, Name: "Prepaid Assessments", Type: "liability", IsSystem: true},
	{Number: 2300, ParentNum: 2000, Name: "Owner Deposits", Type: "liability", IsSystem: true},
	{Number: 2400, ParentNum: 2000, Name: "Deferred Revenue-Other", Type: "liability", IsSystem: true},
	{Number: 2500, ParentNum: 2000, Name: "Due To Other Funds", Type: "liability", IsSystem: true},
	{Number: 2600, ParentNum: 2000, Name: "Income Tax Payable", Type: "liability", IsSystem: true},

	// ── Equity / Fund Balances (6) ──────────────────────────────────
	{Number: 3010, ParentNum: 3000, Name: "Operating Fund Balance", Type: "equity", IsSystem: true, FundKey: "operating"},
	{Number: 3020, ParentNum: 3000, Name: "Reserve Fund Balance", Type: "equity", IsSystem: true, FundKey: "reserve"},
	{Number: 3030, ParentNum: 3000, Name: "Capital Fund Balance", Type: "equity", IsSystem: true, FundKey: "capital"},
	{Number: 3040, ParentNum: 3000, Name: "Special Fund Balance", Type: "equity", IsSystem: true, FundKey: "special"},
	{Number: 3100, ParentNum: 3000, Name: "Interfund Transfer Out", Type: "equity", IsSystem: true},
	{Number: 3110, ParentNum: 3000, Name: "Interfund Transfer In", Type: "equity", IsSystem: true},

	// ── Revenue (11) ────────────────────────────────────────────────
	{Number: 4010, ParentNum: 4000, Name: "Assessment Revenue-Operating", Type: "revenue", IsSystem: true, FundKey: "operating"},
	{Number: 4020, ParentNum: 4000, Name: "Assessment Revenue-Reserve", Type: "revenue", IsSystem: true, FundKey: "reserve"},
	{Number: 4030, ParentNum: 4000, Name: "Assessment Revenue-Capital", Type: "revenue", IsSystem: true, FundKey: "capital"},
	{Number: 4040, ParentNum: 4000, Name: "Assessment Revenue-Special", Type: "revenue", IsSystem: true, FundKey: "special"},
	{Number: 4100, ParentNum: 4000, Name: "Late Fee Revenue", Type: "revenue", IsSystem: true},
	{Number: 4200, ParentNum: 4000, Name: "Interest Income", Type: "revenue", IsSystem: true},
	{Number: 4310, ParentNum: 4000, Name: "Facility Rental Income", Type: "revenue", IsSystem: true},
	{Number: 4320, ParentNum: 4000, Name: "Parking and Amenity Fees", Type: "revenue", IsSystem: true},
	{Number: 4330, ParentNum: 4000, Name: "Move-In/Move-Out Fees", Type: "revenue", IsSystem: true},
	{Number: 4400, ParentNum: 4000, Name: "Insurance Proceeds", Type: "revenue", IsSystem: true},
	{Number: 4900, ParentNum: 4000, Name: "Other Income", Type: "revenue", IsSystem: true},

	// ── Expenses (14) ───────────────────────────────────────────────
	{Number: 5010, ParentNum: 5000, Name: "Management Fee", Type: "expense", IsSystem: true},
	{Number: 5020, ParentNum: 5000, Name: "Insurance Premium", Type: "expense", IsSystem: true},
	{Number: 5030, ParentNum: 5000, Name: "Utilities", Type: "expense", IsSystem: true},
	{Number: 5040, ParentNum: 5000, Name: "Landscaping", Type: "expense", IsSystem: true},
	{Number: 5050, ParentNum: 5000, Name: "Maintenance and Repairs", Type: "expense", IsSystem: true},
	{Number: 5060, ParentNum: 5000, Name: "Professional Services", Type: "expense", IsSystem: true},
	{Number: 5070, ParentNum: 5000, Name: "Bad Debt Expense", Type: "expense", IsSystem: true},
	{Number: 5100, ParentNum: 5000, Name: "Administrative Expenses", Type: "expense", IsSystem: true},
	{Number: 5110, ParentNum: 5000, Name: "Payroll and Salaries", Type: "expense", IsSystem: true},
	{Number: 5120, ParentNum: 5000, Name: "Payroll Taxes and Benefits", Type: "expense", IsSystem: true},
	{Number: 5200, ParentNum: 5000, Name: "Reserve Expenses", Type: "expense", IsSystem: true, FundKey: "reserve"},
	{Number: 5210, ParentNum: 5000, Name: "Casualty Loss", Type: "expense", IsSystem: true},
	{Number: 5220, ParentNum: 5000, Name: "Depreciation Expense", Type: "expense", IsSystem: true},
	{Number: 5300, ParentNum: 5000, Name: "Insurance Deductible", Type: "expense", IsSystem: true},
}
