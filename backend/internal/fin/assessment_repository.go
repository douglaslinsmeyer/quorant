package fin

import (
	"context"

	"github.com/google/uuid"
)

// AssessmentRepository persists and retrieves assessment schedules, assessments,
// and ledger entries for the Finance module.
type AssessmentRepository interface {
	// ── Schedule CRUD ─────────────────────────────────────────────────────────

	// CreateSchedule inserts a new assessment schedule and returns the
	// fully-populated row (including generated id and timestamps).
	CreateSchedule(ctx context.Context, s *AssessmentSchedule) (*AssessmentSchedule, error)

	// FindScheduleByID returns the schedule with the given id, or nil, nil if
	// no matching (non-deleted) row exists.
	FindScheduleByID(ctx context.Context, id uuid.UUID) (*AssessmentSchedule, error)

	// ListSchedulesByOrg returns all non-deleted schedules for the given org,
	// ordered by created_at. Returns an empty (non-nil) slice when none exist.
	ListSchedulesByOrg(ctx context.Context, orgID uuid.UUID) ([]AssessmentSchedule, error)

	// UpdateSchedule persists changes to an existing schedule and returns the
	// updated row.
	UpdateSchedule(ctx context.Context, s *AssessmentSchedule) (*AssessmentSchedule, error)

	// DeactivateSchedule sets is_active = false for the given schedule.
	DeactivateSchedule(ctx context.Context, id uuid.UUID) error

	// ── Assessment CRUD ───────────────────────────────────────────────────────

	// CreateAssessment inserts a new assessment charge and returns the
	// fully-populated row.
	CreateAssessment(ctx context.Context, a *Assessment) (*Assessment, error)

	// FindAssessmentByID returns the assessment with the given id, or nil, nil
	// if not found or soft-deleted.
	FindAssessmentByID(ctx context.Context, id uuid.UUID) (*Assessment, error)

	// ListAssessmentsByOrg returns all non-deleted assessments for the given
	// org ordered by due_date. Returns an empty (non-nil) slice when none exist.
	ListAssessmentsByOrg(ctx context.Context, orgID uuid.UUID) ([]Assessment, error)

	// ListAssessmentsByUnit returns all non-deleted assessments for the given
	// unit ordered by due_date. Returns an empty (non-nil) slice when none exist.
	ListAssessmentsByUnit(ctx context.Context, unitID uuid.UUID) ([]Assessment, error)

	// UpdateAssessment persists changes to an existing assessment and returns
	// the updated row.
	UpdateAssessment(ctx context.Context, a *Assessment) (*Assessment, error)

	// SoftDeleteAssessment marks the assessment as deleted without removing the row.
	SoftDeleteAssessment(ctx context.Context, id uuid.UUID) error

	// ── Ledger ────────────────────────────────────────────────────────────────

	// CreateLedgerEntry inserts an immutable ledger entry. It computes
	// balance_cents automatically by adding amount_cents to the previous
	// entry's balance for the same unit (or 0 if this is the first entry).
	// The entry.BalanceCents field is ignored on input; the computed value is
	// reflected in the returned row.
	CreateLedgerEntry(ctx context.Context, entry *LedgerEntry) (*LedgerEntry, error)

	// ListLedgerByUnit returns all ledger entries for a unit ordered by
	// effective_date ASC, created_at ASC. Returns an empty (non-nil) slice
	// when none exist.
	ListLedgerByUnit(ctx context.Context, unitID uuid.UUID) ([]LedgerEntry, error)

	// ListLedgerByOrg returns all ledger entries for an org ordered by
	// effective_date ASC, created_at ASC. Returns an empty (non-nil) slice
	// when none exist.
	ListLedgerByOrg(ctx context.Context, orgID uuid.UUID) ([]LedgerEntry, error)

	// GetUnitBalance returns the balance_cents from the most recent ledger
	// entry for the given unit, or 0 if no entries exist.
	GetUnitBalance(ctx context.Context, unitID uuid.UUID) (int64, error)
}
