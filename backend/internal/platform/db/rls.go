package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SetRLSContext sets the PostgreSQL session variables used by Row-Level Security policies.
// Call this at the start of each transaction that accesses tenant data.
// SET LOCAL scopes the variable to the current transaction only, which is correct
// for connection-pooled environments where connections are reused across requests.
func SetRLSContext(ctx context.Context, tx pgx.Tx, userID, orgID uuid.UUID) error {
	_, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_user_id = '%s'", userID))
	if err != nil {
		return fmt.Errorf("setting RLS user_id: %w", err)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_org_id = '%s'", orgID))
	if err != nil {
		return fmt.Errorf("setting RLS org_id: %w", err)
	}
	return nil
}
