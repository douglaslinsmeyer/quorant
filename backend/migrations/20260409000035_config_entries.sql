-- Dedicated configuration storage table.
-- Supports hierarchical resolution: platform -> firm -> org.
-- Replaces piggy-backing on organizations.settings JSONB.

CREATE TABLE config_entries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope       TEXT NOT NULL,           -- 'platform', 'firm', 'org'
    scope_id    UUID,                    -- NULL for platform, org_id for firm/org
    key         TEXT NOT NULL,
    value       JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    CONSTRAINT chk_scope_valid CHECK (scope IN ('platform', 'firm', 'org')),
    CONSTRAINT chk_platform_no_scope_id CHECK (scope != 'platform' OR scope_id IS NULL),
    CONSTRAINT chk_nonplatform_has_scope_id CHECK (scope = 'platform' OR scope_id IS NOT NULL)
);

-- Unique: one value per scope+scope_id+key
CREATE UNIQUE INDEX idx_config_entries_lookup
    ON config_entries (scope, COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'), key);

-- Fast lookups by scope_id+key (for org and firm scope)
CREATE INDEX idx_config_entries_scope_id ON config_entries (scope_id, key)
    WHERE scope_id IS NOT NULL;
