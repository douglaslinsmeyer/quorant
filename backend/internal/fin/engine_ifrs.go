package fin

import (
	"context"
	"fmt"
	"time"

	"github.com/quorant/quorant/internal/platform/policy"
)

// IfrsEngine implements AccountingEngine for International Financial Reporting
// Standards. IFRS uses a single "Retained Earnings" account rather than
// per-fund balance accounts, and does not support modified accrual basis.
type IfrsEngine struct {
	resolver AccountResolver
	registry *policy.Registry
	config   EngineConfig
}

// NewIfrsEngine returns a new IFRS accounting engine.
func NewIfrsEngine(resolver AccountResolver, registry *policy.Registry, config EngineConfig) *IfrsEngine {
	return &IfrsEngine{
		resolver: resolver,
		registry: registry,
		config:   config,
	}
}

// Compile-time interface check.
var _ AccountingEngine = (*IfrsEngine)(nil)

// Standard returns the accounting standard this engine implements.
func (e *IfrsEngine) Standard() AccountingStandard {
	return AccountingStandardIFRS
}

// ChartOfAccounts returns the IFRS chart of accounts for HOA accounting.
// Key difference from GAAP: equity uses a single "Retained Earnings" account
// (3010) instead of per-fund balance accounts (3010/3020/3030/3040).
// Fund-specific equity is a presentation choice under IFRS, not mandated.
func (e *IfrsEngine) ChartOfAccounts() []GLAccountSeed {
	return ifrsChartOfAccounts
}

// RecordTransaction records a financial transaction and returns the resulting
// effects under IFRS rules. Modified accrual is not supported.
func (e *IfrsEngine) RecordTransaction(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	if e.config.RecognitionBasis == RecognitionBasisModifiedAccrual {
		return nil, fmt.Errorf("record transaction: IFRS does not support modified_accrual basis")
	}
	switch tx.Type {
	case TxTypeAssessment:
		return e.assessmentEffects(ctx, tx)
	case TxTypePayment:
		return e.paymentEffects(ctx, tx)
	case TxTypeFundTransfer:
		return e.fundTransferEffects(ctx, tx)
	case TxTypeExpense:
		return e.expenseEffects(ctx, tx)
	case TxTypeLateFee:
		return e.lateFeeEffects(ctx, tx)
	case TxTypeInterestAccrual:
		return e.interestAccrualEffects(ctx, tx)
	case TxTypeBadDebtProvision:
		return e.badDebtProvisionEffects(ctx, tx)
	case TxTypeBadDebtWriteOff:
		return e.badDebtWriteOffEffects(ctx, tx)
	case TxTypeBadDebtRecovery:
		return e.badDebtRecoveryEffects(ctx, tx)
	case TxTypeYearEndClose:
		return e.yearEndCloseEffects(ctx, tx)
	case TxTypeVoidReversal:
		return e.voidReversalEffects(ctx, tx)
	default:
		return nil, fmt.Errorf("record transaction: unsupported type %q", tx.Type)
	}
}

