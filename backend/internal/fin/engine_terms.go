package fin

import (
	"context"
	"time"
)

// PaymentTerms computes payment terms for a payable by parsing the vendor terms
// string. Delegates to the shared parsePaymentTerms implementation.
func (e *GaapEngine) PaymentTerms(_ context.Context, pc PayableContext) (*PaymentTermsResult, error) {
	return parsePaymentTerms(pc)
}

// PayableRecognitionDate determines when an expense should be recognized as a
// payable based on the recognition basis. Delegates to the shared
// resolvePayableRecognitionDate implementation.
func (e *GaapEngine) PayableRecognitionDate(_ context.Context, ec ExpenseContext) (time.Time, error) {
	return resolvePayableRecognitionDate(e.config.RecognitionBasis, ec)
}
