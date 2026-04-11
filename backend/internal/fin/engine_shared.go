package fin

import (
	"context"
	"encoding/json"
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

// validatePeriodBoundary checks whether the transaction's EffectiveDate falls
// in an open (or soft-closed for adjusting entries) accounting period. When
// periods is nil, all transactions are allowed (backward compatible).
func validatePeriodBoundary(ctx context.Context, periods AccountingPeriodRepository, tx FinancialTransaction) error {
	if periods == nil {
		return nil
	}

	period, err := periods.GetPeriodForDate(ctx, tx.OrgID, tx.EffectiveDate)
	if err != nil {
		// Period not found — allow (periods may not be set up yet).
		return nil
	}

	switch period.Status {
	case PeriodStatusClosed:
		return ErrClosedPeriod
	case PeriodStatusSoftClosed:
		if tx.Type != TxTypeAdjustingEntry {
			return ErrSoftClosedPeriod
		}
	}

	return nil
}

// feeCapRuling holds the decoded late_fee_cap policy ruling.
type feeCapRuling struct {
	MaxCents *int64 `json:"max_cents"`
}

// interestCapRuling holds the decoded interest_rate_cap policy ruling.
type interestCapRuling struct {
	MaxCents *int64 `json:"max_cents"`
}

// OverpaymentAction defines how the org wants to handle overpayments.
type OverpaymentAction string

const (
	OverpaymentActionAccept OverpaymentAction = "accept"
	OverpaymentActionReject OverpaymentAction = "reject"
	OverpaymentActionCap    OverpaymentAction = "cap"
)

// overpaymentRuling holds the decoded overpayment_policy ruling.
type overpaymentRuling struct {
	Action            OverpaymentAction `json:"action"`
	MaxOverpayPercent *float64          `json:"max_overpay_percent,omitempty"`
}

// validateOverpayment checks whether a payment's overpayment is permitted by
// the org's overpayment policy. When the registry is nil or no policy is
// configured, overpayments are accepted (GAAP default: prepaid assessment).
func validateOverpayment(ctx context.Context, registry *policy.Registry, tx FinancialTransaction) error {
	if registry == nil {
		return nil
	}

	overpaymentCents, ok := metadataInt64(tx.Metadata, "overpayment_cents")
	if !ok || overpaymentCents <= 0 {
		return nil // no overpayment, nothing to validate
	}

	resolution, err := registry.Resolve(ctx, tx.OrgID, nil, "overpayment_policy")
	if err != nil || resolution == nil || resolution.Ruling == nil {
		return nil // no policy configured — accept (GAAP default)
	}

	var ruling overpaymentRuling
	if err := json.Unmarshal(resolution.Ruling, &ruling); err != nil {
		return nil
	}

	balanceCents, _ := metadataInt64(tx.Metadata, "unit_balance_cents")

	switch ruling.Action {
	case OverpaymentActionReject:
		return fmt.Errorf("validate: payment overpayment of %d cents exceeds outstanding balance of %d cents", overpaymentCents, balanceCents)
	case OverpaymentActionCap:
		if ruling.MaxOverpayPercent != nil && balanceCents > 0 {
			maxOverpay := int64(float64(balanceCents) * *ruling.MaxOverpayPercent)
			if overpaymentCents > maxOverpay {
				return fmt.Errorf("validate: payment overpayment of %d cents exceeds cap of %.0f%% (max %d cents) on balance of %d cents",
					overpaymentCents, *ruling.MaxOverpayPercent*100, maxOverpay, balanceCents)
			}
		}
		return nil
	default:
		return nil // accept
	}
}

// reserveWithdrawalRuling holds the decoded reserve_withdrawal_policy ruling.
type reserveWithdrawalRuling struct {
	RequiresApproval bool `json:"requires_approval"`
}

// validateReserveWithdrawal checks whether a fund transfer from a reserve fund
// requires board approval. When the registry is nil or no policy is configured,
// reserve withdrawals are allowed without approval.
func validateReserveWithdrawal(ctx context.Context, registry *policy.Registry, tx FinancialTransaction) error {
	if registry == nil {
		return nil
	}

	// Only applies to transfers FROM reserve funds.
	fromType, _ := tx.Metadata["from_fund_type"].(string)
	if fromType != string(FundTypeReserve) {
		return nil
	}

	resolution, err := registry.Resolve(ctx, tx.OrgID, nil, "reserve_withdrawal_policy")
	if err != nil || resolution == nil || resolution.Ruling == nil {
		return nil // no policy configured — allow
	}

	var ruling reserveWithdrawalRuling
	if err := json.Unmarshal(resolution.Ruling, &ruling); err != nil {
		return nil
	}

	if !ruling.RequiresApproval {
		return nil
	}

	// Check if approval was provided.
	if approvedBy, ok := tx.Metadata["approved_by"].(string); ok && approvedBy != "" {
		return nil
	}

	return fmt.Errorf("validate: reserve fund withdrawal requires board approval")
}

// validateFeeCap checks whether a late fee exceeds the jurisdiction-scoped cap.
// When the registry is nil or no cap policy is configured, all fees are allowed.
func validateFeeCap(ctx context.Context, registry *policy.Registry, tx FinancialTransaction) error {
	if registry == nil {
		return nil
	}

	resolution, err := registry.Resolve(ctx, tx.OrgID, nil, "late_fee_cap")
	if err != nil || resolution == nil || resolution.Ruling == nil {
		return nil // no cap configured
	}

	var cap feeCapRuling
	if err := json.Unmarshal(resolution.Ruling, &cap); err != nil {
		return nil
	}

	if cap.MaxCents != nil && tx.AmountCents > *cap.MaxCents {
		return fmt.Errorf("validate: late fee %d cents exceeds jurisdiction cap of %d cents", tx.AmountCents, *cap.MaxCents)
	}

	return nil
}

// validateInterestCap checks whether an interest accrual exceeds the
// jurisdiction-scoped cap. When the registry is nil or no cap policy is
// configured, all interest charges are allowed.
func validateInterestCap(ctx context.Context, registry *policy.Registry, tx FinancialTransaction) error {
	if registry == nil {
		return nil
	}

	resolution, err := registry.Resolve(ctx, tx.OrgID, nil, "interest_rate_cap")
	if err != nil || resolution == nil || resolution.Ruling == nil {
		return nil // no cap configured
	}

	var cap interestCapRuling
	if err := json.Unmarshal(resolution.Ruling, &cap); err != nil {
		return nil
	}

	if cap.MaxCents != nil && tx.AmountCents > *cap.MaxCents {
		return fmt.Errorf("validate: interest accrual %d cents exceeds jurisdiction cap of %d cents", tx.AmountCents, *cap.MaxCents)
	}

	return nil
}
