-- Migration: 20260409000025_gl_permissions_seed.sql
-- Description: Seed GL-specific permissions and role-permission mappings.

-- ============================================================
-- GL Permissions
-- ============================================================
INSERT INTO permissions (key, description, module) VALUES
    ('fin.gl.account.manage',  'Create, update, and delete GL accounts',              'fin'),
    ('fin.gl.account.read',    'View chart of accounts',                              'fin'),
    ('fin.gl.journal.create',  'Post manual journal entries',                          'fin'),
    ('fin.gl.journal.read',    'View journal entries',                                 'fin'),
    ('fin.gl.report.read',     'View trial balance and account balance reports',       'fin');

-- ============================================================
-- Role -> Permission mappings for GL permissions
-- ============================================================

-- platform_admin: already gets ALL permissions via CROSS JOIN in seed_roles,
-- but these permissions are added after that migration, so grant explicitly.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'platform_admin'
AND p.key IN (
    'fin.gl.account.manage',
    'fin.gl.account.read',
    'fin.gl.journal.create',
    'fin.gl.journal.read',
    'fin.gl.report.read'
);

-- platform_support: read-only access
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'platform_support'
AND p.key IN (
    'fin.gl.account.read',
    'fin.gl.journal.read',
    'fin.gl.report.read'
);

-- firm_admin: full GL access
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'firm_admin'
AND p.key IN (
    'fin.gl.account.manage',
    'fin.gl.account.read',
    'fin.gl.journal.create',
    'fin.gl.journal.read',
    'fin.gl.report.read'
);

-- firm_staff: read-only GL access
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'firm_staff'
AND p.key IN (
    'fin.gl.account.read',
    'fin.gl.journal.read',
    'fin.gl.report.read'
);

-- hoa_manager: full GL access
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'hoa_manager'
AND p.key IN (
    'fin.gl.account.manage',
    'fin.gl.account.read',
    'fin.gl.journal.create',
    'fin.gl.journal.read',
    'fin.gl.report.read'
);

-- board_president: read-only GL access
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'board_president'
AND p.key IN (
    'fin.gl.account.read',
    'fin.gl.journal.read',
    'fin.gl.report.read'
);

-- board_member: account and report read only (no journal read)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'board_member'
AND p.key IN (
    'fin.gl.account.read',
    'fin.gl.report.read'
);

-- homeowner: no GL permissions (intentionally omitted)
