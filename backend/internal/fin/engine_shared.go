package fin

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/quorant/quorant/internal/platform/policy"
)

// Regex patterns for parsing vendor payment terms.
var (
	// discountTermsRE matches "2/10 Net 30" style terms.
	// Group 1: discount percent, Group 2: discount days, Group 3: net days.
	discountTermsRE = regexp.MustCompile(`(?i)(\d+)/(\d+)\s+net\s+(\d+)`)

	// netTermsRE matches "Net 30" style terms (no discount).
	// Group 1: net days.
	netTermsRE = regexp.MustCompile(`(?i)net\s+(\d+)`)
)

const defaultNetDays = 30

// parsePaymentTerms computes payment terms for a payable by parsing the vendor
// terms string. This is the shared implementation used by all engine drivers.
//
// Supported formats:
//   - "2/10 Net 30" -- 2% discount if paid within 10 days, due in 30 days
//   - "Net 30"      -- due in 30 days, no discount
//   - "Net 60"      -- due in 60 days, no discount
//   - empty/unknown -- defaults to Net 30
func parsePaymentTerms(pc PayableContext) (*PaymentTermsResult, error) {
	terms := pc.VendorTerms

	// Try discount terms first: "2/10 Net 30"
	if m := discountTermsRE.FindStringSubmatch(terms); len(m) == 4 {
		discountPct, _ := strconv.ParseFloat(m[1], 64)
		discountDays, _ := strconv.Atoi(m[2])
		netDays, _ := strconv.Atoi(m[3])

		dueDate := pc.InvoiceDate.AddDate(0, 0, netDays)
		discountDate := pc.InvoiceDate.AddDate(0, 0, discountDays)

		return &PaymentTermsResult{
			DueDate:         dueDate,
			DiscountDate:    &discountDate,
			DiscountPercent: &discountPct,
		}, nil
	}

	// Try simple net terms: "Net 30"
	if m := netTermsRE.FindStringSubmatch(terms); len(m) == 2 {
		netDays, _ := strconv.Atoi(m[1])
		dueDate := pc.InvoiceDate.AddDate(0, 0, netDays)

		return &PaymentTermsResult{
			DueDate: dueDate,
		}, nil
	}

	// Default: Net 30
	dueDate := pc.InvoiceDate.AddDate(0, 0, defaultNetDays)
	return &PaymentTermsResult{
		DueDate: dueDate,
	}, nil
}

// resolvePayableRecognitionDate determines when an expense should be recognized
// as a payable based on the recognition basis. This is the shared implementation
// used by all engine drivers.
//
//   - Cash basis: payables are not recognized; returns ErrCashBasisNoPayable.
//   - Accrual basis with ServiceDate: recognized on ServiceDate.
//   - Accrual basis without ServiceDate: recognized on InvoiceDate.
func resolvePayableRecognitionDate(basis RecognitionBasis, ec ExpenseContext) (time.Time, error) {
	if basis == RecognitionBasisCash {
		return time.Time{}, ErrCashBasisNoPayable
	}

	// Accrual: prefer service date, fall back to invoice date.
	if ec.ServiceDate != nil {
		return *ec.ServiceDate, nil
	}
	return ec.InvoiceDate, nil
}

// resolvePaymentStrategy determines how a payment should be applied to
// outstanding charges. This is the shared implementation used by all engine
// drivers.
//
// Resolution order:
//  1. Designated invoice -- if the payer specified an invoice, apply directly.
//  2. Policy engine -- if a registry is configured, resolve org-level payment
//     allocation rules via the two-tier policy pipeline.
//  3. Default -- oldest-first (FIFO by due date).
func resolvePaymentStrategy(ctx context.Context, registry *policy.Registry, pc PaymentContext) (*ApplicationStrategy, error) {
	// 1. Designated invoice takes precedence.
	if pc.DesignatedInvoice != nil {
		return &ApplicationStrategy{
			Method: ApplicationMethodDesignated,
		}, nil
	}

	// 2. Policy engine resolution.
	if registry != nil {
		res, err := registry.Resolve(ctx, pc.OrgID, nil, "payment_allocation_rules")
		if err != nil {
			return nil, fmt.Errorf("payment application strategy: resolve policy: %w", err)
		}
		if res != nil && res.Ruling != nil {
			var ruling AllocationRuling
			if err := res.Decode(&ruling); err != nil {
				return nil, fmt.Errorf("payment application strategy: decode ruling: %w", err)
			}
			return rulingToStrategy(&ruling), nil
		}
	}

	// 3. Default: oldest first.
	return &ApplicationStrategy{
		Method:         ApplicationMethodOldestFirst,
		WithinPriority: SortOldestFirst,
	}, nil
}

// rulingToStrategy converts an AllocationRuling from the policy engine into
// an ApplicationStrategy for the payment pipeline.
func rulingToStrategy(ruling *AllocationRuling) *ApplicationStrategy {
	strategy := &ApplicationStrategy{
		WithinPriority: SortOldestFirst,
	}

	if len(ruling.PriorityOrder) > 0 {
		strategy.Method = ApplicationMethodPriorityFIFO
		strategy.PriorityOrder = ruling.PriorityOrder
	} else {
		strategy.Method = ApplicationMethodOldestFirst
	}

	return strategy
}

// resolveRevenueRecognitionDate determines when revenue should be recognized for
// a given financial transaction. This is the shared implementation used by all
// engine drivers.
//
// Under both cash and accrual basis, HOA assessments, late fees, and interest
// are recognized on their EffectiveDate:
//
//   - Cash basis: revenue is recognized when cash is received.
//   - Accrual basis, assessment: revenue is recognized in the period billed.
//   - Accrual basis, late fee / interest: revenue is recognized when charged.
func resolveRevenueRecognitionDate(_ RecognitionBasis, tx FinancialTransaction) (time.Time, error) {
	return tx.EffectiveDate, nil
}
