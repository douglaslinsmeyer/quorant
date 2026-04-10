-- Migration: 20260409000026_estoppel_tables.sql
-- Description: Create estoppel_requests and estoppel_certificates tables with
-- RLS tenant isolation, indexes, and deferred cross-table FK.

-- estoppel_requests: intake record for estoppel certificates and lender
-- questionnaires submitted by homeowners, title companies, or attorneys.
CREATE TABLE estoppel_requests (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                      UUID NOT NULL REFERENCES organizations(id),
    unit_id                     UUID NOT NULL REFERENCES units(id),
    task_id                     UUID REFERENCES tasks(id),
    request_type                TEXT NOT NULL
        CHECK (request_type IN ('estoppel_certificate', 'lender_questionnaire')),
    requestor_type              TEXT NOT NULL
        CHECK (requestor_type IN ('homeowner', 'title_company', 'closing_agent', 'attorney')),
    requestor_name              TEXT NOT NULL,
    requestor_email             TEXT NOT NULL,
    requestor_phone             TEXT,
    requestor_company           TEXT,
    property_address            TEXT NOT NULL,
    owner_name                  TEXT NOT NULL,
    closing_date                DATE,
    rush_requested              BOOLEAN NOT NULL DEFAULT false,
    status                      TEXT NOT NULL DEFAULT 'submitted',
    fee_cents                   INTEGER NOT NULL DEFAULT 0,
    rush_fee_cents              INTEGER NOT NULL DEFAULT 0,
    delinquent_surcharge_cents  INTEGER NOT NULL DEFAULT 0,
    total_fee_cents             INTEGER NOT NULL DEFAULT 0,
    deadline_at                 TIMESTAMPTZ NOT NULL,
    assigned_to                 UUID REFERENCES users(id),
    amendment_of                UUID,  -- FK to estoppel_certificates added below
    metadata                    JSONB DEFAULT '{}',
    created_by                  UUID NOT NULL REFERENCES users(id),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at                  TIMESTAMPTZ
);

-- estoppel_certificates: the generated output document produced for an
-- estoppel request.
CREATE TABLE estoppel_certificates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id          UUID NOT NULL REFERENCES estoppel_requests(id),
    org_id              UUID NOT NULL REFERENCES organizations(id),
    unit_id             UUID NOT NULL REFERENCES units(id),
    document_id         UUID NOT NULL REFERENCES documents(id),
    jurisdiction        TEXT NOT NULL,
    effective_date      DATE NOT NULL,
    expires_at          DATE,
    data_snapshot       JSONB NOT NULL,
    narrative_sections  JSONB NOT NULL DEFAULT '{}',
    signed_by           UUID NOT NULL REFERENCES users(id),
    signed_at           TIMESTAMPTZ NOT NULL,
    signer_title        TEXT NOT NULL,
    template_version    TEXT NOT NULL,
    amendment_of        UUID REFERENCES estoppel_certificates(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Add the deferred cross-table FK now that estoppel_certificates exists.
ALTER TABLE estoppel_requests
    ADD CONSTRAINT fk_estoppel_requests_amendment_of
    FOREIGN KEY (amendment_of) REFERENCES estoppel_certificates(id);

-- ─── Indexes ──────────────────────────────────────────────────────────────────

CREATE INDEX idx_estoppel_requests_org
    ON estoppel_requests (org_id, status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_estoppel_requests_unit
    ON estoppel_requests (unit_id, created_at DESC);

CREATE INDEX idx_estoppel_requests_deadline
    ON estoppel_requests (deadline_at)
    WHERE status NOT IN ('delivered', 'cancelled');

CREATE INDEX idx_estoppel_certificates_org
    ON estoppel_certificates (org_id);

CREATE INDEX idx_estoppel_certificates_unit
    ON estoppel_certificates (unit_id, effective_date DESC);

CREATE INDEX idx_estoppel_certificates_request
    ON estoppel_certificates (request_id);

-- ─── Row Level Security ───────────────────────────────────────────────────────

ALTER TABLE estoppel_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE estoppel_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY estoppel_requests_tenant_isolation ON estoppel_requests
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE estoppel_certificates ENABLE ROW LEVEL SECURITY;
ALTER TABLE estoppel_certificates FORCE ROW LEVEL SECURITY;
CREATE POLICY estoppel_certificates_tenant_isolation ON estoppel_certificates
    USING (org_id = current_setting('app.current_org_id', true)::uuid);
