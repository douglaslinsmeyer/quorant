-- Migration: 20260409000015_license_billing.sql
-- Description: Create license and billing tables: plans, entitlements, subscriptions,
--              entitlement overrides, usage metering, billing accounts, invoices, invoice line items

-- ============================================================
-- LICENSE: plans, entitlements, usage metering
-- ============================================================

CREATE TABLE plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    description     TEXT,
    plan_type       plan_type NOT NULL,
    price_cents     BIGINT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE entitlements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id         UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    feature_key     TEXT NOT NULL,
    limit_type      limit_type NOT NULL,
    limit_value     BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (plan_id, feature_key)
);

CREATE TABLE org_subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    plan_id         UUID NOT NULL REFERENCES plans(id),
    status          subscription_status NOT NULL DEFAULT 'active',
    starts_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at         TIMESTAMPTZ,
    trial_ends_at   TIMESTAMPTZ,
    stripe_sub_id   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_org_subscriptions_active
    ON org_subscriptions (org_id)
    WHERE status IN ('active', 'trial');

CREATE TABLE org_entitlement_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    feature_key     TEXT NOT NULL,
    limit_value     BIGINT,
    reason          TEXT,
    granted_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ,
    UNIQUE (org_id, feature_key)
);

CREATE TABLE usage_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    feature_key     TEXT NOT NULL,
    quantity        BIGINT NOT NULL,
    period_start    TIMESTAMPTZ NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_usage_records_org_period ON usage_records (org_id, feature_key, period_start);

-- ============================================================
-- BILLING: SaaS subscription billing (Stripe)
-- ============================================================

CREATE TABLE billing_accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) UNIQUE,
    stripe_customer_id TEXT UNIQUE,
    billing_email   TEXT NOT NULL,
    billing_name    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE invoices (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    billing_account_id  UUID NOT NULL REFERENCES billing_accounts(id),
    org_id              UUID NOT NULL REFERENCES organizations(id),
    stripe_invoice_id   TEXT UNIQUE,
    status              invoice_status NOT NULL DEFAULT 'draft',
    subtotal_cents      BIGINT NOT NULL DEFAULT 0,
    tax_cents           BIGINT NOT NULL DEFAULT 0,
    total_cents         BIGINT NOT NULL DEFAULT 0,
    period_start        TIMESTAMPTZ NOT NULL,
    period_end          TIMESTAMPTZ NOT NULL,
    due_date            DATE,
    paid_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoices_org ON invoices (org_id, created_at DESC);
CREATE INDEX idx_invoices_status ON invoices (status) WHERE status IN ('issued', 'overdue');

CREATE TABLE invoice_line_items (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id       UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description      TEXT NOT NULL,
    quantity         INTEGER NOT NULL DEFAULT 1,
    unit_price_cents BIGINT NOT NULL,
    total_cents      BIGINT NOT NULL,
    line_type        TEXT NOT NULL,
    metadata         JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoice_line_items_invoice ON invoice_line_items (invoice_id);
