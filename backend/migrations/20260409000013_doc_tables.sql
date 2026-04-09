-- Migration: 20260409000013_doc_tables.sql
-- Document categories and documents tables with versioning support.
-- Also adds deferred FK constraints from earlier phases that reference documents(id).

-- ---------------------------------------------------------------------------
-- document_categories
-- ---------------------------------------------------------------------------
CREATE TABLE document_categories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,
    parent_id       UUID REFERENCES document_categories(id),
    sort_order      SMALLINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_doc_categories_org ON document_categories (org_id);

-- ---------------------------------------------------------------------------
-- documents (with versioning support)
-- ---------------------------------------------------------------------------
CREATE TABLE documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    category_id     UUID REFERENCES document_categories(id),
    uploaded_by     UUID NOT NULL REFERENCES users(id),
    title           TEXT NOT NULL,
    file_name       TEXT NOT NULL,
    content_type    TEXT NOT NULL,
    size_bytes      BIGINT NOT NULL,
    storage_key     TEXT NOT NULL,
    visibility      TEXT NOT NULL DEFAULT 'members',
    version_number  SMALLINT NOT NULL DEFAULT 1,
    parent_doc_id   UUID REFERENCES documents(id),
    is_current      BOOLEAN NOT NULL DEFAULT TRUE,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_documents_org ON documents (org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_category ON documents (category_id) WHERE deleted_at IS NULL;

-- Ensure only one current version per document chain
CREATE UNIQUE INDEX idx_documents_current_version
    ON documents (COALESCE(parent_doc_id, id))
    WHERE is_current = TRUE AND deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Deferred FK constraints from earlier phases
-- These columns were created as plain UUIDs; now we can add the FK references.
-- ---------------------------------------------------------------------------
ALTER TABLE budgets ADD CONSTRAINT fk_budgets_document
    FOREIGN KEY (document_id) REFERENCES documents(id);

ALTER TABLE expenses ADD CONSTRAINT fk_expenses_receipt_doc
    FOREIGN KEY (receipt_doc_id) REFERENCES documents(id);

ALTER TABLE collection_actions ADD CONSTRAINT fk_collection_actions_document
    FOREIGN KEY (document_id) REFERENCES documents(id);

ALTER TABLE proxy_authorizations ADD CONSTRAINT fk_proxy_auth_document
    FOREIGN KEY (document_id) REFERENCES documents(id);
