# Void/Reversal System for Payments and Journal Entries

**Issue:** #75 (also closes #67)
**Date:** 2026-04-10
**Status:** Draft

## Problem

GL has `reversed_by`/`is_reversal` fields but no service logic. Payments cannot be voided or refunded. Assessment soft-delete (`deleted_at`) leaves the ledger charge and GL journal entry intact, causing permanently inflated unit balances and GL divergence.

Under GAAP, posted financial transactions must never be deleted — they must be reversed with proper adjusting entries that maintain a complete audit trail.

## Scope

Four capabilities:

1. **ReverseJournalEntry** — GL layer foundation, transaction-agnostic
2. **ReverseLedgerEntry** — Ledger layer foundation, transaction-agnostic
3. **VoidAssessment** — First consumer; thin wrapper with assessment business rules
4. **VoidPayment** — Second consumer; thin wrapper with payment business rules

The reversal mechanics (1 and 2) are generic so future void operations (fees, late fees, etc.) only need domain-specific business rule validation before calling the shared reversal functions.

## Design

### ReverseJournalEntry (GL Layer)

`GLService.ReverseJournalEntry(ctx, entryID, reversedBy uuid.UUID) (*GLJournalEntry, error)`

1. Find original entry by ID (with lines)
2. Validate `reversed_by` is nil (not already reversed)
3. Create new journal entry:
   - Swap debits/credits on every line (debit becomes credit, vice versa)
   - `is_reversal = true`
   - Same `source_type`, `source_id`, `unit_id` as original
   - `memo = "Reversal: <original memo>"`
   - `entry_date = today` (GAAP: reversal falls in current period)
4. Update original entry's `reversed_by` to the reversal entry's ID

Creates a bidirectional audit trail between original and reversal entries.

**New repository methods:**

- `FindJournalEntriesBySource(ctx context.Context, sourceType GLSourceType, sourceID uuid.UUID) ([]GLJournalEntry, error)` — uses existing `idx_gl_journal_entries_source` index
- `UpdateJournalEntryReversedBy(ctx context.Context, entryID, reversalID uuid.UUID) error`

### ReverseLedgerEntry (Ledger Layer)

`FinService.ReverseLedgerEntry(ctx, entryID, reversedBy uuid.UUID) (*LedgerEntry, error)`

1. Fetch original ledger entry by ID
2. Validate `reversed_by_entry_id` is nil (not already reversed)
3. Create new `LedgerEntry`:
   - `entry_type = LedgerEntryTypeReversal` (new enum value)
   - `amount_cents = -original.AmountCents`
   - `unit_id`, `org_id`, `assessment_id` copied from original
   - `reference_type = LedgerRefTypeReversal`, `reference_id = original.ID`
   - `effective_date = today`
   - `description = "Reversal: <original description>"`
4. Update original entry's `reversed_by_entry_id` to the reversal entry's ID

**New repository methods:**

- `FindLedgerEntryByAssessment(ctx context.Context, assessmentID uuid.UUID) (*LedgerEntry, error)`
- `FindLedgerEntryByPaymentRef(ctx context.Context, paymentID uuid.UUID) (*LedgerEntry, error)`
- `UpdateLedgerEntryReversedBy(ctx context.Context, entryID, reversalEntryID uuid.UUID) error`

### VoidAssessment

`FinService.VoidAssessment(ctx, assessmentID, voidedBy uuid.UUID) error`

Replaces the current `DeleteAssessment` soft-delete behavior.

1. Fetch assessment — 404 if not found
2. Validate `status = posted` — error if already `void`
3. Check for payments — query ledger entries for this assessment where `entry_type = payment`. If any exist, return validation error `"fin.assessment.has_payments"`. Payments must be voided first (GAAP: cannot reverse revenue while related cash receipts are still on the books).
4. Find ledger entry by `assessment_id` + `entry_type = charge`
5. Call `ReverseLedgerEntry`
6. Find GL entry by `source_type = assessment`, `source_id = assessmentID`
7. Call `ReverseJournalEntry`
8. Update assessment: `status = void`, `voided_by = voidedBy`, `voided_at = now()`

Edge case: If GL service is nil, skip GL reversal (same pattern `CreateAssessment` uses).

The existing `DELETE /assessments/{id}` endpoint is rewired to call `VoidAssessment`. HTTP contract stays the same (204 No Content).

### VoidPayment

`FinService.VoidPayment(ctx, paymentID, voidedBy uuid.UUID) error`

1. Fetch payment — 404 if not found
2. Validate `status = completed` — error if already `void` or `failed`
3. Find ledger entry by `reference_type = payment`, `reference_id = paymentID`
4. Call `ReverseLedgerEntry` — creates a positive amount entry (original payment was negative), restoring the unit's balance
5. Find GL entry by `source_type = payment`, `source_id = paymentID`
6. Call `ReverseJournalEntry` — reverses the Cash debit / AR credit
7. Update payment: `status = void`, `voided_by = voidedBy`, `voided_at = now()`

