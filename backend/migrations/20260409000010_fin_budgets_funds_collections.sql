-- Migration: 20260409000010_fin_budgets_funds_collections.sql
-- Description: Create financial tables: budget categories, budgets, line items,
--              expenses, funds, fund transactions/transfers, collection cases/actions,
--              and payment plans

-- budget_categories
CREATE TABLE budget_categories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,
    category_type   TEXT NOT NULL,
    parent_id       UUID REFERENCES budget_categories(id),
    sort_order      SMALLINT NOT NULL DEFAULT 0,
    is_reserve      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, name, category_type)
);

-- budgets
CREATE TABLE budgets (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID NOT NULL REFERENCES organizations(id),
    fiscal_year           SMALLINT NOT NULL,
    name                  TEXT NOT NULL,
    status                budget_status NOT NULL DEFAULT 'draft',
    total_income_cents    BIGINT NOT NULL DEFAULT 0,
    total_expense_cents   BIGINT NOT NULL DEFAULT 0,
    net_cents             BIGINT NOT NULL DEFAULT 0,
    notes                 TEXT,
    proposed_at           TIMESTAMPTZ,
    proposed_by           UUID REFERENCES users(id),
    approved_at           TIMESTAMPTZ,
    approved_by           UUID REFERENCES users(id),
    document_id           UUID,  -- FK to documents(id) will be added in Doc module phase
    created_by            UUID NOT NULL REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at            TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_budgets_org_year
    ON budgets (org_id, fiscal_year)
    WHERE deleted_at IS NULL AND status != 'amended';

-- budget_line_items
CREATE TABLE budget_line_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id       UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    category_id     UUID NOT NULL REFERENCES budget_categories(id),
    description     TEXT,
    planned_cents   BIGINT NOT NULL,
    actual_cents    BIGINT NOT NULL DEFAULT 0,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_budget_line_items_budget ON budget_line_items (budget_id);

-- expenses
CREATE TABLE expenses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    vendor_id       UUID REFERENCES vendors(id),
    category_id     UUID REFERENCES budget_categories(id),
    budget_id       UUID REFERENCES budgets(id),
    fund_type       TEXT NOT NULL DEFAULT 'operating',
    description     TEXT NOT NULL,
    amount_cents    BIGINT NOT NULL,
    tax_cents       BIGINT NOT NULL DEFAULT 0,
    total_cents     BIGINT NOT NULL,
    status          expense_status NOT NULL DEFAULT 'draft',
    expense_date    DATE NOT NULL,
    due_date        DATE,
    paid_date       DATE,
    payment_method  TEXT,
    payment_ref     TEXT,
    invoice_number  TEXT,
    receipt_doc_id  UUID,  -- FK to documents(id) will be added in Doc module phase
    submitted_by    UUID NOT NULL REFERENCES users(id),
    approved_by     UUID REFERENCES users(id),
    approved_at     TIMESTAMPTZ,
    approval_notes  TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_expenses_org ON expenses (org_id, expense_date DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_expenses_status ON expenses (org_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_expenses_vendor ON expenses (vendor_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_expenses_budget ON expenses (budget_id, category_id) WHERE deleted_at IS NULL;

-- funds
CREATE TABLE funds (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID NOT NULL REFERENCES organizations(id),
    name                 TEXT NOT NULL,
    fund_type            TEXT NOT NULL,
    balance_cents        BIGINT NOT NULL DEFAULT 0,
    target_balance_cents BIGINT,
    is_default           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at           TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_funds_default
    ON funds (org_id)
    WHERE is_default = TRUE AND deleted_at IS NULL;

CREATE INDEX idx_funds_org ON funds (org_id) WHERE deleted_at IS NULL;

-- fund_transactions
CREATE TABLE fund_transactions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fund_id             UUID NOT NULL REFERENCES funds(id),
    org_id              UUID NOT NULL REFERENCES organizations(id),
    transaction_type    TEXT NOT NULL,
    amount_cents        BIGINT NOT NULL,
    balance_after_cents BIGINT NOT NULL,
    description         TEXT,
    reference_type      TEXT,
    reference_id        UUID,
    effective_date      DATE NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fund_transactions_fund ON fund_transactions (fund_id, effective_date DESC);
CREATE INDEX idx_fund_transactions_org ON fund_transactions (org_id, effective_date DESC);

-- fund_transfers
CREATE TABLE fund_transfers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    from_fund_id    UUID NOT NULL REFERENCES funds(id),
    to_fund_id      UUID NOT NULL REFERENCES funds(id),
    amount_cents    BIGINT NOT NULL,
    description     TEXT,
    approved_by     UUID REFERENCES users(id),
    approved_at     TIMESTAMPTZ,
    effective_date  DATE NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fund_transfers_org ON fund_transfers (org_id, effective_date DESC);

-- collection_cases
CREATE TABLE collection_cases (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id),
    unit_id             UUID NOT NULL REFERENCES units(id),
    status              collection_status NOT NULL DEFAULT 'late',
    total_owed_cents    BIGINT NOT NULL,
    current_owed_cents  BIGINT NOT NULL,
    escalation_paused   BOOLEAN NOT NULL DEFAULT FALSE,
    pause_reason        TEXT,
    opened_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at           TIMESTAMPTZ,
    closed_reason       TEXT,
    assigned_to         UUID REFERENCES users(id),
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_collection_cases_active
    ON collection_cases (unit_id)
    WHERE closed_at IS NULL;

CREATE INDEX idx_collection_cases_org ON collection_cases (org_id, status) WHERE closed_at IS NULL;

-- collection_actions
CREATE TABLE collection_actions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID NOT NULL REFERENCES collection_cases(id),
    action_type     TEXT NOT NULL,
    notes           TEXT,
    document_id     UUID,  -- FK to documents(id) will be added in Doc module phase
    triggered_by    TEXT NOT NULL,
    performed_by    UUID REFERENCES users(id),
    scheduled_for   DATE,
    completed_at    TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_collection_actions_case ON collection_actions (case_id, created_at);

-- payment_plans
CREATE TABLE payment_plans (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES collection_cases(id),
    org_id              UUID NOT NULL REFERENCES organizations(id),
    unit_id             UUID NOT NULL REFERENCES units(id),
    total_owed_cents    BIGINT NOT NULL,
    installment_cents   BIGINT NOT NULL,
    frequency           TEXT NOT NULL DEFAULT 'monthly',
    installments_total  SMALLINT NOT NULL,
    installments_paid   SMALLINT NOT NULL DEFAULT 0,
    next_due_date       DATE NOT NULL,
    status              TEXT NOT NULL DEFAULT 'active',
    approved_by         UUID REFERENCES users(id),
    approved_at         TIMESTAMPTZ,
    defaulted_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_plans_case ON payment_plans (case_id);
CREATE INDEX idx_payment_plans_next_due ON payment_plans (next_due_date)
    WHERE status = 'active';
