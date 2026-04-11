CREATE TABLE org_accounting_configs (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                   UUID NOT NULL REFERENCES organizations(id),
    standard                 TEXT NOT NULL DEFAULT 'gaap' CHECK (standard IN ('gaap', 'ifrs')),
    recognition_basis        TEXT NOT NULL DEFAULT 'accrual' CHECK (recognition_basis IN ('cash', 'accrual', 'modified_accrual')),
    fiscal_year_start        INTEGER NOT NULL DEFAULT 1 CHECK (fiscal_year_start BETWEEN 1 AND 12),
    availability_period_days INTEGER NOT NULL DEFAULT 60,
    effective_date           DATE NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by               UUID NOT NULL REFERENCES users(id)
);

CREATE INDEX idx_org_accounting_configs_org_effective ON org_accounting_configs (org_id, effective_date DESC);
