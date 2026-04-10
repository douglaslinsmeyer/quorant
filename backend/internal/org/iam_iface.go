package org

import (
	"context"

	"github.com/quorant/quorant/internal/iam"
)

// UserFinder is the subset of iam.UserRepository needed by the org module.
// Defined here to follow the interface segregation principle.
// iam.PostgresUserRepository satisfies this interface via Go structural typing.
type UserFinder interface {
	FindByIDPUserID(ctx context.Context, idpUserID string) (*iam.User, error)
}
