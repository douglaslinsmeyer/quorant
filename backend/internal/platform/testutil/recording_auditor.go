package testutil

import (
	"context"
	"sync"

	"github.com/quorant/quorant/internal/audit"
)

// RecordingAuditor captures audit entries in memory for test assertions.
type RecordingAuditor struct {
	mu      sync.Mutex
	entries []audit.AuditEntry
}

// NewRecordingAuditor creates a new RecordingAuditor with an empty entry slice.
func NewRecordingAuditor() *RecordingAuditor {
	return &RecordingAuditor{entries: make([]audit.AuditEntry, 0)}
}

// Record appends the entry to the internal slice.
func (a *RecordingAuditor) Record(_ context.Context, entry audit.AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, entry)
	return nil
}

// Entries returns a copy of all recorded audit entries.
func (a *RecordingAuditor) Entries() []audit.AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make([]audit.AuditEntry, len(a.entries))
	copy(cp, a.entries)
	return cp
}

// Reset clears all recorded entries.
func (a *RecordingAuditor) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = a.entries[:0]
}
