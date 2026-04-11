package fin

import (
	"context"
	"time"
)

// RevenueRecognitionDate determines when revenue should be recognized for a
// given financial transaction. Under both cash and accrual basis, HOA
// assessments, late fees, and interest are recognized on their EffectiveDate:
//
//   - Cash basis: revenue is recognized when cash is received.
//   - Accrual basis, assessment: revenue is recognized in the period billed.
//   - Accrual basis, late fee / interest: revenue is recognized when charged.
//
// Future phases may introduce deferred recognition (e.g., annual prepaid
// assessments amortized over 12 months), at which point this method will
// return dates based on the DeferralSchedule.
func (e *GaapEngine) RevenueRecognitionDate(_ context.Context, tx FinancialTransaction) (time.Time, error) {
	return tx.EffectiveDate, nil
}
