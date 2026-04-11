package fin

import (
	"context"
	"fmt"
)

// badDebtProvisionEffects records an estimated provision for bad debt.
// GL: DR 5070 (Bad Debt Expense) / CR 1105 (Allowance for Doubtful Accounts).
// No ledger entry (provision is an estimate, not unit-specific).
// No fund transaction.
func (e *GaapEngine) badDebtProvisionEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
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
// Ledger: adjustment entry on the unit (reduces their balance).
// No fund transaction.
func (e *GaapEngine) badDebtWriteOffEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
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

	// Ledger: adjustment reduces the unit's balance.
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
// records the cash recovery in a single FinancialEffects bundle.
//
// GL step 1: DR 1100 (AR) / CR 1105 (Allowance) -- reinstate the receivable.
// GL step 2: DR 1010 (Cash-Operating) / CR 1100 (AR) -- record the payment.
// Total: 4 GL lines.
// Ledger: reinstatement entry + payment entry on the unit.
func (e *GaapEngine) badDebtRecoveryEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_recovery: resolve AR account 1100: %w", err)
	}
	allowanceAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1105)
	if err != nil {
		return nil, fmt.Errorf("bad_debt_recovery: resolve allowance account 1105: %w", err)
	}

	// Determine cash account: use fund allocation if present, default to 1010.
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

	// Ledger: reinstatement + payment on the unit.
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