// ValidateTransaction validates a financial transaction against IFRS rules.
// Modified accrual basis is rejected outright under IFRS.
func (e *IfrsEngine) ValidateTransaction(_ context.Context, tx FinancialTransaction) error {
	if e.config.RecognitionBasis == RecognitionBasisModifiedAccrual {
		return fmt.Errorf("validate transaction: IFRS does not support modified_accrual basis")
	}
	if tx.Type != TxTypeAdjustingEntry && tx.Type != TxTypeYearEndClose && tx.Type != TxTypeVoidReversal && tx.AmountCents <= 0 {
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

// PaymentTerms computes payment terms for a payable. Delegates to the shared
// parsePaymentTerms implementation.
func (e *IfrsEngine) PaymentTerms(_ context.Context, pc PayableContext) (*PaymentTermsResult, error) {
	return parsePaymentTerms(pc)
}

// PayableRecognitionDate determines when an expense should be recognized as a
// payable. Delegates to the shared resolvePayableRecognitionDate implementation.
func (e *IfrsEngine) PayableRecognitionDate(_ context.Context, ec ExpenseContext) (time.Time, error) {
	return resolvePayableRecognitionDate(e.config.RecognitionBasis, ec)
}

// PaymentApplicationStrategy determines how a payment should be applied to
// outstanding charges. Delegates to the shared resolvePaymentStrategy
// implementation.
func (e *IfrsEngine) PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error) {
	return resolvePaymentStrategy(ctx, e.registry, pc)
}

// RevenueRecognitionDate determines when revenue should be recognized. Delegates
// to the shared resolveRevenueRecognitionDate implementation.
func (e *IfrsEngine) RevenueRecognitionDate(_ context.Context, tx FinancialTransaction) (time.Time, error) {
	return resolveRevenueRecognitionDate(e.config.RecognitionBasis, tx)
}

// ── IFRS transaction effect methods ─────────────────────────────────────

func (e *IfrsEngine) assessmentEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

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

func (e *IfrsEngine) paymentEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

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
		if len(tx.FundAllocations) == 0 {
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

	if overpayment, ok := metadataInt64(tx.Metadata, "overpayment_cents"); ok && overpayment > 0 && tx.UnitID != nil {
		creditType := CreditTypeOnAccount
		if e.config.RecognitionBasis == RecognitionBasisAccrual {
			creditType = CreditTypePrepayment
		}
		effects.Credits = append(effects.Credits, CreditDirective{
			UnitID:      *tx.UnitID,
			AmountCents: overpayment,
			Type:        creditType,
		})
	}

	return effects, nil
}

func (e *IfrsEngine) fundTransferEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

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

	effects.JournalLines = append(effects.JournalLines,
		GLJournalLine{AccountID: transferOutAccount.ID, DebitCents: tx.AmountCents},
		GLJournalLine{AccountID: srcCashAccount.ID, CreditCents: tx.AmountCents},
		GLJournalLine{AccountID: dstCashAccount.ID, DebitCents: tx.AmountCents},
		GLJournalLine{AccountID: transferInAccount.ID, CreditCents: tx.AmountCents},
	)

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

	return effects, nil
}

func (e *IfrsEngine) lateFeeEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

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

	if e.config.RecognitionBasis == RecognitionBasisCash {
		return effects, nil
	}

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

func (e *IfrsEngine) interestAccrualEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

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

	if e.config.RecognitionBasis == RecognitionBasisCash {
		return effects, nil
	}

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

