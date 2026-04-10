package fin

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

	// ListAssessmentsByOrg returns non-deleted assessments for the given org,
	// supporting cursor-based pagination ordered by id.
	// afterID is the cursor from the previous page; hasMore is true when more items exist.
	ListAssessmentsByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Assessment, bool, error)

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

	// ListLedgerByUnit returns ledger entries for a unit, supporting cursor-based
	// pagination ordered by id.
	// afterID is the cursor from the previous page; hasMore is true when more items exist.
	ListLedgerByUnit(ctx context.Context, unitID uuid.UUID, limit int, afterID *uuid.UUID) ([]LedgerEntry, bool, error)

	// ListLedgerByOrg returns all ledger entries for an org ordered by
	// effective_date ASC, created_at ASC. Returns an empty (non-nil) slice
	// when none exist.
	ListLedgerByOrg(ctx context.Context, orgID uuid.UUID) ([]LedgerEntry, error)

	// GetUnitBalance returns the balance_cents from the most recent ledger
	// entry for the given unit, or 0 if no entries exist.
	GetUnitBalance(ctx context.Context, unitID uuid.UUID) (int64, error)

	// FindLedgerEntryByID returns the ledger entry with the given id, or nil, nil if not found.
	FindLedgerEntryByID(ctx context.Context, id uuid.UUID) (*LedgerEntry, error)

	// FindLedgerEntriesByAssessment returns all ledger entries linked to the given assessment.
	FindLedgerEntriesByAssessment(ctx context.Context, assessmentID uuid.UUID) ([]LedgerEntry, error)

	// FindLedgerEntryByPaymentRef returns the ledger entry whose reference_type is
	// "payment" and reference_id matches paymentID, or nil, nil if not found.
	FindLedgerEntryByPaymentRef(ctx context.Context, paymentID uuid.UUID) (*LedgerEntry, error)

	// UpdateLedgerEntryReversedBy sets the reversed_by_entry_id field on the given entry.
	UpdateLedgerEntryReversedBy(ctx context.Context, entryID, reversalEntryID uuid.UUID) error

	// UpdateAssessmentStatus updates the status and void metadata on an assessment.
	UpdateAssessmentStatus(ctx context.Context, id uuid.UUID, status AssessmentStatus, voidedBy *uuid.UUID, voidedAt *time.Time) error

	// WithTx returns a copy of the repository that runs queries against the
	// given transaction. Used by UnitOfWork to enlist the repo in a shared tx.
	WithTx(tx pgx.Tx) AssessmentRepository
}
