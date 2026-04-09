-- jurisdiction_rules: platform-managed deterministic statutory parameters.
-- NOT tenant-scoped. No org_id. No RLS.
CREATE TABLE jurisdiction_rules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction      TEXT NOT NULL,
    rule_category     TEXT NOT NULL,
    rule_key          TEXT NOT NULL,
    value_type        TEXT NOT NULL,
    value             JSONB NOT NULL,
    statute_reference TEXT NOT NULL,
    effective_date    DATE NOT NULL,
    expiration_date   DATE,
    notes             TEXT,
    source_doc_id     UUID REFERENCES governing_documents(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        UUID REFERENCES users(id),
    UNIQUE (jurisdiction, rule_category, rule_key, effective_date)
);

CREATE INDEX idx_jurisdiction_rules_active
    ON jurisdiction_rules (jurisdiction, rule_category, rule_key)
    WHERE expiration_date IS NULL OR expiration_date > now();

CREATE INDEX idx_jurisdiction_rules_jurisdiction
    ON jurisdiction_rules (jurisdiction);

CREATE INDEX idx_jurisdiction_rules_upcoming
    ON jurisdiction_rules (effective_date)
    WHERE effective_date > now();

-- compliance_checks: tenant-scoped audit trail of compliance evaluations.
CREATE TABLE compliance_checks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           UUID NOT NULL REFERENCES organizations(id),
    rule_id          UUID NOT NULL REFERENCES jurisdiction_rules(id),
    status           TEXT NOT NULL,
    details          JSONB,
    checked_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at      TIMESTAMPTZ,
    resolution_notes TEXT
);

CREATE INDEX idx_compliance_checks_org ON compliance_checks (org_id, checked_at DESC);
CREATE INDEX idx_compliance_checks_rule ON compliance_checks (rule_id);
CREATE INDEX idx_compliance_checks_unresolved
    ON compliance_checks (org_id)
    WHERE resolved_at IS NULL AND status = 'non_compliant';

-- RLS for compliance_checks (tenant-scoped).
ALTER TABLE compliance_checks ENABLE ROW LEVEL SECURITY;
ALTER TABLE compliance_checks FORCE ROW LEVEL SECURITY;
CREATE POLICY compliance_checks_tenant_isolation ON compliance_checks
    USING (org_id = current_setting('app.current_org_id', true)::uuid);
