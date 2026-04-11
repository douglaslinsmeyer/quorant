-- Add idempotency key to payments for duplicate submission protection.
ALTER TABLE payments ADD COLUMN idempotency_key TEXT;

-- Partial unique index: only enforce uniqueness on non-null keys.
-- This allows legacy rows and payments without idempotency keys to coexist.
CREATE UNIQUE INDEX idx_payments_idempotency_key
    ON payments (org_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
