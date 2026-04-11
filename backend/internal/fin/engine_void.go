package fin

import (
	"context"
	"fmt"
)

// voidReversalEffects produces a reversal of a previously recorded transaction.
// It reconstructs the original effects and mirrors them: GL debits become
// credits (and vice versa), fund transaction amounts are negated, and ledger
// entries become adjustment entries with negated amounts.
//
// Expected Metadata keys:
//   - "original_type" string – the TransactionType of the original transaction
//
// The caller must populate FundAllocations and other fields from the original
// record so the engine can reconstruct the original effects.
func (e *GaapEngine) voidReversalEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	originalTypeStr, ok := tx.Metadata["original_type"].(string)
	if !ok || originalTypeStr == "" {
		return nil, fmt.Errorf("void_reversal: missing or empty original_type in metadata")
	}
	originalType := TransactionType(originalTypeStr)

	// Build a copy of the transaction with the original type to get the
	// original effects. This avoids infinite recursion because the copy has
	// the original type, not TxTypeVoidReversal.
	originalTx := tx
	originalTx.Type = originalType

	originalEffects, err := e.RecordTransaction(ctx, originalTx)
	if err != nil {
		return nil, fmt.Errorf("void_reversal: compute original effects for %q: %w", originalType, err)
	}

	effects := &FinancialEffects{
		IsReversal: true,
	}

	// Mirror GL lines: swap debits and credits.
	for _, line := range originalEffects.JournalLines {
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID:   line.AccountID,
			DebitCents:  line.CreditCents,
			CreditCents: line.DebitCents,
			Memo:        line.Memo,
		})
	}

	// Negate fund transaction amounts.
	for _, ftd := range originalEffects.FundTransactions {
		effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
			FundID:      ftd.FundID,
			Type:        ftd.Type,
			AmountCents: -ftd.AmountCents,
			Description: "Reversal: " + ftd.Description,
		})
	}

	// Convert ledger entries to adjustment type with amounts that reverse the
	// stored value. Payment-type entries are negated by executeEffects (stored
	// as -amount), so we produce +amount. Other types are stored as +amount,
	// so we produce -amount. Since the output uses LedgerEntryTypeAdjustment
	// (which executeEffects does not negate), we compute the final stored
	// value here.
	for _, led := range originalEffects.LedgerEntries {
		reversalAmount := -led.AmountCents
		if led.Type == LedgerEntryTypePayment {
			// Payment entries are stored as -amount by executeEffects.
			// To reverse, store +amount.
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
