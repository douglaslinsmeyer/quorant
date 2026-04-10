package fin

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// OutstandingCharge represents a charge eligible for payment allocation.
type OutstandingCharge struct {
	ID          uuid.UUID
	ChargeType  ChargeType
	AmountCents int64
	DueDate     time.Time
}

// AllocationRuling is the typed ruling from the policy engine.
type AllocationRuling struct {
	PriorityOrder    []ChargeType `json:"priority_order"`
	FrozenChargeIDs  []uuid.UUID  `json:"frozen_charge_ids"`
	FrozenCutoffDate *time.Time   `json:"frozen_cutoff_date,omitempty"`
	AcceptPartial    bool         `json:"accept_partial"`
	CreditHandling   string       `json:"credit_handling,omitempty"`
	EstoppelOverride bool         `json:"estoppel_override,omitempty"`
	TrusteeOverride  bool         `json:"trustee_override,omitempty"`
}

// AllocationResult is one line of a payment allocation.
type AllocationResult struct {
	ChargeID       uuid.UUID
	ChargeType     ChargeType
	AllocatedCents int64
}

// Allocate distributes paymentCents across charges per the ruling.
// Returns allocation results and any remaining credit (overpayment).
func Allocate(charges []OutstandingCharge, paymentCents int64, ruling AllocationRuling) ([]AllocationResult, int64) {
	if len(charges) == 0 {
		return nil, paymentCents
	}

	// Build frozen set.
	frozen := make(map[uuid.UUID]bool, len(ruling.FrozenChargeIDs))
	for _, id := range ruling.FrozenChargeIDs {
		frozen[id] = true
	}

	// Filter out frozen charges.
	var eligible []OutstandingCharge
	for _, c := range charges {
		if frozen[c.ID] {
			continue
		}
		if ruling.FrozenCutoffDate != nil && c.DueDate.Before(*ruling.FrozenCutoffDate) {
			continue
		}
		eligible = append(eligible, c)
	}

	// Build priority index.
	priority := make(map[ChargeType]int, len(ruling.PriorityOrder))
	for i, ct := range ruling.PriorityOrder {
		priority[ct] = i
	}

	// Sort: primary by priority tier, secondary by due date (FIFO).
	sort.Slice(eligible, func(i, j int) bool {
		pi := priority[eligible[i].ChargeType]
		pj := priority[eligible[j].ChargeType]
		if pi != pj {
			return pi < pj
		}
		return eligible[i].DueDate.Before(eligible[j].DueDate)
	})

	// Walk and allocate.
	remaining := paymentCents
	var results []AllocationResult

	for _, charge := range eligible {
		if remaining <= 0 {
			break
		}
		alloc := charge.AmountCents
		if alloc > remaining {
			alloc = remaining
		}
		results = append(results, AllocationResult{
			ChargeID:       charge.ID,
			ChargeType:     charge.ChargeType,
			AllocatedCents: alloc,
		})
		remaining -= alloc
	}

	return results, remaining
}
