-- 20260409000030_payment_allocations.sql
-- Adds charge_type enum, payment_allocations table, and new payment_status values.

CREATE TYPE charge_type AS ENUM (
    'regular_assessment', 'special_assessment',
    'late_fee', 'interest', 'collection_cost', 'attorney_fee', 'fine'
);

ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'pending_review';
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'reversed';
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'nsf';

CREATE TABLE payment_allocations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id      UUID NOT NULL REFERENCES payments(id),
    charge_type     charge_type NOT NULL,
    charge_id       UUID NOT NULL,
    allocated_cents BIGINT NOT NULL CHECK (allocated_cents > 0),
    resolution_id   UUID NOT NULL REFERENCES policy_resolutions(id),
    estoppel_id     UUID,
    reversed_at     TIMESTAMPTZ,
    reversed_by_id  UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_allocations_payment ON payment_allocations (payment_id);
CREATE INDEX idx_payment_allocations_charge ON payment_allocations (charge_id);
CREATE INDEX idx_payment_allocations_estoppel ON payment_allocations (estoppel_id) WHERE estoppel_id IS NOT NULL;