func (e *IfrsEngine) expenseEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	status, _ := tx.Metadata["status"].(string)

	if e.config.RecognitionBasis == RecognitionBasisCash && status == string(ExpenseStatusApproved) {
		return effects, nil
	}

	expenseAcctNum := 5010
	if num, ok := metadataInt(tx.Metadata, "expense_account"); ok {
		expenseAcctNum = num
	}

	switch {
	case e.config.RecognitionBasis == RecognitionBasisAccrual && status == string(ExpenseStatusApproved):
		expenseAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, expenseAcctNum)
		if err != nil {
			return nil, fmt.Errorf("expense: resolve expense account %d: %w", expenseAcctNum, err)
		}
		apAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 2100)
		if err != nil {
			return nil, fmt.Errorf("expense: resolve AP account 2100: %w", err)
		}
		effects.JournalLines = append(effects.JournalLines,
			GLJournalLine{AccountID: expenseAccount.ID, DebitCents: tx.AmountCents},
			GLJournalLine{AccountID: apAccount.ID, CreditCents: tx.AmountCents},
		)

	case e.config.RecognitionBasis == RecognitionBasisAccrual && status == string(ExpenseStatusPaid):
		apAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 2100)
		if err != nil {
			return nil, fmt.Errorf("expense: resolve AP account 2100: %w", err)
		}
		effects.JournalLines = append(effects.JournalLines,
			GLJournalLine{AccountID: apAccount.ID, DebitCents: tx.AmountCents},
		)
		if len(tx.FundAllocations) == 0 {
			cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1010)
			if err != nil {
				return nil, fmt.Errorf("expense: resolve cash account 1010: %w", err)
			}
			effects.JournalLines = append(effects.JournalLines,
				GLJournalLine{AccountID: cashAccount.ID, CreditCents: tx.AmountCents},
			)
		} else {
			for _, alloc := range tx.FundAllocations {
				cashNum := cashAccountForFundKey(alloc.FundKey)
				cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, cashNum)
				if err != nil {
					return nil, fmt.Errorf("expense: resolve cash account %d: %w", cashNum, err)
				}
				effects.JournalLines = append(effects.JournalLines,
					GLJournalLine{AccountID: cashAccount.ID, CreditCents: alloc.AmountCents},
				)
			}
		}

	case e.config.RecognitionBasis == RecognitionBasisCash && status == string(ExpenseStatusPaid):
		expenseAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, expenseAcctNum)
		if err != nil {
			return nil, fmt.Errorf("expense: resolve expense account %d: %w", expenseAcctNum, err)
		}
		effects.JournalLines = append(effects.JournalLines,
			GLJournalLine{AccountID: expenseAccount.ID, DebitCents: tx.AmountCents},
		)
		if len(tx.FundAllocations) == 0 {
			cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1010)
			if err != nil {
				return nil, fmt.Errorf("expense: resolve cash account 1010: %w", err)
			}
			effects.JournalLines = append(effects.JournalLines,
				GLJournalLine{AccountID: cashAccount.ID, CreditCents: tx.AmountCents},
			)
		} else {
			for _, alloc := range tx.FundAllocations {
				cashNum := cashAccountForFundKey(alloc.FundKey)
				cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, cashNum)
				if err != nil {
					return nil, fmt.Errorf("expense: resolve cash account %d: %w", cashNum, err)
				}
				effects.JournalLines = append(effects.JournalLines,
					GLJournalLine{AccountID: cashAccount.ID, CreditCents: alloc.AmountCents},
				)
			}
		}
	}

	for _, alloc := range tx.FundAllocations {
		effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
			FundID: alloc.FundID, Type: "expense",
			AmountCents: alloc.AmountCents, Description: tx.Memo,
		})
	}

	return effects, nil
}

// badDebtProvisionEffects records an estimated provision for bad debt under IFRS 9.
// GL: DR 5070 (Bad Debt Expense) / CR 1105 (Allowance for Doubtful Accounts).
func (e *IfrsEngine) badDebtProvisionEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	badDebtAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 5070)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_provision: resolve bad debt expense account 5070: %w", err)
	}
	allowanceAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1105)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_provision: resolve allowance account 1105: %w", err)
	}

	effects.JournalLines = append(effects.JournalLines,
		GLJournalLine{AccountID: badDebtAccount.ID, DebitCents: tx.AmountCents},
		GLJournalLine{AccountID: allowanceAccount.ID, CreditCents: tx.AmountCents},
	)

	return effects, nil
}

// badDebtWriteOffEffects writes off a specific receivable against the allowance.
// GL: DR 1105 (Allowance) / CR 1100 (AR).
func (e *IfrsEngine) badDebtWriteOffEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	allowanceAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1105)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_write_off: resolve allowance account 1105: %w", err)
	}
	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_write_off: resolve AR account 1100: %w", err)
	}

	effects.JournalLines = append(effects.JournalLines,
		GLJournalLine{AccountID: allowanceAccount.ID, DebitCents: tx.AmountCents},
		GLJournalLine{AccountID: arAccount.ID, CreditCents: tx.AmountCents},
	)

	if tx.UnitID != nil {
		desc := tx.Memo
		if desc == "" {
			desc = "Bad debt write-off"
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID:      *tx.UnitID,
			Type:        LedgerEntryTypeAdjustment,
			AmountCents: tx.AmountCents,
			Description: desc,
			SourceID:    tx.SourceID,
		})
	}

	return effects, nil
}

