-- Migration: 20260409000017_ai_tables.sql
-- AI context lake tables: governing documents, context chunks (pgvector),
-- policy extractions, and policy resolutions.

-- ---------------------------------------------------------------------------
-- governing_documents: registry of governing docs that drive policy
-- ---------------------------------------------------------------------------
CREATE TABLE governing_documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    document_id     UUID NOT NULL REFERENCES documents(id),
    doc_type        TEXT NOT NULL,
    title           TEXT NOT NULL,
    effective_date  DATE NOT NULL,
    supersedes_id   UUID REFERENCES governing_documents(id),
    indexing_status  indexing_status NOT NULL DEFAULT 'pending',
    indexed_at      TIMESTAMPTZ,
    chunk_count     INTEGER,
    extraction_count INTEGER,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_governing_docs_org ON governing_documents (org_id, doc_type);

-- ---------------------------------------------------------------------------
-- context_chunks: the context lake (pgvector embeddings)
-- ---------------------------------------------------------------------------
CREATE TABLE context_chunks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope           context_scope NOT NULL DEFAULT 'org',
    org_id          UUID REFERENCES organizations(id),
    jurisdiction    TEXT,
    source_type     context_source_type NOT NULL,
    source_id       UUID NOT NULL,
    chunk_index     INTEGER NOT NULL,
    content         TEXT NOT NULL,
    section_ref     TEXT,
    page_number     INTEGER,
    embedding       vector(1536) NOT NULL,
    token_count     INTEGER NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_context_scope_global CHECK (
        scope != 'global' OR (org_id IS NULL AND jurisdiction IS NULL)),
    CONSTRAINT chk_context_scope_jurisdiction CHECK (
        scope != 'jurisdiction' OR (org_id IS NULL AND jurisdiction IS NOT NULL)),
    CONSTRAINT chk_context_scope_firm CHECK (
        scope != 'firm' OR (org_id IS NOT NULL AND jurisdiction IS NULL)),
    CONSTRAINT chk_context_scope_org CHECK (
        scope != 'org' OR (org_id IS NOT NULL AND jurisdiction IS NULL))
);

CREATE INDEX idx_context_chunks_org ON context_chunks (org_id, source_type) WHERE scope = 'org';
CREATE INDEX idx_context_chunks_jurisdiction ON context_chunks (jurisdiction) WHERE scope = 'jurisdiction';
CREATE INDEX idx_context_chunks_firm ON context_chunks (org_id) WHERE scope = 'firm';
CREATE INDEX idx_context_chunks_global ON context_chunks (source_type) WHERE scope = 'global';
CREATE INDEX idx_context_chunks_source ON context_chunks (source_id);
-- NOTE: IVFFlat index requires data to be useful; succeeds on empty table but
-- optimal only once rows are present.
CREATE INDEX idx_context_chunks_embedding ON context_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX idx_context_chunks_metadata ON context_chunks USING GIN (metadata);

-- ---------------------------------------------------------------------------
-- policy_extractions: AI-extracted structured policies from governing docs
-- ---------------------------------------------------------------------------
CREATE TABLE policy_extractions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    domain          policy_domain NOT NULL,
    policy_key      TEXT NOT NULL,
    config          JSONB NOT NULL,
    confidence      NUMERIC(3,2) NOT NULL,
    source_doc_id   UUID NOT NULL REFERENCES governing_documents(id),
    source_text     TEXT NOT NULL,
    source_section  TEXT,
    source_page     INTEGER,
    review_status   extraction_review_status NOT NULL DEFAULT 'pending',
    reviewed_by     UUID REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ,
    human_override  JSONB,
    model_version   TEXT NOT NULL,
    effective_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    superseded_by   UUID REFERENCES policy_extractions(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_policy_extractions_active
    ON policy_extractions (org_id, policy_key)
    WHERE superseded_by IS NULL AND review_status IN ('approved', 'pending');
CREATE INDEX idx_policy_extractions_review
    ON policy_extractions (review_status, org_id)
    WHERE review_status = 'pending';
CREATE INDEX idx_policy_extractions_history
    ON policy_extractions (org_id, policy_key, effective_at DESC);

-- ---------------------------------------------------------------------------
-- policy_resolutions: runtime AI inference log
-- ---------------------------------------------------------------------------
CREATE TABLE policy_resolutions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    query           TEXT NOT NULL,
    policy_keys     TEXT[] NOT NULL,
    resolution      JSONB NOT NULL,
    reasoning       TEXT NOT NULL,
    source_passages JSONB NOT NULL,
    confidence      NUMERIC(3,2) NOT NULL,
    resolution_type TEXT NOT NULL,
    model_version   TEXT,
    latency_ms      INTEGER,
    requesting_module TEXT NOT NULL,
    requesting_context JSONB NOT NULL DEFAULT '{}',
    human_decision  JSONB,
    decided_by      UUID REFERENCES users(id),
    decided_at      TIMESTAMPTZ,
    fed_back        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_policy_resolutions_org ON policy_resolutions (org_id, created_at DESC);
CREATE INDEX idx_policy_resolutions_pending
    ON policy_resolutions (org_id, resolution_type)
    WHERE resolution_type = 'human_escalated' AND decided_at IS NULL;
