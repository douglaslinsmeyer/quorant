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
