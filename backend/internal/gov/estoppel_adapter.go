package gov

import (
	"context"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/estoppel"
)

// EstoppelComplianceAdapter wraps GovService to satisfy the
// estoppel.ComplianceDataProvider interface.
type EstoppelComplianceAdapter struct {
	service *GovService
}

// NewEstoppelComplianceAdapter returns a new EstoppelComplianceAdapter backed
// by the provided GovService.
func NewEstoppelComplianceAdapter(service *GovService) *EstoppelComplianceAdapter {
	return &EstoppelComplianceAdapter{service: service}
}

// GetUnitComplianceSnapshot builds a ComplianceSnapshot for the given unit by
// fetching all violations for the org and filtering to those belonging to the
// unit. Fields not available from existing service methods are left at their
// zero values.
func (a *EstoppelComplianceAdapter) GetUnitComplianceSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*estoppel.ComplianceSnapshot, error) {
	snap := &estoppel.ComplianceSnapshot{}

	// ── Violations ────────────────────────────────────────────────────────────
	// ListViolations paginates by org; we fetch up to 1000 at a time and filter
	// by unitID client-side. GovService exposes no unit-scoped listing method.
	const pageSize = 1000
	var afterID *uuid.UUID
	var summary estoppel.ViolationSummary
	categorySet := make(map[string]struct{})

	for {
		violations, hasMore, err := a.service.ListViolations(ctx, orgID, pageSize, afterID)
		if err != nil {
			return nil, err
		}

		for _, v := range violations {
			if v.UnitID != unitID {
				continue
			}
			if v.ResolvedAt != nil {
				summary.ResolvedCount++
				continue
			}
			// Non-resolved violations are considered open.
			summary.OpenCount++
			categorySet[v.Category] = struct{}{}
			if v.FineTotalCents > 0 {
				summary.HasActiveFine = true
				summary.TotalFinesCents += v.FineTotalCents
			}
		}

		if !hasMore || len(violations) == 0 {
			break
		}
		last := violations[len(violations)-1]
		afterID = &last.ID
	}

	for cat := range categorySet {
		summary.Categories = append(summary.Categories, cat)
	}

	// HasHearingPending: requires joining to HearingLink — not available via
	// GovService without a dedicated query. Left as false.

	snap.Violations = summary

	// Fields not available from existing service methods and left at zero values:
	//   Litigation (HasActiveLitigation, CaseDescription, InvolvesUnit, InvolvesAssociation),
	//   Insurance (HasMasterPolicy, PolicyNumber, Carrier, ExpiresAt, CoverageAmountCents,
	//              FloodCoverage, EarthquakeCoverage),
	//   CCRVersion, ByLawVersion, RulesVersion, PendingAmendments.

	return snap, nil
}
