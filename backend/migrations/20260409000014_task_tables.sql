-- Migration: 20260409000014_task_tables.sql
-- Description: Create task module tables: task_types, tasks, task_comments, task_status_history

-- ---------------------------------------------------------------------------
-- task_types: configurable task type definitions
-- ---------------------------------------------------------------------------
CREATE TABLE task_types (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID REFERENCES organizations(id),  -- NULL = system-defined
    key                 TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    default_priority    task_priority NOT NULL DEFAULT 'normal',
    sla_hours           INTEGER,
    workflow_stages     JSONB NOT NULL DEFAULT '[]',
    checklist_template  JSONB NOT NULL DEFAULT '[]',
    auto_assign_role    TEXT,
    source_module       TEXT NOT NULL,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_task_types_key
    ON task_types (COALESCE(org_id, '00000000-0000-0000-0000-000000000000'), key);

-- ---------------------------------------------------------------------------
-- tasks
-- ---------------------------------------------------------------------------
CREATE TABLE tasks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id),
    task_type_id        UUID NOT NULL REFERENCES task_types(id),
    title               TEXT NOT NULL,
    description         TEXT,
    status              task_status NOT NULL DEFAULT 'open',
    priority            task_priority NOT NULL DEFAULT 'normal',
    current_stage       TEXT,
    resource_type       TEXT NOT NULL,
    resource_id         UUID NOT NULL,
    unit_id             UUID REFERENCES units(id),
    assigned_to         UUID REFERENCES users(id),
    assigned_role       TEXT,
    assigned_at         TIMESTAMPTZ,
    assigned_by         UUID REFERENCES users(id),
    due_at              TIMESTAMPTZ,
    sla_deadline        TIMESTAMPTZ,
    sla_breached        BOOLEAN NOT NULL DEFAULT FALSE,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    checklist           JSONB NOT NULL DEFAULT '[]',
    parent_task_id      UUID REFERENCES tasks(id),
    blocked_by_task_id  UUID REFERENCES tasks(id),
    created_by          UUID NOT NULL REFERENCES users(id),
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tasks_assigned ON tasks (assigned_to, status, priority)
    WHERE status NOT IN ('completed', 'cancelled');
CREATE INDEX idx_tasks_role_queue ON tasks (org_id, assigned_role, priority)
    WHERE assigned_to IS NULL AND status = 'open';
CREATE INDEX idx_tasks_org ON tasks (org_id, status, priority)
    WHERE status NOT IN ('completed', 'cancelled');
CREATE INDEX idx_tasks_resource ON tasks (resource_type, resource_id);
CREATE INDEX idx_tasks_sla ON tasks (sla_deadline)
    WHERE sla_breached = FALSE AND status NOT IN ('completed', 'cancelled');
CREATE INDEX idx_tasks_due ON tasks (due_at)
    WHERE status NOT IN ('completed', 'cancelled');
CREATE INDEX idx_tasks_parent ON tasks (parent_task_id)
    WHERE parent_task_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- task_comments
-- ---------------------------------------------------------------------------
CREATE TABLE task_comments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    author_id       UUID NOT NULL REFERENCES users(id),
    body            TEXT NOT NULL,
    attachment_ids  UUID[] DEFAULT '{}',
    is_internal     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_task_comments_task ON task_comments (task_id, created_at);

-- ---------------------------------------------------------------------------
-- task_status_history (immutable)
-- ---------------------------------------------------------------------------
CREATE TABLE task_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_status task_status,
    to_status   task_status NOT NULL,
    from_stage  TEXT,
    to_stage    TEXT,
    changed_by  UUID NOT NULL REFERENCES users(id),
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_task_status_history_task ON task_status_history (task_id, created_at);
