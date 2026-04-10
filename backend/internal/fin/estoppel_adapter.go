package fin

import (
	"context"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/estoppel"
)

// EstoppelFinancialAdapter wraps FinService to satisfy the
// estoppel.FinancialDataProvider interface.
type EstoppelFinancialAdapter struct {
	service *FinService
}

// NewEstoppelFinancialAdapter returns a new EstoppelFinancialAdapter backed by
// the provided FinService.
func NewEstoppelFinancialAdapter(service *FinService) *EstoppelFinancialAdapter {
	return &EstoppelFinancialAdapter{service: service}
}

// GetUnitFinancialSnapshot builds a FinancialSnapshot for the given unit by
// calling FinService methods. Fields that cannot be populated from the
// available service methods are left at their zero values.
func (a *EstoppelFinancialAdapter) GetUnitFinancialSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*estoppel.FinancialSnapshot, error) {
	snap := &estoppel.FinancialSnapshot{}

	// ── Assessment schedules ──────────────────────────────────────────────────
	// Populate regular assessment amount and frequency from the first active
	// schedule. If none exists the fields remain at zero values.
	schedules, err := a.service.ListSchedules(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, s := range schedules {
		if s.IsActive {
			snap.RegularAssessmentCents = s.BaseAmountCents
			snap.AssessmentFrequency = string(s.Frequency)
			break
		}
	}

	// ── Current balance ───────────────────────────────────────────────────────
	// GetUnitBalance is on the AssessmentRepository (not exposed on FinService
	// directly). We use GetUnitLedger to derive the current balance from the
	// most-recent entry's BalanceCents field.
	entries, _, err := a.service.GetUnitLedger(ctx, unitID, 1, nil)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		snap.CurrentBalanceCents = entries[0].BalanceCents
	}

	// ── Collection status ─────────────────────────────────────────────────────
	// GetUnitCollectionStatus returns a NotFoundError when there is no active
	// collection case; treat that as "no active collection".
	collCase, err := a.service.GetUnitCollectionStatus(ctx, unitID)
	if err == nil && collCase != nil {
		snap.HasActiveCollection = true
		snap.CollectionStatus = string(collCase.Status)
		snap.TotalDelinquentCents = collCase.CurrentOwedCents
	}
	// If err != nil (e.g. NotFoundError) we leave collection fields at zero.

	// ── Funds – association-level financials ─────────────────────────────────
	funds, err := a.service.ListFunds(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, f := range funds {
		switch f.FundType {
		case FundTypeReserve:
			snap.ReserveBalanceCents += f.BalanceCents
			if f.TargetBalanceCents != nil {
				snap.ReserveTargetCents += *f.TargetBalanceCents
			}
		}
	}

	// Fields not available from existing service methods and left as zero values:
	//   PaidThroughDate, NextInstallmentDueDate, SpecialAssessments,
	//   CapitalContributionCents, TransferFeeCents, ReserveFundFeeCents,
	//   PastDueItems, LateFeesCents, InterestCents, CollectionCostsCents,
	//   AttorneyName, AttorneyContact, LienStatus,
	//   HasPaymentPlan, PaymentPlanDetails,
	//   DelinquencyRate60Days, BudgetStatus,
	//   TotalUnits, OwnerOccupiedCount, RentalCount.

	return snap, nil
}
