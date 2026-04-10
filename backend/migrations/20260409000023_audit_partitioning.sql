-- Convert audit_log to a partitioned table.
-- This is a forward-looking migration — it creates monthly partitions
-- for the current year. A scheduled worker should create future partitions.

-- Note: PostgreSQL cannot convert an existing table to partitioned in-place.
-- Strategy: rename old table, create new partitioned table, copy data.

-- Step 1: Rename existing table
ALTER TABLE audit_log RENAME TO audit_log_old;

-- Step 2: Drop indexes on old table (will be recreated on partitioned table)
DROP INDEX IF EXISTS idx_audit_log_org_time;
DROP INDEX IF EXISTS idx_audit_log_actor;
DROP INDEX IF EXISTS idx_audit_log_resource;
DROP INDEX IF EXISTS idx_audit_log_action;

-- Step 3: Create partitioned table
CREATE TABLE audit_log (
    id              BIGINT GENERATED ALWAYS AS IDENTITY,
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
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- Step 4: Create monthly partitions for 2026
CREATE TABLE audit_log_y2026m01 PARTITION OF audit_log FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE audit_log_y2026m02 PARTITION OF audit_log FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE audit_log_y2026m03 PARTITION OF audit_log FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE audit_log_y2026m04 PARTITION OF audit_log FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE audit_log_y2026m05 PARTITION OF audit_log FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE audit_log_y2026m06 PARTITION OF audit_log FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE audit_log_y2026m07 PARTITION OF audit_log FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE audit_log_y2026m08 PARTITION OF audit_log FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE audit_log_y2026m09 PARTITION OF audit_log FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE audit_log_y2026m10 PARTITION OF audit_log FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE audit_log_y2026m11 PARTITION OF audit_log FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE audit_log_y2026m12 PARTITION OF audit_log FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

-- Step 5: Create default partition for data outside the defined ranges
CREATE TABLE audit_log_default PARTITION OF audit_log DEFAULT;

-- Step 6: Copy existing data
INSERT INTO audit_log (event_id, org_id, actor_id, impersonator_id, action, resource_type, resource_id, module, before_state, after_state, metadata, occurred_at)
SELECT event_id, org_id, actor_id, impersonator_id, action, resource_type, resource_id, module, before_state, after_state, metadata, occurred_at
FROM audit_log_old;

-- Step 7: Recreate indexes on partitioned table
CREATE INDEX idx_audit_log_org_time ON audit_log (org_id, occurred_at DESC);
CREATE INDEX idx_audit_log_actor ON audit_log (actor_id, occurred_at DESC);
CREATE INDEX idx_audit_log_resource ON audit_log (resource_type, resource_id, occurred_at DESC);
CREATE INDEX idx_audit_log_action ON audit_log (action, occurred_at DESC);

-- Step 8: Drop old table
DROP TABLE audit_log_old;
