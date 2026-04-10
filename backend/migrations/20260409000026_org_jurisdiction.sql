-- Add jurisdiction field to organizations (decoupled from address state).
ALTER TABLE organizations ADD COLUMN jurisdiction TEXT;
CREATE INDEX idx_organizations_jurisdiction ON organizations (jurisdiction) WHERE jurisdiction IS NOT NULL;

-- Backfill from existing state field.
UPDATE organizations SET jurisdiction = state WHERE state IS NOT NULL;

-- Seed system task type for compliance alerts.
INSERT INTO task_types (key, name, description, default_priority, source_module, auto_assign_role, is_active)
VALUES (
    'compliance_alert',
    'Compliance Alert',
    'Automated compliance status change notification',
    'high',
    'ai',
    'hoa_manager',
    TRUE
) ON CONFLICT DO NOTHING;
