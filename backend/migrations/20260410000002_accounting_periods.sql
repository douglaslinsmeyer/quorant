CREATE TABLE accounting_periods (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id),
    fiscal_year   INTEGER NOT NULL,
    period_number INTEGER NOT NULL CHECK (period_number BETWEEN 1 AND 13),
    start_date    DATE NOT NULL,
    end_date      DATE NOT NULL,
    status        TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'soft_closed', 'closed')),
    closed_by     UUID REFERENCES users(id),
    closed_at     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, fiscal_year, period_number)
);

CREATE INDEX idx_accounting_periods_org_date ON accounting_periods (org_id, start_date, end_date);
