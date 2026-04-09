package license

import (
	"context"

	"github.com/google/uuid"
)

// EntitlementChecker verifies whether an org has access to a feature.
// The real implementation resolves from plans → overrides → firm bundles.
type EntitlementChecker interface {
	Check(ctx context.Context, orgID uuid.UUID, featureKey string) (allowed bool, remaining int, err error)
}

// AllowAllChecker is a stub that allows all features. Used until
// the real license module is built in Phase 10.
type AllowAllChecker struct{}

func NewAllowAllChecker() *AllowAllChecker { return &AllowAllChecker{} }
func (c *AllowAllChecker) Check(ctx context.Context, orgID uuid.UUID, featureKey string) (bool, int, error) {
	return true, -1, nil // -1 = unlimited
}
