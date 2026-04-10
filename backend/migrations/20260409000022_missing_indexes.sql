-- Add missing indexes identified during architecture review

-- Ledger entries: unit ledger queries
CREATE INDEX IF NOT EXISTS idx_ledger_entries_unit_date
    ON ledger_entries (unit_id, effective_date DESC);

-- Tasks: "my tasks" queries by assignee
CREATE INDEX IF NOT EXISTS idx_tasks_assignee_status
    ON tasks (assigned_to, status)
    WHERE assigned_to IS NOT NULL AND status NOT IN ('completed', 'cancelled');
