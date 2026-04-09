package ai

import (
	"context"

	"github.com/google/uuid"
)

// ComplianceCheckRepository persists compliance check audit records.
type ComplianceCheckRepository interface {
	Create(ctx context.Context, check *ComplianceCheck) (*ComplianceCheck, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]ComplianceCheck, bool, error)
	GetLatestByOrgAndRule(ctx context.Context, orgID, ruleID uuid.UUID) (*ComplianceCheck, error)
	Resolve(ctx context.Context, id uuid.UUID, notes string) (*ComplianceCheck, error)
}
