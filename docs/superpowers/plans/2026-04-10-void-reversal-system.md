# Void/Reversal System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement GAAP-compliant void/reversal for assessments and payments, with generic ledger and GL reversal mechanics that future transaction types can reuse.

**Architecture:** Two generic reversal functions (ReverseLedgerEntry, ReverseJournalEntry) form the foundation. VoidAssessment and VoidPayment are thin wrappers that enforce domain-specific business rules then delegate to the generic reversals. Schema migrations add status/void tracking columns.

**Tech Stack:** Go, PostgreSQL, pgx, testify, Atlas migrations

**Spec:** `docs/superpowers/specs/2026-04-10-void-reversal-system-design.md`

**Worktree:** `.worktrees/feat-void-reversal` (branch `feature/void-reversal-system`)

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `backend/internal/fin/enums.go` | Add LedgerEntryTypeReversal, LedgerRefTypeReversal, PaymentStatusVoid, AssessmentStatus type |
| Modify | `backend/internal/fin/enums_test.go` | Test new enum values |
| Modify | `backend/internal/fin/domain.go` | Add fields to Assessment, Payment, LedgerEntry structs |
| Modify | `backend/internal/fin/assessment_repository.go` | Add new interface methods |
| Modify | `backend/internal/fin/gl_repository.go` | Add new interface methods |
| Modify | `backend/internal/fin/payment_repository.go` | Add new interface method |
| Modify | `backend/internal/fin/assessment_postgres.go` | Implement new repo methods, update scanLedgerEntry |
| Modify | `backend/internal/fin/gl_postgres.go` | Implement new repo methods |
| Modify | `backend/internal/fin/payment_postgres.go` | Implement UpdatePaymentVoid |
| Modify | `backend/internal/fin/gl_service.go` | Add ReverseJournalEntry method |
| Modify | `backend/internal/fin/gl_service_test.go` | Tests for ReverseJournalEntry |
| Modify | `backend/internal/fin/service.go` | Add ReverseLedgerEntry, VoidAssessment, VoidPayment; update DeleteAssessment |
| Modify | `backend/internal/fin/service_iface.go` | Add VoidAssessment, VoidPayment to Service interface |
| Modify | `backend/internal/fin/service_test.go` | Tests for ReverseLedgerEntry, VoidAssessment, VoidPayment |
| Modify | `backend/internal/fin/handler_assessment.go` | Update DeleteAssessment handler to pass voidedBy |
| Modify | `backend/internal/fin/handler_assessment_test.go` | Update delete tests |
| Create | `backend/internal/fin/handler_payment_void.go` | VoidPayment HTTP handler |
| Modify | `backend/internal/fin/routes.go` | Add VoidPayment route |
| Create | `backend/migrations/20260410000001_void_reversal.sql` | Schema migration |

---

### Task 1: Schema Migration

**Files:**
- Create: `backend/migrations/20260410000001_void_reversal.sql`

- [ ] **Step 1: Create migration file**

```sql
-- Add reversal entry type to ledger
ALTER TYPE ledger_entry_type ADD VALUE IF NOT EXISTS 'reversal';

-- Add assessment status and void tracking
ALTER TABLE assessments
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'posted',
    ADD COLUMN IF NOT EXISTS voided_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ;

-- Backfill: soft-deleted assessments are void
UPDATE assessments SET status = 'void' WHERE deleted_at IS NOT NULL;

-- Add payment void tracking
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'void';

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS voided_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ;

-- Add reversal linkage to ledger entries
ALTER TABLE ledger_entries
    ADD COLUMN IF NOT EXISTS reversed_by_entry_id UUID REFERENCES ledger_entries(id);
```

- [ ] **Step 2: Verify migration syntax**

Run: `cd backend && go build ./...`
Expected: Compiles (migration is SQL-only, no Go changes yet)

- [ ] **Step 3: Commit**

```bash
git add backend/migrations/20260410000001_void_reversal.sql
git commit -m "feat(fin): add void/reversal schema migration (issue #75)"
```

---

### Task 2: Enum Constants and Domain Types

**Files:**
- Modify: `backend/internal/fin/enums.go`
- Modify: `backend/internal/fin/enums_test.go`
- Modify: `backend/internal/fin/domain.go`

- [ ] **Step 1: Write failing tests for new enum values**

In `backend/internal/fin/enums_test.go`, add tests:

```go
func TestAssessmentStatus_IsValid(t *testing.T) {
	valid := []fin.AssessmentStatus{fin.AssessmentStatusPosted, fin.AssessmentStatusVoid}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []fin.AssessmentStatus{"", "unknown", "pending"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestLedgerEntryType_IsValid_Reversal(t *testing.T) {
	assert.True(t, fin.LedgerEntryTypeReversal.IsValid())
}

func TestLedgerReferenceType_IsValid_Reversal(t *testing.T) {
	assert.True(t, fin.LedgerRefTypeReversal.IsValid())
}

func TestPaymentStatus_IsValid_Void(t *testing.T) {
	assert.True(t, fin.PaymentStatusVoid.IsValid())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run "TestAssessmentStatus_IsValid|TestLedgerEntryType_IsValid_Reversal|TestLedgerReferenceType_IsValid_Reversal|TestPaymentStatus_IsValid_Void" -short -count=1`
Expected: Compilation errors — types and constants don't exist yet

- [ ] **Step 3: Add AssessmentStatus type to enums.go**

Add after the `BudgetStatus` block (after line 19 in `enums.go`):

```go
// AssessmentStatus represents the lifecycle state of an assessment.
type AssessmentStatus string

const (
	AssessmentStatusPosted AssessmentStatus = "posted"
	AssessmentStatusVoid   AssessmentStatus = "void"
)

// IsValid returns true if the AssessmentStatus value is one of the defined constants.
func (s AssessmentStatus) IsValid() bool {
	switch s {
	case AssessmentStatusPosted, AssessmentStatusVoid:
		return true
	}
	return false
}
```

- [ ] **Step 4: Add LedgerEntryTypeReversal to existing constants**

In `enums.go`, add `LedgerEntryTypeReversal` to the `LedgerEntryType` const block and update `IsValid`:

```go
const (
	LedgerEntryTypeCharge     LedgerEntryType = "charge"
	LedgerEntryTypePayment    LedgerEntryType = "payment"
	LedgerEntryTypeCredit     LedgerEntryType = "credit"
	LedgerEntryTypeAdjustment LedgerEntryType = "adjustment"
	LedgerEntryTypeLateFee    LedgerEntryType = "late_fee"
	LedgerEntryTypeReversal   LedgerEntryType = "reversal"
)

func (s LedgerEntryType) IsValid() bool {
	switch s {
	case LedgerEntryTypeCharge, LedgerEntryTypePayment, LedgerEntryTypeCredit,
		LedgerEntryTypeAdjustment, LedgerEntryTypeLateFee, LedgerEntryTypeReversal:
		return true
	}
	return false
}
```

- [ ] **Step 5: Add LedgerRefTypeReversal to existing constants**

