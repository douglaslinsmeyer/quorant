package fin

import (
	"context"
	"fmt"
)

// yearEndCloseEffects produces journal lines that zero out temporary accounts
// (revenue, expense, interfund transfer) and roll the net difference into the
// fund balance account. Account balances are supplied via Metadata because the
// engine does not query the database directly.
//
// Expected Metadata keys:
//   - "fund_key"               string   – e.g. "operating"
//   - "fund_balance_account"   float64  – account number for fund balance (e.g. 3010)
//   - "account_balances"       []any    – slice of maps with keys:
//     "account_number" (float64), "balance_cents" (float64), "account_type" (string)
func (e *GaapEngine) yearEndCloseEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	fundBalanceNum, ok := metadataInt(tx.Metadata, "fund_balance_account")
	if !ok {
		return nil, fmt.Errorf("year_end_close: missing fund_balance_account in metadata")
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
		return nil, fmt.Errorf("year_end_close: resolve fund balance account %d: %w", fundBalanceNum, err)
	}

	// Track the net amount that must go to fund balance.
	// Positive net = net income (CR fund balance); negative = net loss (DR fund balance).
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
			// Revenue accounts carry credit balances. DR to close (zero them out).
			effects.JournalLines = append(effects.JournalLines, GLJournalLine{
				AccountID:  account.ID,
				DebitCents: balanceCents,
			})
			netCents += balanceCents

		case "expense":
			// Expense accounts carry debit balances. CR to close (zero them out).
			effects.JournalLines = append(effects.JournalLines, GLJournalLine{
				AccountID:   account.ID,
				CreditCents: balanceCents,
			})
			netCents -= balanceCents

		case "equity":
			// Interfund transfer accounts: 3100 (transfer out) has a debit balance,
			// 3110 (transfer in) has a credit balance. Close both to zero.
			if acctNum == 3100 {
				// Transfer out has debit balance: CR to close.
				effects.JournalLines = append(effects.JournalLines, GLJournalLine{
					AccountID:   account.ID,
					CreditCents: balanceCents,
				})
				netCents -= balanceCents
			} else if acctNum == 3110 {
				// Transfer in has credit balance: DR to close.
				effects.JournalLines = append(effects.JournalLines, GLJournalLine{
					AccountID:  account.ID,
					DebitCents: balanceCents,
				})
				netCents += balanceCents
			}
		}
	}

	// Post the net difference to fund balance.
	if netCents > 0 {
		// Net income: CR fund balance.
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID:   fundBalanceAccount.ID,
			CreditCents: netCents,
		})
	} else if netCents < 0 {
		// Net loss: DR fund balance.
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID:  fundBalanceAccount.ID,
			DebitCents: -netCents,
		})
	}

	return effects, nil
}
