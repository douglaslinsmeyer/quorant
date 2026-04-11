package fin

import (
	"context"
	"fmt"
)

// PaymentApplicationStrategy determines how a payment should be applied to
// outstanding charges. Resolution order:
//  1. Designated invoice -- if the payer specified an invoice, apply directly.
//  2. Policy engine -- if a registry is configured, resolve org-level payment
//     allocation rules via the two-tier policy pipeline.
//  3. Default -- oldest-first (FIFO by due date).
func (e *GaapEngine) PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error) {
	// 1. Designated invoice takes precedence.
	if pc.DesignatedInvoice != nil {
		return &ApplicationStrategy{
			Method: ApplicationMethodDesignated,
		}, nil
	}

	// 2. Policy engine resolution.
	if e.registry != nil {
		res, err := e.registry.Resolve(ctx, pc.OrgID, nil, "payment_allocation_rules")
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
