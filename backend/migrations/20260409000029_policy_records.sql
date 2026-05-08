-- 20260409000029_policy_records.sql
-- Policy records store jurisdiction rules, org overrides, and unit-level policies.
-- Policy resolutions store the Tier 2 AI ruling audit trail.

CREATE TABLE policy_records (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope             TEXT NOT NULL CHECK (scope IN ('jurisdiction', 'org', 'unit')),
    jurisdiction      TEXT,
    org_id            UUID REFERENCES organizations(id),
    unit_id           UUID REFERENCES units(id),
    category          TEXT NOT NULL,
    key               TEXT NOT NULL,
    value             JSONB NOT NULL,
    priority_hint     TEXT NOT NULL CHECK (priority_hint IN (
                          'federal', 'state', 'local', 'cc_r', 'board_policy'
                      )),
    statute_reference TEXT,
    source_doc_id     UUID,
    effective_date    DATE NOT NULL DEFAULT CURRENT_DATE,
    expiration_date   DATE,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_jurisdiction_scope CHECK (
        scope != 'jurisdiction' OR jurisdiction IS NOT NULL
    ),
    CONSTRAINT chk_org_scope CHECK (
        scope NOT IN ('org', 'unit') OR org_id IS NOT NULL
    ),
    CONSTRAINT chk_unit_scope CHECK (
        scope != 'unit' OR unit_id IS NOT NULL
    )
);

-- Lookup index by category and active flag. The original predicate
-- `expiration_date > CURRENT_DATE` can't live in an index (CURRENT_DATE is
-- STABLE, not IMMUTABLE), so the time check is applied at query time and the
-- partial index covers the common case of records with no expiration set.
CREATE INDEX idx_policy_records_lookup
    ON policy_records (category, is_active)
    WHERE expiration_date IS NULL;

CREATE INDEX idx_policy_records_unit
    ON policy_records (unit_id, category)
    WHERE unit_id IS NOT NULL AND is_active = true;

CREATE TABLE policy_resolutions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID NOT NULL REFERENCES organizations(id),
    unit_id               UUID REFERENCES units(id),
    category              TEXT NOT NULL,
    input_policy_ids      UUID[] NOT NULL,
    ruling                JSONB NOT NULL,
    reasoning             TEXT NOT NULL,
    confidence            DECIMAL(3,2) NOT NULL,
    model_id              TEXT NOT NULL,
    parent_resolution_id  UUID REFERENCES policy_resolutions(id),
    review_status         TEXT NOT NULL DEFAULT 'auto_approved'
                          CHECK (review_status IN (
                              'auto_approved', 'pending_review',
                              'confirmed', 'corrected', 'ai_unavailable'
                          )),
    review_sla_deadline   TIMESTAMPTZ,
    reviewed_by           UUID,
    review_notes          TEXT,
    reviewed_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_policy_resolutions_review
    ON policy_resolutions (review_status)
    WHERE review_status IN ('pending_review', 'ai_unavailable');

CREATE INDEX idx_policy_resolutions_sla
    ON policy_resolutions (review_sla_deadline)
    WHERE review_status IN ('pending_review', 'ai_unavailable')
      AND review_sla_deadline IS NOT NULL;

-- RLS on policy_resolutions (created here, after the table). The bulk RLS
-- sweep in migration 20 happens before this table exists, so RLS is enabled
-- alongside creation.
ALTER TABLE policy_resolutions ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_resolutions FORCE ROW LEVEL SECURITY;
CREATE POLICY policy_resolutions_tenant_isolation ON policy_resolutions
    USING (org_id = current_setting('app.current_org_id', true)::uuid);