// badDebtRecoveryEffects reinstates a previously written-off receivable and
// records the cash recovery.
func (e *IfrsEngine) badDebtRecoveryEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_recovery: resolve AR account 1100: %w", err)
	}
	allowanceAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1105)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_recovery: resolve allowance account 1105: %w", err)
	}

	cashAcctNum := 1010
	if len(tx.FundAllocations) > 0 && tx.FundAllocations[0].FundKey != "" {
		cashAcctNum = cashAccountForFundKey(tx.FundAllocations[0].FundKey)
	}
	cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, cashAcctNum)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_recovery: resolve cash account %d: %w", cashAcctNum, err)
	}

	// Step 1: reinstate the receivable.
	effects.JournalLines = append(effects.JournalLines,
		GLJournalLine{AccountID: arAccount.ID, DebitCents: tx.AmountCents},
		GLJournalLine{AccountID: allowanceAccount.ID, CreditCents: tx.AmountCents},
	)

	// Step 2: record the cash receipt.
	effects.JournalLines = append(effects.JournalLines,
		GLJournalLine{AccountID: cashAccount.ID, DebitCents: tx.AmountCents},
		GLJournalLine{AccountID: arAccount.ID, CreditCents: tx.AmountCents},
	)

	if tx.UnitID != nil {
		reinstatementDesc := tx.Memo
		if reinstatementDesc == "" {
			reinstatementDesc = "Bad debt recovery - reinstatement"
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID:      *tx.UnitID,
			Type:        LedgerEntryTypeAdjustment,
			AmountCents: tx.AmountCents,
			Description: reinstatementDesc,
			SourceID:    tx.SourceID,
		})

		paymentDesc := "Bad debt recovery - payment"
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID:      *tx.UnitID,
			Type:        LedgerEntryTypePayment,
			AmountCents: tx.AmountCents,
			Description: paymentDesc,
			SourceID:    tx.SourceID,
		})
	}

	return effects, nil
}

// yearEndCloseEffects produces journal lines that zero out temporary accounts
// and roll the net difference into Retained Earnings (3010).
// Under IFRS, all funds close to the single Retained Earnings account.
func (e *IfrsEngine) yearEndCloseEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	// IFRS always uses Retained Earnings (3010) for year-end close,
	// regardless of which fund the metadata specifies.
	fundBalanceNum := 3010
	if num, ok := metadataInt(tx.Metadata, "fund_balance_account"); ok {
		// Allow override from metadata, but default to 3010.
		fundBalanceNum = num
	}

	balancesRaw, ok := tx.Metadata["account_balances"]
	if !ok {
		return nil, fmt.Errorf("year_end_close: missing account_balances in metadata")
	}
	balances, ok := balancesRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("year_end_close: account_balances must be a slice")
	}

	fundBalanceAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, fundBalanceNum)
	if err != nil {
		return nil, fmt.Errorf("year_end_close: resolve retained earnings account %d: %w", fundBalanceNum, err)
	}

	var netCents int64

	for _, raw := range balances {
		entry, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("year_end_close: each account_balance must be a map")
		}

		acctNum, ok := metadataInt(entry, "account_number")
		if !ok {
			return nil, fmt.Errorf("year_end_close: account_balance missing account_number")
		}
		balanceCents, ok := metadataInt64(entry, "balance_cents")
		if !ok {
			return nil, fmt.Errorf("year_end_close: account_balance missing balance_cents")
		}
		acctType, _ := entry["account_type"].(string)

		if balanceCents == 0 {
			continue
		}

		account, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, acctNum)
		if err != nil {
			return nil, fmt.Errorf("year_end_close: resolve account %d: %w", acctNum, err)
		}

		switch acctType {
		case "revenue":
			effects.JournalLines = append(effects.JournalLines, GLJournalLine{
				AccountID:  account.ID,
				DebitCents: balanceCents,
			})
			netCents += balanceCents

		case "expense":
			effects.JournalLines = append(effects.JournalLines, GLJournalLine{
				AccountID:   account.ID,
				CreditCents: balanceCents,
			})
			netCents -= balanceCents

		case "equity":
			if acctNum == 3100 {
				effects.JournalLines = append(effects.JournalLines, GLJournalLine{
					AccountID:   account.ID,
					CreditCents: balanceCents,
				})
				netCents -= balanceCents
			} else if acctNum == 3110 {
				effects.JournalLines = append(effects.JournalLines, GLJournalLine{
					AccountID:  account.ID,
					DebitCents: balanceCents,
				})
				netCents += balanceCents
			}
		}
	}

	if netCents > 0 {
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID:   fundBalanceAccount.ID,
			CreditCents: netCents,
		})
	} else if netCents < 0 {
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID:  fundBalanceAccount.ID,
			DebitCents: -netCents,
		})
	}

	return effects, nil
}

