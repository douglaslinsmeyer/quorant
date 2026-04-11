package fin

import (
	"context"

	"github.com/google/uuid"
)

// AssessmentFundSplit determines how assessment revenue should be split across
// funds. Delegates to the shared resolveAssessmentFundSplit implementation.
func (e *GaapEngine) AssessmentFundSplit(ctx context.Context, orgID uuid.UUID) ([]fundSplitEntry, error) {
	return resolveAssessmentFundSplit(ctx, e.registry, orgID)
}

// AssessmentFundSplit determines how assessment revenue should be split across
// funds. Delegates to the shared resolveAssessmentFundSplit implementation.
func (e *IfrsEngine) AssessmentFundSplit(ctx context.Context, orgID uuid.UUID) ([]fundSplitEntry, error) {
	return resolveAssessmentFundSplit(ctx, e.registry, orgID)
}