GAAP note: Voiding a payment restores the receivable. If the homeowner paid $500 on a $500 assessment and the payment is voided, the unit balance returns to $500 owed.

## Schema Changes

### assessments table

```sql
ALTER TABLE assessments
    ADD COLUMN status TEXT NOT NULL DEFAULT 'posted',
    ADD COLUMN voided_by UUID REFERENCES users(id),
    ADD COLUMN voided_at TIMESTAMPTZ;

-- Backfill: rows with deleted_at set are treated as void
UPDATE assessments SET status = 'void' WHERE deleted_at IS NOT NULL;
```

Update partial indexes to filter on `status = 'posted'` in addition to `deleted_at IS NULL`.

### payments table

```sql
ALTER TYPE payment_status ADD VALUE 'void';

ALTER TABLE payments
    ADD COLUMN voided_by UUID REFERENCES users(id),
    ADD COLUMN voided_at TIMESTAMPTZ;
```

### ledger_entries table

```sql
ALTER TABLE ledger_entries
    ADD COLUMN reversed_by_entry_id UUID REFERENCES ledger_entries(id);
```

Add `reversal` to the DB enum:

```sql
ALTER TYPE ledger_entry_type ADD VALUE 'reversal';
```

Note: `reference_type` is `TEXT` (not a DB enum), so no migration needed for `LedgerRefTypeReversal`.

### New Go Enum Values

- `LedgerEntryTypeReversal LedgerEntryType = "reversal"`
- `LedgerRefTypeReversal LedgerReferenceType = "reversal"`
- `PaymentStatusVoid PaymentStatus = "void"`
- New `AssessmentStatus` type: `AssessmentStatusPosted = "posted"`, `AssessmentStatusVoid = "void"`

### New Repository Methods

| Repository | Method | Purpose |
|------------|--------|---------|
| Assessment | `FindLedgerEntryByAssessment(ctx, assessmentID)` | Find charge entry for an assessment |
| Assessment | `FindLedgerEntryByPaymentRef(ctx, paymentID)` | Find payment entry by reference |
| Assessment | `UpdateLedgerEntryReversedBy(ctx, entryID, reversalID)` | Link original to reversal |
| Assessment | `UpdateAssessmentStatus(ctx, id, status, voidedBy, voidedAt)` | Set void status |
| GL | `FindJournalEntriesBySource(ctx, sourceType, sourceID)` | Find GL entries by source |
| GL | `UpdateJournalEntryReversedBy(ctx, entryID, reversalID)` | Link original to reversal |
| Payment | `UpdatePaymentVoid(ctx, id, voidedBy, voidedAt)` | Set void status on payment |

## Testing

### Unit Tests (Service Layer)

- `TestReverseLedgerEntry` — happy path, already-reversed error, entry-not-found
- `TestReverseJournalEntry` — happy path, already-reversed error, debit/credit swap verification, bidirectional linking
- `TestVoidAssessment` — happy path (ledger + GL reversed, status updated), block when payments exist, already-void error, GL-nil graceful skip
- `TestVoidPayment` — happy path (ledger + GL reversed, status updated), already-void error, balance restoration verification

### Unit Tests (Repository Layer)

- `TestFindLedgerEntryByAssessment`, `TestFindLedgerEntryByPaymentRef`
- `TestUpdateLedgerEntryReversedBy`, `TestUpdateAssessmentStatus`
- `TestFindJournalEntriesBySource`, `TestUpdateJournalEntryReversedBy`

### Integration Tests

- `TestVoidAssessment_FullFlow` — create assessment, verify ledger charge + GL entry, void, verify reversing ledger entry + reversing GL entry + unit balance back to zero + assessment status = void
- `TestVoidPayment_FullFlow` — create assessment, record payment, void payment, verify balance restored + GL reversed + payment status = void
- `TestVoidAssessment_BlockedByPayment` — create assessment, record payment, attempt void (expect validation error), void payment first, void assessment succeeds

Uses existing test helpers: `InMemoryPublisher`, `NoopAuditor`, stub repos for unit tests; `IntegrationDB` for integration tests.

## Out of Scope

- **Unified charge entity** — generalizing assessments, fees, late fees under a single domain type. Tracked as a separate issue.
- **Transaction coordinator** (issue #60) — wrapping multi-step operations in a single DB transaction. The void operations follow the same non-atomic pattern as `CreateAssessment` currently uses.
- **Partial voids** — voiding a portion of an assessment or payment. Full void only.
- **Refund workflow** — voiding a payment just reverses the books; actual refund disbursement is a separate concern.
