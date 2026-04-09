package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditEntry represents a single audit log record.
type AuditEntry struct {
	OrgID          uuid.UUID         `json:"org_id"`
	ActorID        uuid.UUID         `json:"actor_id"`
	ImpersonatorID *uuid.UUID        `json:"impersonator_id,omitempty"`
	Action         string            `json:"action"`        // e.g., "assessment.created"
	ResourceType   string            `json:"resource_type"` // e.g., "assessment"
	ResourceID     uuid.UUID         `json:"resource_id"`
	Module         string            `json:"module"` // e.g., "fin"
	BeforeState    json.RawMessage   `json:"before_state,omitempty"`
	AfterState     json.RawMessage   `json:"after_state,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	OccurredAt     time.Time         `json:"occurred_at"`
}

// Auditor records audit log entries.
type Auditor interface {
	Record(ctx context.Context, entry AuditEntry) error
}

// NoopAuditor is a stub that discards all audit entries. Used until
// the real audit module is built in Phase 4.
type NoopAuditor struct{}

func NewNoopAuditor() *NoopAuditor { return &NoopAuditor{} }
func (a *NoopAuditor) Record(ctx context.Context, entry AuditEntry) error { return nil }
