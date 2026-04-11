package fin

import (
	"context"
)

// PaymentApplicationStrategy determines how a payment should be applied to
// outstanding charges. Delegates to the shared resolvePaymentStrategy
// implementation.
func (e *GaapEngine) PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error) {
	return resolvePaymentStrategy(ctx, e.registry, pc)
}
