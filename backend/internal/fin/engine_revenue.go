package fin

import (
	"context"
	"time"
)

// RevenueRecognitionDate determines when revenue should be recognized for a
// given financial transaction. Delegates to the shared
// resolveRevenueRecognitionDate implementation.
func (e *GaapEngine) RevenueRecognitionDate(_ context.Context, tx FinancialTransaction) (time.Time, error) {
	return resolveRevenueRecognitionDate(e.config.RecognitionBasis, tx)
}
