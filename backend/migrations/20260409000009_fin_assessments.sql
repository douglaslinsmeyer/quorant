-- Migration: 20260409000009_fin_assessments.sql
-- Description: Create core financial tables: assessment schedules, assessments, ledger entries, payment methods, and payments

-- assessment_schedules: recurring assessment templates
CREATE TABLE assessment_schedules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,
    description     TEXT,
    frequency       TEXT NOT NULL,
    amount_strategy TEXT NOT NULL,
    base_amount_cents BIGINT NOT NULL,
    amount_rules    JSONB NOT NULL DEFAULT '{}',
    day_of_month    SMALLINT NOT NULL DEFAULT 1,
    grace_days      SMALLINT NOT NULL DEFAULT 15,
    starts_at       DATE NOT NULL,
    ends_at         DATE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    approved_by     UUID REFERENCES users(id),
    approved_at     TIMESTAMPTZ,
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_assessment_schedules_org ON assessment_schedules (org_id)
    WHERE deleted_at IS NULL AND is_active = TRUE;

-- assessments: individual charges per unit
CREATE TABLE assessments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    schedule_id     UUID REFERENCES assessment_schedules(id),
    description     TEXT NOT NULL,
    amount_cents    BIGINT NOT NULL,
    due_date        DATE NOT NULL,
    grace_days      SMALLINT NOT NULL DEFAULT 0,
    late_fee_cents  BIGINT NOT NULL DEFAULT 0,
    is_recurring    BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_assessments_org ON assessments (org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_assessments_unit ON assessments (unit_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_assessments_due ON assessments (due_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_assessments_schedule ON assessments (schedule_id) WHERE deleted_at IS NULL;

-- ledger_entries: per-unit financial ledger (homeowner-facing)
CREATE TABLE ledger_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    assessment_id   UUID REFERENCES assessments(id),
    entry_type      ledger_entry_type NOT NULL,
    amount_cents    BIGINT NOT NULL,
    balance_cents   BIGINT NOT NULL,
    description     TEXT,
    reference_type  TEXT,
    reference_id    UUID,
    effective_date  DATE NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_org ON ledger_entries (org_id, effective_date DESC);
CREATE INDEX idx_ledger_unit ON ledger_entries (unit_id, effective_date DESC);

-- payment_methods
CREATE TABLE payment_methods (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    method_type     TEXT NOT NULL,
    provider_ref    TEXT,
    last_four       TEXT,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_payment_methods_user ON payment_methods (user_id) WHERE deleted_at IS NULL;

-- payments
CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    payment_method_id UUID REFERENCES payment_methods(id),
    amount_cents    BIGINT NOT NULL,
    status          payment_status NOT NULL DEFAULT 'pending',
    provider_ref    TEXT,
    description     TEXT,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_org ON payments (org_id, created_at DESC);
CREATE INDEX idx_payments_unit ON payments (unit_id, created_at DESC);
CREATE INDEX idx_payments_status ON payments (status) WHERE status = 'pending';
