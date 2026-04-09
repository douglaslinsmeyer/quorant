-- ============================================================
-- AUDIT: global audit log (append-only, immutable)
-- ============================================================

-- Note: In production this would be PARTITION BY RANGE (occurred_at)
-- with monthly partitions. For now we create a regular table.
-- Partitioning will be added when needed for scale.
CREATE TABLE audit_log (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id        UUID UNIQUE DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    actor_id        UUID NOT NULL,
    impersonator_id UUID,
    action          TEXT NOT NULL,
    resource_type   TEXT NOT NULL,
    resource_id     UUID NOT NULL,
    module          TEXT NOT NULL,
    before_state    JSONB,
    after_state     JSONB,
    metadata        JSONB DEFAULT '{}',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_org_time ON audit_log (org_id, occurred_at DESC);
CREATE INDEX idx_audit_log_actor ON audit_log (actor_id, occurred_at DESC);
CREATE INDEX idx_audit_log_resource ON audit_log (resource_type, resource_id, occurred_at DESC);
CREATE INDEX idx_audit_log_action ON audit_log (action, occurred_at DESC);

-- ============================================================
-- QUEUE: domain events, processed event tracking
-- ============================================================

CREATE TABLE domain_events (
    event_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type      TEXT NOT NULL,
    aggregate_type  TEXT NOT NULL,
    aggregate_id    UUID NOT NULL,
    org_id          UUID NOT NULL,
    payload         JSONB NOT NULL,
    metadata        JSONB DEFAULT '{}',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ
);

CREATE INDEX idx_domain_events_aggregate ON domain_events (aggregate_type, aggregate_id, occurred_at);
CREATE INDEX idx_domain_events_unpublished ON domain_events (occurred_at) WHERE published_at IS NULL;

CREATE TABLE processed_events (
    handler_name    TEXT NOT NULL,
    event_id        UUID NOT NULL REFERENCES domain_events(event_id),
    processed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (handler_name, event_id)
);