// voidReversalEffects produces a reversal of a previously recorded transaction.
func (e *IfrsEngine) voidReversalEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	originalTypeStr, ok := tx.Metadata["original_type"].(string)
	if !ok || originalTypeStr == "" {
		return nil, fmt.Errorf("void_reversal: missing or empty original_type in metadata")
	}
	originalType := TransactionType(originalTypeStr)

	originalTx := tx
	originalTx.Type = originalType

	originalEffects, err := e.RecordTransaction(ctx, originalTx)
	if err != nil {
		return nil, fmt.Errorf("void_reversal: compute original effects for %q: %w", originalType, err)
	}

	effects := &FinancialEffects{
		IsReversal: true,
	}

	for _, line := range originalEffects.JournalLines {
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID:   line.AccountID,
			DebitCents:  line.CreditCents,
			CreditCents: line.DebitCents,
			Memo:        line.Memo,
		})
	}

	for _, ftd := range originalEffects.FundTransactions {
		effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
			FundID:      ftd.FundID,
			Type:        ftd.Type,
			AmountCents: -ftd.AmountCents,
			Description: "Reversal: " + ftd.Description,
		})
	}

	for _, led := range originalEffects.LedgerEntries {
		reversalAmount := -led.AmountCents
		if led.Type == LedgerEntryTypePayment {
			reversalAmount = led.AmountCents
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID:      led.UnitID,
			Type:        LedgerEntryTypeAdjustment,
			AmountCents: reversalAmount,
			Description: "Reversal: " + led.Description,
			SourceID:    led.SourceID,
		})
	}

	return effects, nil
}

// ifrsChartOfAccounts defines the IFRS chart of accounts for HOA accounting.
// Assets, liabilities, revenue, and expenses use the same account numbers as
// GAAP for compatibility. The equity section is simplified per IFRS guidance.
// 5 headers + 48 detail accounts = 53 total.
var ifrsChartOfAccounts = []GLAccountSeed{
	// -- Headers -------------------------------------------------------
	{Number: 1000, Name: "Assets", Type: "asset", IsHeader: true, IsSystem: true},
	{Number: 2000, Name: "Liabilities", Type: "liability", IsHeader: true, IsSystem: true},
	{Number: 3000, Name: "Equity", Type: "equity", IsHeader: true, IsSystem: true},
	{Number: 4000, Name: "Revenue", Type: "revenue", IsHeader: true, IsSystem: true},
	{Number: 5000, Name: "Operating Expenses", Type: "expense", IsHeader: true, IsSystem: true},

	// -- Assets (13) ---------------------------------------------------
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

	// -- Liabilities (7) -----------------------------------------------
	{Number: 2100, ParentNum: 2000, Name: "AP", Type: "liability", IsSystem: true},
	{Number: 2110, ParentNum: 2000, Name: "Accrued Expenses", Type: "liability", IsSystem: true},
	{Number: 2200, ParentNum: 2000, Name: "Prepaid Assessments", Type: "liability", IsSystem: true},
	{Number: 2300, ParentNum: 2000, Name: "Owner Deposits", Type: "liability", IsSystem: true},
	{Number: 2400, ParentNum: 2000, Name: "Deferred Revenue-Other", Type: "liability", IsSystem: true},
	{Number: 2500, ParentNum: 2000, Name: "Due To Other Funds", Type: "liability", IsSystem: true},
	{Number: 2600, ParentNum: 2000, Name: "Income Tax Payable", Type: "liability", IsSystem: true},

	// -- Equity (3) -- IFRS simplified: single retained earnings -------
	{Number: 3010, ParentNum: 3000, Name: "Retained Earnings", Type: "equity", IsSystem: true},
	{Number: 3100, ParentNum: 3000, Name: "Interfund Transfer Out", Type: "equity", IsSystem: true},
	{Number: 3110, ParentNum: 3000, Name: "Interfund Transfer In", Type: "equity", IsSystem: true},

	// -- Revenue (11) --------------------------------------------------
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

	// -- Expenses (14) -------------------------------------------------
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