In `enums.go`, add to the `LedgerReferenceType` const block and update `IsValid`:

```go
const (
	LedgerRefTypePayment  LedgerReferenceType = "payment"
	LedgerRefTypeReversal LedgerReferenceType = "reversal"
)

func (s LedgerReferenceType) IsValid() bool {
	switch s {
	case LedgerRefTypePayment, LedgerRefTypeReversal:
		return true
	}
	return false
}
```

- [ ] **Step 6: Add PaymentStatusVoid to existing constants**

In `enums.go`, add to the `PaymentStatus` const block and update `IsValid`:

```go
const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusVoid      PaymentStatus = "void"
)

func (s PaymentStatus) IsValid() bool {
	switch s {
	case PaymentStatusPending, PaymentStatusCompleted, PaymentStatusFailed, PaymentStatusVoid:
		return true
	}
	return false
}
```

- [ ] **Step 7: Add fields to Assessment struct in domain.go**

Update the `Assessment` struct (lines 37-53 of `domain.go`):

```go
type Assessment struct {
	ID           uuid.UUID         `json:"id"`
	OrgID        uuid.UUID         `json:"org_id"`
	CurrencyCode string            `json:"currency_code"`
	UnitID       uuid.UUID         `json:"unit_id"`
	ScheduleID   *uuid.UUID        `json:"schedule_id,omitempty"`
	Description  string            `json:"description"`
	AmountCents  int64             `json:"amount_cents"`
	DueDate      time.Time         `json:"due_date"`
	GraceDays    *int              `json:"grace_days,omitempty"`
	LateFeeCents *int64            `json:"late_fee_cents,omitempty"`
	IsRecurring  bool              `json:"is_recurring"`
	Status       AssessmentStatus  `json:"status"`
	CreatedBy    *uuid.UUID        `json:"created_by,omitempty"`
	VoidedBy     *uuid.UUID        `json:"voided_by,omitempty"`
	VoidedAt     *time.Time        `json:"voided_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	DeletedAt    *time.Time        `json:"deleted_at,omitempty"`
}
```

- [ ] **Step 8: Add fields to Payment struct in domain.go**

Update the `Payment` struct (lines 85-100 of `domain.go`):

```go
type Payment struct {
	ID              uuid.UUID     `json:"id"`
	OrgID           uuid.UUID     `json:"org_id"`
	CurrencyCode    string        `json:"currency_code"`
	UnitID          uuid.UUID     `json:"unit_id"`
	UserID          uuid.UUID     `json:"user_id"`
	PaymentMethodID *uuid.UUID    `json:"payment_method_id,omitempty"`
	AmountCents     int64         `json:"amount_cents"`
	Status          PaymentStatus `json:"status"`
	ProviderRef     *string       `json:"provider_ref,omitempty"`
	Description     *string       `json:"description,omitempty"`
	PaidAt          *time.Time    `json:"paid_at,omitempty"`
	VoidedBy        *uuid.UUID    `json:"voided_by,omitempty"`
	VoidedAt        *time.Time    `json:"voided_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}
```

- [ ] **Step 9: Add ReversedByEntryID field to LedgerEntry struct in domain.go**

Update the `LedgerEntry` struct (lines 55-70 of `domain.go`):

```go
type LedgerEntry struct {
	ID                uuid.UUID            `json:"id"`
	OrgID             uuid.UUID            `json:"org_id"`
	CurrencyCode      string               `json:"currency_code"`
	UnitID            uuid.UUID            `json:"unit_id"`
	AssessmentID      *uuid.UUID           `json:"assessment_id,omitempty"`
	EntryType         LedgerEntryType      `json:"entry_type"`
	AmountCents       int64                `json:"amount_cents"`
	BalanceCents      int64                `json:"balance_cents"`
	Description       *string              `json:"description,omitempty"`
	ReferenceType     *LedgerReferenceType `json:"reference_type,omitempty"`
	ReferenceID       *uuid.UUID           `json:"reference_id,omitempty"`
	ReversedByEntryID *uuid.UUID           `json:"reversed_by_entry_id,omitempty"`
	EffectiveDate     time.Time            `json:"effective_date"`
	CreatedAt         time.Time            `json:"created_at"`
}
```

- [ ] **Step 10: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run "TestAssessmentStatus_IsValid|TestLedgerEntryType_IsValid_Reversal|TestLedgerReferenceType_IsValid_Reversal|TestPaymentStatus_IsValid_Void" -short -count=1`
Expected: PASS

- [ ] **Step 11: Run all fin tests to check nothing broke**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS (existing tests may need minor updates if they scan LedgerEntry or Assessment structs from mock repos — fix any that break)

- [ ] **Step 12: Commit**

```bash
git add backend/internal/fin/enums.go backend/internal/fin/enums_test.go backend/internal/fin/domain.go
git commit -m "feat(fin): add void/reversal enum constants and domain fields (issue #75)"
```

---

### Task 3: Repository Interfaces and Mock Updates

**Files:**
- Modify: `backend/internal/fin/assessment_repository.go`
- Modify: `backend/internal/fin/gl_repository.go`
- Modify: `backend/internal/fin/payment_repository.go`
- Modify: `backend/internal/fin/service_test.go` (mock implementations)
- Modify: `backend/internal/fin/gl_service_test.go` (mock implementations)
- Modify: `backend/internal/fin/handler_assessment_test.go` (mock implementations)

- [ ] **Step 1: Add methods to AssessmentRepository interface**

In `backend/internal/fin/assessment_repository.go`, add to the `// ── Ledger` section (after `GetUnitBalance`):

```go
	// FindLedgerEntryByID returns the ledger entry with the given id,
	// or nil, nil if not found.
	FindLedgerEntryByID(ctx context.Context, id uuid.UUID) (*LedgerEntry, error)

	// FindLedgerEntriesByAssessment returns all ledger entries linked to
	// the given assessment, ordered by created_at ASC.
	FindLedgerEntriesByAssessment(ctx context.Context, assessmentID uuid.UUID) ([]LedgerEntry, error)

	// FindLedgerEntryByPaymentRef returns the ledger entry with
	// reference_type='payment' and reference_id matching the given payment ID,
	// or nil, nil if not found.
	FindLedgerEntryByPaymentRef(ctx context.Context, paymentID uuid.UUID) (*LedgerEntry, error)

	// UpdateLedgerEntryReversedBy sets the reversed_by_entry_id field on
	// the given ledger entry to link it to its reversal.
	UpdateLedgerEntryReversedBy(ctx context.Context, entryID, reversalEntryID uuid.UUID) error

	// UpdateAssessmentStatus sets the status, voided_by, and voided_at fields
	// on the given assessment.
	UpdateAssessmentStatus(ctx context.Context, id uuid.UUID, status AssessmentStatus, voidedBy *uuid.UUID, voidedAt *time.Time) error
```

- [ ] **Step 2: Add methods to GLRepository interface**

In `backend/internal/fin/gl_repository.go`, add to the `// ── Journal Entries` section (after `ListJournalEntriesByOrg`):

```go
	// FindJournalEntriesBySource returns all journal entries matching the
	// given source_type and source_id, ordered by entry_number ASC.
	FindJournalEntriesBySource(ctx context.Context, sourceType GLSourceType, sourceID uuid.UUID) ([]GLJournalEntry, error)

	// UpdateJournalEntryReversedBy sets the reversed_by field on the
	// given journal entry to link it to its reversal entry.
	UpdateJournalEntryReversedBy(ctx context.Context, entryID, reversalID uuid.UUID) error
```

- [ ] **Step 3: Add method to PaymentRepository interface**

In `backend/internal/fin/payment_repository.go`, add after `UpdatePaymentStatus`:

```go
	// UpdatePaymentVoid sets the payment status to void and records
	// who voided it and when.
	UpdatePaymentVoid(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID, voidedAt time.Time) error
```

- [ ] **Step 4: Add mock implementations to service_test.go**

Add to `mockAssessmentRepo` in `backend/internal/fin/service_test.go`:

```go
func (r *mockAssessmentRepo) FindLedgerEntryByID(_ context.Context, id uuid.UUID) (*fin.LedgerEntry, error) {
	for _, e := range r.ledger {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, nil
}

func (r *mockAssessmentRepo) FindLedgerEntriesByAssessment(_ context.Context, assessmentID uuid.UUID) ([]fin.LedgerEntry, error) {
	var result []fin.LedgerEntry
	for _, e := range r.ledger {
		if e.AssessmentID != nil && *e.AssessmentID == assessmentID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *mockAssessmentRepo) FindLedgerEntryByPaymentRef(_ context.Context, paymentID uuid.UUID) (*fin.LedgerEntry, error) {
	refType := fin.LedgerRefTypePayment
	for _, e := range r.ledger {
		if e.ReferenceType != nil && *e.ReferenceType == refType && e.ReferenceID != nil && *e.ReferenceID == paymentID {
			return &e, nil
		}
	}
	return nil, nil
}

func (r *mockAssessmentRepo) UpdateLedgerEntryReversedBy(_ context.Context, entryID, reversalEntryID uuid.UUID) error {
	for i, e := range r.ledger {
		if e.ID == entryID {
			r.ledger[i].ReversedByEntryID = &reversalEntryID
			return nil
		}
	}
	return fmt.Errorf("ledger entry %s not found", entryID)
}

func (r *mockAssessmentRepo) UpdateAssessmentStatus(_ context.Context, id uuid.UUID, status fin.AssessmentStatus, voidedBy *uuid.UUID, voidedAt *time.Time) error {
	for i, a := range r.assessments {
		if a.ID == id {
			r.assessments[i].Status = status
			r.assessments[i].VoidedBy = voidedBy
			r.assessments[i].VoidedAt = voidedAt
			return nil
		}
	}
	return fmt.Errorf("assessment %s not found", id)
}
```

Add to `mockPaymentRepo` in `backend/internal/fin/service_test.go`:

```go
func (r *mockPaymentRepo) UpdatePaymentVoid(_ context.Context, id uuid.UUID, voidedBy uuid.UUID, voidedAt time.Time) error {
	for i, p := range r.payments {
		if p.ID == id {
			r.payments[i].Status = fin.PaymentStatusVoid
			r.payments[i].VoidedBy = &voidedBy
			r.payments[i].VoidedAt = &voidedAt
			return nil
		}
	}
	return fmt.Errorf("payment %s not found", id)
}
```

- [ ] **Step 5: Add mock implementations to handler_assessment_test.go**

The same mock types are used in handler tests. Add the same methods to the mock repos in `handler_assessment_test.go`. If the mocks are shared via the `fin_test` package, they only need to be added once. Check which file defines the mock types and add to that file only.

- [ ] **Step 6: Add mock implementations to gl_service_test.go**

Add to `mockGLRepo` in `backend/internal/fin/gl_service_test.go`:

```go
func (r *mockGLRepo) FindJournalEntriesBySource(_ context.Context, sourceType fin.GLSourceType, sourceID uuid.UUID) ([]fin.GLJournalEntry, error) {
	var result []fin.GLJournalEntry
	for _, e := range r.entries {
		if e.SourceType != nil && *e.SourceType == sourceType && e.SourceID != nil && *e.SourceID == sourceID {
			result = append(result, *e)
		}
	}
	return result, nil
}

func (r *mockGLRepo) UpdateJournalEntryReversedBy(_ context.Context, entryID, reversalID uuid.UUID) error {
	e, ok := r.entries[entryID]
	if !ok {
		return fmt.Errorf("journal entry %s not found", entryID)
	}
	e.ReversedBy = &reversalID
	return nil
}
```

- [ ] **Step 7: Verify compilation**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS (all existing tests still pass with new interface methods satisfied by mocks)

- [ ] **Step 8: Commit**

```bash
git add backend/internal/fin/assessment_repository.go backend/internal/fin/gl_repository.go backend/internal/fin/payment_repository.go backend/internal/fin/service_test.go backend/internal/fin/gl_service_test.go backend/internal/fin/handler_assessment_test.go
git commit -m "feat(fin): add void/reversal repository interfaces and test mocks (issue #75)"
```

---

### Task 4: ReverseJournalEntry (GL Service)

**Files:**
- Modify: `backend/internal/fin/gl_service.go`
- Modify: `backend/internal/fin/gl_service_test.go`

- [ ] **Step 1: Write failing test for happy path**

In `backend/internal/fin/gl_service_test.go`:

```go
func TestGLService_ReverseJournalEntry(t *testing.T) {
	svc, repo := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()
	sourceType := fin.GLSourceTypeAssessment
	sourceID := uuid.New()
	unitID := uuid.New()
	acct1 := uuid.New()
	acct2 := uuid.New()

	// Post original entry: Debit AR 500, Credit Revenue 500
	original, err := svc.PostSystemJournalEntry(ctx, orgID, uuid.Nil,
		time.Now(), "Assessment charge", &sourceType, &sourceID, &unitID,
		[]fin.GLJournalLine{
			{AccountID: acct1, DebitCents: 50000, CreditCents: 0},
			{AccountID: acct2, DebitCents: 0, CreditCents: 50000},
		})
	require.NoError(t, err)

	// Reverse it
	reversal, err := svc.ReverseJournalEntry(ctx, original.ID, uuid.New())
	require.NoError(t, err)

	// Reversal entry checks
	assert.True(t, reversal.IsReversal)
	assert.Equal(t, "Reversal: Assessment charge", reversal.Memo)
	assert.Equal(t, &sourceType, reversal.SourceType)
	assert.Equal(t, &sourceID, reversal.SourceID)
	assert.Equal(t, &unitID, reversal.UnitID)
	require.Len(t, reversal.Lines, 2)

	// Lines should be swapped: Credit AR 500, Debit Revenue 500
	assert.Equal(t, acct1, reversal.Lines[0].AccountID)
	assert.Equal(t, int64(0), reversal.Lines[0].DebitCents)
	assert.Equal(t, int64(50000), reversal.Lines[0].CreditCents)
	assert.Equal(t, acct2, reversal.Lines[1].AccountID)
	assert.Equal(t, int64(50000), reversal.Lines[1].DebitCents)
	assert.Equal(t, int64(0), reversal.Lines[1].CreditCents)

	// Original should be marked as reversed
	updated := repo.entries[original.ID]
	require.NotNil(t, updated.ReversedBy)
	assert.Equal(t, reversal.ID, *updated.ReversedBy)
}
```

- [ ] **Step 2: Write failing test for already-reversed error**

```go
func TestGLService_ReverseJournalEntry_AlreadyReversed(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()
	orgID := uuid.New()
	acct1, acct2 := uuid.New(), uuid.New()

	original, err := svc.PostSystemJournalEntry(ctx, orgID, uuid.Nil,
		time.Now(), "Test entry", nil, nil, nil,
		[]fin.GLJournalLine{
			{AccountID: acct1, DebitCents: 1000, CreditCents: 0},
			{AccountID: acct2, DebitCents: 0, CreditCents: 1000},
		})
	require.NoError(t, err)

	_, err = svc.ReverseJournalEntry(ctx, original.ID, uuid.New())
	require.NoError(t, err)

	// Second reversal should fail
	_, err = svc.ReverseJournalEntry(ctx, original.ID, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already reversed")
}
```

- [ ] **Step 3: Write failing test for not-found error**

```go
func TestGLService_ReverseJournalEntry_NotFound(t *testing.T) {
	svc, _ := newTestGLService()
	ctx := context.Background()

	_, err := svc.ReverseJournalEntry(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run "TestGLService_ReverseJournalEntry" -short -count=1`
Expected: Compilation error — `ReverseJournalEntry` method doesn't exist

- [ ] **Step 5: Add FindJournalEntriesBySource pass-through on GLService**

`FinService` holds a `*GLService` pointer but `GLService.gl` (the repo) is private. `VoidAssessment` and `VoidPayment` need to find GL entries by source. Add a pass-through method in `backend/internal/fin/gl_service.go`:

```go
// FindJournalEntriesBySource delegates to the repository to find journal
// entries by source_type and source_id.
func (s *GLService) FindJournalEntriesBySource(ctx context.Context, sourceType GLSourceType, sourceID uuid.UUID) ([]GLJournalEntry, error) {
	return s.gl.FindJournalEntriesBySource(ctx, sourceType, sourceID)
}
```

- [ ] **Step 6: Implement ReverseJournalEntry**

In `backend/internal/fin/gl_service.go`, add:

```go
// ReverseJournalEntry creates a reversing journal entry that swaps all debits
// and credits from the original entry. It marks the original as reversed and
// sets is_reversal=true on the new entry. Returns an error if the original
// has already been reversed or does not exist.
func (s *GLService) ReverseJournalEntry(ctx context.Context, entryID, reversedBy uuid.UUID) (*GLJournalEntry, error) {
	original, err := s.gl.FindJournalEntryByID(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("gl: ReverseJournalEntry find original: %w", err)
	}
	if original == nil {
		return nil, fmt.Errorf("gl: journal entry %s not found", entryID)
	}
	if original.ReversedBy != nil {
		return nil, fmt.Errorf("gl: journal entry %s already reversed", entryID)
	}

	// Build reversal lines with swapped debits/credits.
	reversalLines := make([]GLJournalLine, len(original.Lines))
	for i, line := range original.Lines {
		reversalLines[i] = GLJournalLine{
			AccountID:   line.AccountID,
			DebitCents:  line.CreditCents,
			CreditCents: line.DebitCents,
			Memo:        line.Memo,
		}
	}

	reversal := &GLJournalEntry{
		OrgID:      original.OrgID,
		EntryDate:  time.Now(),
		Memo:       "Reversal: " + original.Memo,
		SourceType: original.SourceType,
		SourceID:   original.SourceID,
		UnitID:     original.UnitID,
		PostedBy:   reversedBy,
		IsReversal: true,
		Lines:      reversalLines,
	}

	posted, err := s.gl.PostJournalEntry(ctx, reversal)
	if err != nil {
		return nil, fmt.Errorf("gl: ReverseJournalEntry post reversal: %w", err)
	}

	if err := s.gl.UpdateJournalEntryReversedBy(ctx, entryID, posted.ID); err != nil {
		return nil, fmt.Errorf("gl: ReverseJournalEntry update original: %w", err)
	}

	return posted, nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run "TestGLService_ReverseJournalEntry" -short -count=1`
Expected: PASS (all 3 tests)

- [ ] **Step 8: Run all fin tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add backend/internal/fin/gl_service.go backend/internal/fin/gl_service_test.go
git commit -m "feat(fin): implement ReverseJournalEntry and FindJournalEntriesBySource on GLService (issue #75)"
```

---

### Task 5: Postgres Repository Implementations

**Files:**
- Modify: `backend/internal/fin/assessment_postgres.go`
- Modify: `backend/internal/fin/gl_postgres.go`
- Modify: `backend/internal/fin/payment_postgres.go`

- [ ] **Step 1: Update scanLedgerEntry to include reversed_by_entry_id**

In `backend/internal/fin/assessment_postgres.go`, update `scanLedgerEntry` (line 709) to scan the new column. The RETURNING clauses in `CreateLedgerEntry` and any SELECT queries for ledger entries must also include `reversed_by_entry_id`.

Update `scanLedgerEntry`:

```go
func scanLedgerEntry(row pgx.Row) (*LedgerEntry, error) {
	var e LedgerEntry
	err := row.Scan(
		&e.ID,
		&e.OrgID,
		&e.CurrencyCode,
		&e.UnitID,
		&e.AssessmentID,
		&e.EntryType,
		&e.AmountCents,
		&e.BalanceCents,
		&e.Description,
		&e.ReferenceType,
		&e.ReferenceID,
		&e.ReversedByEntryID,
		&e.EffectiveDate,
		&e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
```

Update the RETURNING clause in `CreateLedgerEntry` (and any other ledger queries) to include `reversed_by_entry_id` in the correct position (after `reference_id`, before `effective_date`).

- [ ] **Step 2: Implement FindLedgerEntryByID**

In `backend/internal/fin/assessment_postgres.go`:

```go
// FindLedgerEntryByID returns the ledger entry with the given id, or nil, nil
// if not found.
func (r *PostgresAssessmentRepository) FindLedgerEntryByID(ctx context.Context, id uuid.UUID) (*LedgerEntry, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, assessment_id, entry_type,
		       amount_cents, balance_cents, description, reference_type, reference_id,
		       reversed_by_entry_id, effective_date, created_at
		FROM ledger_entries
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	entry, err := scanLedgerEntry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindLedgerEntryByID: %w", err)
	}
	return entry, nil
}
```

- [ ] **Step 3: Implement FindLedgerEntriesByAssessment**

```go
// FindLedgerEntriesByAssessment returns all ledger entries linked to the given
// assessment, ordered by created_at ASC.
func (r *PostgresAssessmentRepository) FindLedgerEntriesByAssessment(ctx context.Context, assessmentID uuid.UUID) ([]LedgerEntry, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, assessment_id, entry_type,
		       amount_cents, balance_cents, description, reference_type, reference_id,
		       reversed_by_entry_id, effective_date, created_at
		FROM ledger_entries
		WHERE assessment_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("fin: FindLedgerEntriesByAssessment: %w", err)
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		e, err := scanLedgerEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("fin: FindLedgerEntriesByAssessment scan: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, nil
}
```

- [ ] **Step 4: Implement FindLedgerEntryByPaymentRef**

```go
// FindLedgerEntryByPaymentRef returns the ledger entry with reference_type='payment'
// and reference_id matching the given payment ID, or nil, nil if not found.
func (r *PostgresAssessmentRepository) FindLedgerEntryByPaymentRef(ctx context.Context, paymentID uuid.UUID) (*LedgerEntry, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, assessment_id, entry_type,
		       amount_cents, balance_cents, description, reference_type, reference_id,
		       reversed_by_entry_id, effective_date, created_at
		FROM ledger_entries
		WHERE reference_type = 'payment' AND reference_id = $1`

	row := r.pool.QueryRow(ctx, q, paymentID)
	entry, err := scanLedgerEntry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindLedgerEntryByPaymentRef: %w", err)
	}
	return entry, nil
}
```

- [ ] **Step 5: Implement UpdateLedgerEntryReversedBy**

```go
// UpdateLedgerEntryReversedBy sets the reversed_by_entry_id field on the given
// ledger entry to link it to its reversal.
func (r *PostgresAssessmentRepository) UpdateLedgerEntryReversedBy(ctx context.Context, entryID, reversalEntryID uuid.UUID) error {
	const q = `UPDATE ledger_entries SET reversed_by_entry_id = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, reversalEntryID, entryID)
	if err != nil {
		return fmt.Errorf("fin: UpdateLedgerEntryReversedBy: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Implement UpdateAssessmentStatus**

In `backend/internal/fin/assessment_postgres.go`:

```go
// UpdateAssessmentStatus sets the status, voided_by, and voided_at fields on
// the given assessment.
func (r *PostgresAssessmentRepository) UpdateAssessmentStatus(ctx context.Context, id uuid.UUID, status AssessmentStatus, voidedBy *uuid.UUID, voidedAt *time.Time) error {
	const q = `
		UPDATE assessments
		SET status = $1, voided_by = $2, voided_at = $3, updated_at = now()
		WHERE id = $4`

	_, err := r.pool.Exec(ctx, q, status, voidedBy, voidedAt, id)
	if err != nil {
		return fmt.Errorf("fin: UpdateAssessmentStatus: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Implement FindJournalEntriesBySource in gl_postgres.go**

In `backend/internal/fin/gl_postgres.go`:

```go
// FindJournalEntriesBySource returns all journal entries matching the given
// source_type and source_id, ordered by entry_number ASC. Uses the
// idx_gl_journal_entries_source index.
func (r *PostgresGLRepository) FindJournalEntriesBySource(ctx context.Context, sourceType GLSourceType, sourceID uuid.UUID) ([]GLJournalEntry, error) {
	const q = `
		SELECT id, org_id, entry_number, entry_date, memo, source_type,
		       source_id, unit_id, posted_by, reversed_by, is_reversal, created_at
		FROM gl_journal_entries
		WHERE source_type = $1 AND source_id = $2
		ORDER BY entry_number ASC`

	rows, err := r.pool.Query(ctx, q, sourceType, sourceID)
	if err != nil {
		return nil, fmt.Errorf("fin: FindJournalEntriesBySource: %w", err)
	}
	defer rows.Close()

	var entries []GLJournalEntry
	for rows.Next() {
		e, err := scanGLJournalEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("fin: FindJournalEntriesBySource scan: %w", err)
		}
		// Fetch lines for each entry.
		linesRows, err := r.pool.Query(ctx, `
			SELECT id, journal_entry_id, account_id, debit_cents, credit_cents, memo
			FROM gl_journal_lines WHERE journal_entry_id = $1 ORDER BY id`, e.ID)
		if err != nil {
			return nil, fmt.Errorf("fin: FindJournalEntriesBySource lines: %w", err)
		}
		lines, err := collectGLJournalLines(linesRows, "FindJournalEntriesBySource")
		if err != nil {
			return nil, err
		}
		e.Lines = lines
		entries = append(entries, *e)
	}
	return entries, nil
}
```

- [ ] **Step 8: Implement UpdateJournalEntryReversedBy in gl_postgres.go**

```go
// UpdateJournalEntryReversedBy sets the reversed_by field on the given journal
// entry to link it to its reversal entry.
func (r *PostgresGLRepository) UpdateJournalEntryReversedBy(ctx context.Context, entryID, reversalID uuid.UUID) error {
	const q = `UPDATE gl_journal_entries SET reversed_by = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, q, reversalID, entryID)
	if err != nil {
		return fmt.Errorf("fin: UpdateJournalEntryReversedBy: %w", err)
	}
	return nil
}
```

- [ ] **Step 9: Implement UpdatePaymentVoid in payment_postgres.go**

In `backend/internal/fin/payment_postgres.go`:

```go
// UpdatePaymentVoid sets the payment status to void and records who voided it
// and when.
func (r *PostgresPaymentRepository) UpdatePaymentVoid(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID, voidedAt time.Time) error {
	const q = `
		UPDATE payments
		SET status     = 'void',
		    voided_by  = $1,
		    voided_at  = $2,
		    updated_at = now()
		WHERE id = $3`

	_, err := r.pool.Exec(ctx, q, voidedBy, voidedAt, id)
	if err != nil {
		return fmt.Errorf("fin: UpdatePaymentVoid: %w", err)
	}
	return nil
}
```

- [ ] **Step 10: Update all existing ledger SELECT/RETURNING clauses**

Search for all SQL queries in `assessment_postgres.go` that SELECT or RETURN ledger_entries columns and add `reversed_by_entry_id` in the correct position (after `reference_id`, before `effective_date`). This includes:
- `CreateLedgerEntry` RETURNING clause
- `ListLedgerByUnit` SELECT
- `ListLedgerByOrg` SELECT
- Any other ledger queries

- [ ] **Step 11: Verify compilation and tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS

- [ ] **Step 12: Commit**

```bash
git add backend/internal/fin/assessment_postgres.go backend/internal/fin/gl_postgres.go backend/internal/fin/payment_postgres.go
git commit -m "feat(fin): implement void/reversal repository methods (issue #75)"
```

---

### Task 6: ReverseLedgerEntry (FinService)

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Write failing test for ReverseLedgerEntry happy path**

In `backend/internal/fin/service_test.go`:

```go
func TestReverseLedgerEntry(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()

	// Create an assessment to get a ledger entry
	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	_, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	require.Len(t, assessmentRepo.ledger, 1)
	originalEntry := assessmentRepo.ledger[0]

	// Reverse it
	reversal, err := svc.ReverseLedgerEntry(ctx, originalEntry.ID, uuid.New())
	require.NoError(t, err)

	// Reversal entry checks
	assert.Equal(t, fin.LedgerEntryTypeReversal, reversal.EntryType)
	assert.Equal(t, int64(-15000), reversal.AmountCents)
	assert.Equal(t, unitID, reversal.UnitID)
	assert.Equal(t, orgID, reversal.OrgID)

	refType := fin.LedgerRefTypeReversal
	assert.Equal(t, &refType, reversal.ReferenceType)
	assert.Equal(t, &originalEntry.ID, reversal.ReferenceID)

	// Original should be marked as reversed
	assert.NotNil(t, assessmentRepo.ledger[0].ReversedByEntryID)
	assert.Equal(t, reversal.ID, *assessmentRepo.ledger[0].ReversedByEntryID)

	// Balance should be back to zero
	assert.Equal(t, int64(0), reversal.BalanceCents)
}
```

- [ ] **Step 2: Write failing test for already-reversed error**

```go
func TestReverseLedgerEntry_AlreadyReversed(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()

	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	_, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	originalEntry := assessmentRepo.ledger[0]

	_, err = svc.ReverseLedgerEntry(ctx, originalEntry.ID, uuid.New())
	require.NoError(t, err)

	// Second reversal should fail
	_, err = svc.ReverseLedgerEntry(ctx, originalEntry.ID, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already reversed")
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run "TestReverseLedgerEntry" -short -count=1`
Expected: Compilation error — method doesn't exist

- [ ] **Step 4: Implement ReverseLedgerEntry**

In `backend/internal/fin/service.go`:

```go
// ReverseLedgerEntry creates a reversing ledger entry that negates the original
// entry's amount. It marks the original as reversed via reversed_by_entry_id.
// Returns an error if the original has already been reversed or does not exist.
func (s *FinService) ReverseLedgerEntry(ctx context.Context, entryID, reversedBy uuid.UUID) (*LedgerEntry, error) {
	original, err := s.assessments.FindLedgerEntryByID(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("fin: ReverseLedgerEntry find original: %w", err)
	}
	if original == nil {
		return nil, api.NewNotFoundError("fin.ledger_entry.not_found")
	}
	if original.ReversedByEntryID != nil {
		return nil, fmt.Errorf("fin: ledger entry %s already reversed", entryID)
	}

	desc := "Reversal"
	if original.Description != nil {
		desc = "Reversal: " + *original.Description
	}
	refType := LedgerRefTypeReversal
	reversal := &LedgerEntry{
		OrgID:         original.OrgID,
		CurrencyCode:  original.CurrencyCode,
		UnitID:        original.UnitID,
		AssessmentID:  original.AssessmentID,
		EntryType:     LedgerEntryTypeReversal,
		AmountCents:   -original.AmountCents,
		Description:   &desc,
		ReferenceType: &refType,
		ReferenceID:   &entryID,
		EffectiveDate: time.Now(),
	}

	created, err := s.assessments.CreateLedgerEntry(ctx, reversal)
	if err != nil {
		return nil, fmt.Errorf("fin: ReverseLedgerEntry create: %w", err)
	}

	if err := s.assessments.UpdateLedgerEntryReversedBy(ctx, entryID, created.ID); err != nil {
		return nil, fmt.Errorf("fin: ReverseLedgerEntry update original: %w", err)
	}

	return created, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run "TestReverseLedgerEntry" -short -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "feat(fin): implement ReverseLedgerEntry on FinService (issue #75)"
```

---

### Task 7: VoidAssessment (FinService)

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_iface.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Write failing test for VoidAssessment happy path**

In `backend/internal/fin/service_test.go`:

```go
func TestVoidAssessment(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	voidedBy := uuid.New()

	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)

	// Set status to posted (CreateAssessment doesn't set it in current code)
	assessmentRepo.assessments[0].Status = fin.AssessmentStatusPosted

	err = svc.VoidAssessment(ctx, assessment.ID, voidedBy)
	require.NoError(t, err)

	// Assessment should be void
	assert.Equal(t, fin.AssessmentStatusVoid, assessmentRepo.assessments[0].Status)
	assert.Equal(t, &voidedBy, assessmentRepo.assessments[0].VoidedBy)
	assert.NotNil(t, assessmentRepo.assessments[0].VoidedAt)

	// Ledger should have reversal entry
	require.Len(t, assessmentRepo.ledger, 2)
	reversal := assessmentRepo.ledger[1]
	assert.Equal(t, fin.LedgerEntryTypeReversal, reversal.EntryType)
	assert.Equal(t, int64(-15000), reversal.AmountCents)
}
```

- [ ] **Step 2: Write failing test for VoidAssessment blocked by payments**

```go
func TestVoidAssessment_BlockedByPayments(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	userID := uuid.New()

	// Create assessment
	aReq := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	assessment, err := svc.CreateAssessment(ctx, orgID, aReq)
	require.NoError(t, err)
	assessmentRepo.assessments[0].Status = fin.AssessmentStatusPosted

	// Record a payment (creates a payment-type ledger entry with this assessment's unit)
	pReq := fin.CreatePaymentRequest{
		UnitID:      unitID,
		AmountCents: 15000,
	}
	_, err = svc.RecordPayment(ctx, orgID, userID, pReq)
	require.NoError(t, err)

	// Void should be blocked
	err = svc.VoidAssessment(ctx, assessment.ID, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has_payments")
}
```

- [ ] **Step 3: Write failing test for already-void error**

```go
func TestVoidAssessment_AlreadyVoid(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()

	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	assessmentRepo.assessments[0].Status = fin.AssessmentStatusVoid

	err = svc.VoidAssessment(ctx, assessment.ID, uuid.New())
	require.Error(t, err)
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run "TestVoidAssessment" -short -count=1`
Expected: Compilation error

- [ ] **Step 5: Add VoidAssessment to Service interface**

In `backend/internal/fin/service_iface.go`, add to the `// Assessments` section:

```go
	VoidAssessment(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID) error
```

- [ ] **Step 6: Implement VoidAssessment**

In `backend/internal/fin/service.go`:

```go
// VoidAssessment reverses the ledger charge and GL journal entry for an
// assessment, then marks it as void. Returns a validation error if the
// assessment has any associated payments (void payments first).
func (s *FinService) VoidAssessment(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID) error {
	assessment, err := s.GetAssessment(ctx, id)
	if err != nil {
		return err
	}
	if assessment.Status == AssessmentStatusVoid {
		return api.NewValidationError("fin.assessment.already_void", "status")
	}

	// Check for payment-type ledger entries linked to this assessment.
	entries, err := s.assessments.FindLedgerEntriesByAssessment(ctx, id)
	if err != nil {
		return fmt.Errorf("fin: VoidAssessment find ledger entries: %w", err)
	}
	for _, e := range entries {
		if e.EntryType == LedgerEntryTypePayment {
			return api.NewValidationError("fin.assessment.has_payments", "id",
				api.P("assessment_id", id.String()))
		}
	}

	// Find and reverse the original charge ledger entry.
	var chargeEntry *LedgerEntry
	for _, e := range entries {
		if e.EntryType == LedgerEntryTypeCharge && e.ReversedByEntryID == nil {
			chargeEntry = &e
			break
		}
	}
	if chargeEntry == nil {
		return fmt.Errorf("fin: VoidAssessment: no unreversed charge entry for assessment %s", id)
	}

	if _, err := s.ReverseLedgerEntry(ctx, chargeEntry.ID, voidedBy); err != nil {
		return fmt.Errorf("fin: VoidAssessment reverse ledger: %w", err)
	}

	// Reverse GL entry if GL service is configured.
	if s.gl != nil {
		sourceType := GLSourceTypeAssessment
		glEntries, err := s.gl.FindJournalEntriesBySource(ctx, sourceType, id)
		if err != nil {
			s.logger.Error("GL: failed to find assessment journal entries", "assessment_id", id, "error", err)
		}
		for _, glEntry := range glEntries {
			if glEntry.ReversedBy == nil && !glEntry.IsReversal {
				if _, glErr := s.gl.ReverseJournalEntry(ctx, glEntry.ID, voidedBy); glErr != nil {
					s.logger.Error("GL: failed to reverse assessment journal entry", "assessment_id", id, "journal_entry_id", glEntry.ID, "error", glErr)
				}
			}
		}
	}

	// Update assessment status to void.
	now := time.Now()
	if err := s.assessments.UpdateAssessmentStatus(ctx, id, AssessmentStatusVoid, &voidedBy, &now); err != nil {
		return fmt.Errorf("fin: VoidAssessment update status: %w", err)
	}

	return nil
}
```

- [ ] **Step 7: Update DeleteAssessment to call VoidAssessment**

Replace the existing `DeleteAssessment` in `service.go`:

```go
// DeleteAssessment voids an assessment, reversing its ledger and GL entries.
func (s *FinService) DeleteAssessment(ctx context.Context, id uuid.UUID) error {
	return s.VoidAssessment(ctx, id, uuid.Nil)
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run "TestVoidAssessment" -short -count=1`
Expected: PASS

- [ ] **Step 9: Run all fin tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS (existing DeleteAssessment tests still work since it delegates to VoidAssessment)

- [ ] **Step 10: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_iface.go backend/internal/fin/service_test.go
git commit -m "feat(fin): implement VoidAssessment with ledger and GL reversal (issue #75)"
```

---

### Task 8: VoidPayment (FinService)

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_iface.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Write failing test for VoidPayment happy path**

In `backend/internal/fin/service_test.go`:

```go
func TestVoidPayment(t *testing.T) {
	svc, assessmentRepo, paymentRepo, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	userID := uuid.New()
	voidedBy := uuid.New()

	// Create assessment and payment
	aReq := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	_, err := svc.CreateAssessment(ctx, orgID, aReq)
	require.NoError(t, err)

	pReq := fin.CreatePaymentRequest{
		UnitID:      unitID,
		AmountCents: 15000,
	}
	payment, err := svc.RecordPayment(ctx, orgID, userID, pReq)
	require.NoError(t, err)

	// Ledger: charge (+15000) then payment (-15000) = balance 0
	require.Len(t, assessmentRepo.ledger, 2)

	err = svc.VoidPayment(ctx, payment.ID, voidedBy)
	require.NoError(t, err)

	// Payment should be void
	assert.Equal(t, fin.PaymentStatusVoid, paymentRepo.payments[0].Status)
	assert.Equal(t, &voidedBy, paymentRepo.payments[0].VoidedBy)
	assert.NotNil(t, paymentRepo.payments[0].VoidedAt)

	// Ledger should have reversal entry (positive, restoring balance)
	require.Len(t, assessmentRepo.ledger, 3)
	reversal := assessmentRepo.ledger[2]
	assert.Equal(t, fin.LedgerEntryTypeReversal, reversal.EntryType)
	assert.Equal(t, int64(15000), reversal.AmountCents) // Positive — reverses the negative payment
}
```

- [ ] **Step 2: Write failing test for VoidPayment already void**

```go
func TestVoidPayment_AlreadyVoid(t *testing.T) {
	svc, _, paymentRepo, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	userID := uuid.New()

	pReq := fin.CreatePaymentRequest{
		UnitID:      unitID,
		AmountCents: 15000,
	}
	payment, err := svc.RecordPayment(ctx, orgID, userID, pReq)
	require.NoError(t, err)

	paymentRepo.payments[0].Status = fin.PaymentStatusVoid

	err = svc.VoidPayment(ctx, payment.ID, uuid.New())
	require.Error(t, err)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run "TestVoidPayment" -short -count=1`
Expected: Compilation error

- [ ] **Step 4: Add VoidPayment to Service interface**

In `backend/internal/fin/service_iface.go`, add to the `// Payments` section:

```go
	VoidPayment(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID) error
```

- [ ] **Step 5: Implement VoidPayment**

In `backend/internal/fin/service.go`:

```go
// VoidPayment reverses the ledger entry and GL journal entry for a payment,
// then marks it as void. This restores the unit's receivable balance.
func (s *FinService) VoidPayment(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID) error {
	payment, err := s.GetPayment(ctx, id)
	if err != nil {
		return err
	}
	if payment.Status != PaymentStatusCompleted {
		return api.NewValidationError("fin.payment.invalid_void_status", "status",
			api.P("current", string(payment.Status)),
			api.P("expected", string(PaymentStatusCompleted)))
	}

	// Find and reverse the payment's ledger entry.
	ledgerEntry, err := s.assessments.FindLedgerEntryByPaymentRef(ctx, id)
	if err != nil {
		return fmt.Errorf("fin: VoidPayment find ledger entry: %w", err)
	}
	if ledgerEntry != nil && ledgerEntry.ReversedByEntryID == nil {
		if _, err := s.ReverseLedgerEntry(ctx, ledgerEntry.ID, voidedBy); err != nil {
			return fmt.Errorf("fin: VoidPayment reverse ledger: %w", err)
		}
	}

	// Reverse GL entry if GL service is configured.
	if s.gl != nil {
		sourceType := GLSourceTypePayment
		glEntries, err := s.gl.FindJournalEntriesBySource(ctx, sourceType, id)
		if err != nil {
			s.logger.Error("GL: failed to find payment journal entries", "payment_id", id, "error", err)
		}
		for _, glEntry := range glEntries {
			if glEntry.ReversedBy == nil && !glEntry.IsReversal {
				if _, glErr := s.gl.ReverseJournalEntry(ctx, glEntry.ID, voidedBy); glErr != nil {
					s.logger.Error("GL: failed to reverse payment journal entry", "payment_id", id, "journal_entry_id", glEntry.ID, "error", glErr)
				}
			}
		}
	}

	// Mark payment as void.
	now := time.Now()
	if err := s.payments.UpdatePaymentVoid(ctx, id, voidedBy, now); err != nil {
		return fmt.Errorf("fin: VoidPayment update status: %w", err)
	}

	return nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run "TestVoidPayment" -short -count=1`
Expected: PASS

- [ ] **Step 7: Run all fin tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_iface.go backend/internal/fin/service_test.go
git commit -m "feat(fin): implement VoidPayment with ledger and GL reversal (issue #75)"
```

---

### Task 9: Update Handler and Add VoidPayment Route

**Files:**
- Modify: `backend/internal/fin/handler_assessment.go`
- Create: `backend/internal/fin/handler_payment_void.go`
- Modify: `backend/internal/fin/routes.go`
- Modify: `backend/internal/fin/handler_assessment_test.go`

- [ ] **Step 1: Update DeleteAssessment handler to pass voidedBy**

In `backend/internal/fin/handler_assessment.go`, update the `DeleteAssessment` handler to extract the user ID from context and pass it to `VoidAssessment`:

```go
// DeleteAssessment handles DELETE /organizations/{org_id}/assessments/{assessment_id}.
func (h *AssessmentHandler) DeleteAssessment(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	assessmentID, err := parsePathUUID(r, "assessment_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if err := h.service.VoidAssessment(r.Context(), assessmentID, userID); err != nil {
		h.logger.Error("VoidAssessment failed", "assessment_id", assessmentID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

Note: This requires importing `middleware`. Check the existing imports in handler_assessment.go and add if needed:
```go
"github.com/quorant/quorant/internal/platform/middleware"
```

- [ ] **Step 2: Create VoidPayment handler**

Create `backend/internal/fin/handler_payment_void.go`:

```go
package fin

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// VoidPayment handles POST /organizations/{org_id}/payments/{payment_id}/void.
func (h *PaymentHandler) VoidPayment(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	paymentID, err := parsePathUUID(r, "payment_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if err := h.service.VoidPayment(r.Context(), paymentID, userID); err != nil {
		h.logger.Error("VoidPayment failed", "payment_id", paymentID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Add VoidPayment route**

In `backend/internal/fin/routes.go`, add a new route for void payment. Find the existing payment routes section and add:

```go
mux.Handle("POST /api/v1/organizations/{org_id}/payments/{payment_id}/void", permMw("fin.payment.void", paymentHandler.VoidPayment))
```

- [ ] **Step 4: Update handler tests**

In `backend/internal/fin/handler_assessment_test.go`, the existing `TestDeleteAssessment_Success` test creates an assessment and deletes it. It needs updating because `VoidAssessment` now checks status and creates reversal entries. Update the mock to set `Status = AssessmentStatusPosted` on seed assessments.

Also add the `VoidAssessment` method to any mock `Service` implementation used in handler tests (if the handler uses the `Service` interface rather than concrete `FinService`).

- [ ] **Step 5: Verify compilation and tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/fin/handler_assessment.go backend/internal/fin/handler_payment_void.go backend/internal/fin/routes.go backend/internal/fin/handler_assessment_test.go
git commit -m "feat(fin): add VoidPayment handler and update DeleteAssessment to use VoidAssessment (issue #75)"
```

---

### Task 10: Update Assessment Scan Functions and CreateAssessment

**Files:**
- Modify: `backend/internal/fin/assessment_postgres.go`
- Modify: `backend/internal/fin/service.go`

- [ ] **Step 1: Update scanAssessment to include new fields**

Find `scanAssessment` in `assessment_postgres.go` and add scanning for `status`, `voided_by`, `voided_at` columns. Update the corresponding SELECT and RETURNING clauses in all assessment queries (`CreateAssessment`, `FindAssessmentByID`, `ListAssessmentsByOrg`, `ListAssessmentsByUnit`, `UpdateAssessment`).

- [ ] **Step 2: Set status=posted in CreateAssessment**

In `service.go`, in the `CreateAssessment` method, set the `Status` field on the new assessment:

```go
a := &Assessment{
	// ... existing fields ...
	Status: AssessmentStatusPosted,
}
```

- [ ] **Step 3: Update scanPayment for new fields**

Find `scanPayment` in `payment_postgres.go` and add scanning for `voided_by`, `voided_at` columns. Update corresponding SELECT/RETURNING clauses.

- [ ] **Step 4: Verify compilation and tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/assessment_postgres.go backend/internal/fin/payment_postgres.go backend/internal/fin/service.go
git commit -m "feat(fin): update scan functions and CreateAssessment for void fields (issue #75)"
```

---

### Task 11: Integration Tests

**Files:**
- Modify: `backend/internal/fin/service_integration_test.go` (or create if needed)

- [ ] **Step 1: Write TestVoidAssessment_FullFlow integration test**

```go
//go:build integration

func TestVoidAssessment_FullFlow(t *testing.T) {
	// Setup: IntegrationDB, real repos, real GL service
	// 1. Create assessment → verify ledger charge + GL entry posted
	// 2. Call VoidAssessment
	// 3. Verify: reversing ledger entry exists with negative amount
	// 4. Verify: unit balance is back to zero
	// 5. Verify: assessment status = void, voided_by set
	// 6. Verify: GL has reversal entry with is_reversal=true
	// 7. Verify: original GL entry has reversed_by set
}
```

- [ ] **Step 2: Write TestVoidPayment_FullFlow integration test**

```go
func TestVoidPayment_FullFlow(t *testing.T) {
	// Setup: IntegrationDB, real repos, real GL service
	// 1. Create assessment → record payment → verify balance = 0
	// 2. Call VoidPayment
	// 3. Verify: balance restored to original assessment amount
	// 4. Verify: payment status = void
	// 5. Verify: GL has reversal entry
}
```

- [ ] **Step 3: Write TestVoidAssessment_BlockedByPayment integration test**

```go
func TestVoidAssessment_BlockedByPayment(t *testing.T) {
	// 1. Create assessment → record payment
	// 2. Attempt VoidAssessment → expect error containing "has_payments"
	// 3. VoidPayment first → succeeds
	// 4. VoidAssessment → succeeds
	// 5. Verify final balance = 0
}
```

- [ ] **Step 4: Run integration tests**

Run: `cd backend && go test ./internal/fin/... -run "TestVoid.*_FullFlow|TestVoidAssessment_BlockedByPayment" -count=1 -tags=integration`
Expected: PASS (requires Docker services running)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/service_integration_test.go
git commit -m "test(fin): add integration tests for void/reversal system (issue #75)"
```

---

### Task 12: Final Verification and Lint

- [ ] **Step 1: Run all unit tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: No new warnings/errors

- [ ] **Step 3: Run build**

Run: `make build`
Expected: Both binaries compile successfully

- [ ] **Step 4: Commit any lint fixes if needed**

```bash
git commit -m "chore(fin): lint fixes for void/reversal system (issue #75)"
```
