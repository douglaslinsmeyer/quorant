-- Quorant Development Seed Data
-- Run after all migrations: make seed

BEGIN;

-- ============================================================
-- Users (one per role — idp_user_id simulates Zitadel IDs)
-- ============================================================

INSERT INTO users (id, idp_user_id, email, display_name, is_active) VALUES
    ('00000000-0000-0000-0000-000000000001', 'zitadel-admin-001', 'admin@quorant.io', 'Platform Admin', TRUE),
    ('00000000-0000-0000-0000-000000000002', 'zitadel-support-001', 'support@quorant.io', 'Platform Support', TRUE),
    ('00000000-0000-0000-0000-000000000003', 'zitadel-finance-001', 'finance@quorant.io', 'Platform Finance', TRUE),
    ('00000000-0000-0000-0000-000000000010', 'zitadel-firm-admin-001', 'frank@acmeprops.com', 'Frank Adams (Firm Admin)', TRUE),
    ('00000000-0000-0000-0000-000000000011', 'zitadel-firm-staff-001', 'sarah@acmeprops.com', 'Sarah Chen (Firm Staff)', TRUE),
    ('00000000-0000-0000-0000-000000000012', 'zitadel-firm-support-001', 'mike@acmeprops.com', 'Mike Torres (Firm Support)', TRUE),
    ('00000000-0000-0000-0000-000000000020', 'zitadel-hoa-manager-001', 'lisa@lakewood.org', 'Lisa Park (HOA Manager)', TRUE),
    ('00000000-0000-0000-0000-000000000021', 'zitadel-vendor-001', 'vendor@greenscapes.com', 'Green Landscapes Contact', TRUE),
    ('00000000-0000-0000-0000-000000000030', 'zitadel-president-001', 'bob@lakewood.org', 'Bob Martinez (Board President)', TRUE),
    ('00000000-0000-0000-0000-000000000031', 'zitadel-board-001', 'carol@lakewood.org', 'Carol White (Board Member)', TRUE),
    ('00000000-0000-0000-0000-000000000040', 'zitadel-homeowner-001', 'alice@example.com', 'Alice Johnson (Homeowner)', TRUE),
    ('00000000-0000-0000-0000-000000000041', 'zitadel-homeowner-002', 'dave@example.com', 'Dave Wilson (Homeowner)', TRUE);

-- ============================================================
-- Organizations: 1 firm + 2 HOAs
-- ============================================================

-- Management firm
INSERT INTO organizations (id, type, name, slug, path, city, state, email, phone) VALUES
    ('10000000-0000-0000-0000-000000000001', 'firm', 'Acme Property Management', 'acme-property-mgmt', 'acme_property_mgmt', 'Phoenix', 'AZ', 'info@acmeprops.com', '602-555-0100');

-- HOA 1: managed by Acme
INSERT INTO organizations (id, type, name, slug, path, city, state, email, phone) VALUES
    ('20000000-0000-0000-0000-000000000001', 'hoa', 'Lakewood Master HOA', 'lakewood-master-hoa', 'lakewood_master_hoa', 'Phoenix', 'AZ', 'board@lakewood.org', '602-555-0200');

-- HOA 2: self-managed
INSERT INTO organizations (id, type, name, slug, path, city, state, email, phone) VALUES
    ('20000000-0000-0000-0000-000000000002', 'hoa', 'Sunset Villas HOA', 'sunset-villas-hoa', 'sunset_villas_hoa', 'Scottsdale', 'AZ', 'board@sunsetvillas.org', '480-555-0300');

-- Link firm to HOA 1
INSERT INTO organizations_management (firm_org_id, hoa_org_id) VALUES
    ('10000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001');

-- ============================================================
-- Memberships (assign users to orgs with roles)
-- ============================================================

-- Platform roles (use a dummy org — platform_admin is global)
INSERT INTO memberships (user_id, org_id, role_id, status, joined_at) VALUES
    ('00000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'platform_admin'), 'active', now()),
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'platform_support'), 'active', now()),
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'platform_finance'), 'active', now());

-- Firm roles
INSERT INTO memberships (user_id, org_id, role_id, status, joined_at) VALUES
    ('00000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'firm_admin'), 'active', now()),
    ('00000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'firm_staff'), 'active', now()),
    ('00000000-0000-0000-0000-000000000012', '10000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'firm_support'), 'active', now());

-- HOA 1 roles (Lakewood — managed by Acme)
INSERT INTO memberships (user_id, org_id, role_id, status, joined_at) VALUES
    ('00000000-0000-0000-0000-000000000020', '20000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'hoa_manager'), 'active', now()),
    ('00000000-0000-0000-0000-000000000021', '20000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'vendor_contact'), 'active', now()),
    ('00000000-0000-0000-0000-000000000030', '20000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'board_president'), 'active', now()),
    ('00000000-0000-0000-0000-000000000031', '20000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'board_member'), 'active', now()),
    ('00000000-0000-0000-0000-000000000040', '20000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'homeowner'), 'active', now()),
    ('00000000-0000-0000-0000-000000000041', '20000000-0000-0000-0000-000000000001', (SELECT id FROM roles WHERE name = 'homeowner'), 'active', now());

-- HOA 2 roles (Sunset Villas — self-managed, board president also hoa_manager)
INSERT INTO memberships (user_id, org_id, role_id, status, joined_at) VALUES
    ('00000000-0000-0000-0000-000000000030', '20000000-0000-0000-0000-000000000002', (SELECT id FROM roles WHERE name = 'board_president'), 'active', now()),
    ('00000000-0000-0000-0000-000000000030', '20000000-0000-0000-0000-000000000002', (SELECT id FROM roles WHERE name = 'hoa_manager'), 'active', now());

-- ============================================================
-- Units (5 per HOA)
-- ============================================================

INSERT INTO units (id, org_id, label, unit_type, status, voting_weight) VALUES
    ('30000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'Unit 101', 'condo', 'occupied', 1.00),
    ('30000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001', 'Unit 102', 'condo', 'occupied', 1.00),
    ('30000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000001', 'Unit 103', 'condo', 'occupied', 1.00),
    ('30000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000001', 'Unit 201', 'condo', 'occupied', 1.00),
    ('30000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000001', 'Unit 202', 'condo', 'vacant', 1.00),
    ('30000000-0000-0000-0000-000000000006', '20000000-0000-0000-0000-000000000002', 'Lot 1', 'single_family', 'occupied', 1.00),
    ('30000000-0000-0000-0000-000000000007', '20000000-0000-0000-0000-000000000002', 'Lot 2', 'single_family', 'occupied', 1.00),
    ('30000000-0000-0000-0000-000000000008', '20000000-0000-0000-0000-000000000002', 'Lot 3', 'single_family', 'occupied', 1.50),
    ('30000000-0000-0000-0000-000000000009', '20000000-0000-0000-0000-000000000002', 'Lot 4', 'single_family', 'occupied', 1.00),
    ('30000000-0000-0000-0000-000000000010', '20000000-0000-0000-0000-000000000002', 'Lot 5', 'single_family', 'vacant', 1.00);

-- Unit memberships (homeowners)
INSERT INTO unit_memberships (unit_id, user_id, relationship, is_voter) VALUES
    ('30000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000040', 'owner', TRUE),
    ('30000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000041', 'owner', TRUE);

-- ============================================================
-- Funds (operating + reserve per HOA)
-- ============================================================

INSERT INTO funds (org_id, name, fund_type, balance_cents, is_default) VALUES
    ('20000000-0000-0000-0000-000000000001', 'Operating Fund', 'operating', 5000000, TRUE),
    ('20000000-0000-0000-0000-000000000001', 'Reserve Fund', 'reserve', 15000000, FALSE),
    ('20000000-0000-0000-0000-000000000002', 'Operating Fund', 'operating', 3000000, TRUE),
    ('20000000-0000-0000-0000-000000000002', 'Reserve Fund', 'reserve', 8000000, FALSE);

-- ============================================================
-- Vendor
-- ============================================================

INSERT INTO vendors (id, name, contact_email, contact_phone, service_types) VALUES
    ('40000000-0000-0000-0000-000000000001', 'Green Landscapes LLC', 'billing@greenscapes.com', '602-555-9000', '{landscaping,irrigation}');

INSERT INTO vendor_assignments (vendor_id, org_id, service_scope) VALUES
    ('40000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'landscaping');

-- ============================================================
-- Sample amenity
-- ============================================================

INSERT INTO amenities (org_id, name, amenity_type, description, is_reservable, capacity, status) VALUES
    ('20000000-0000-0000-0000-000000000001', 'Community Pool', 'pool', 'Heated pool open May-September', TRUE, 50, 'open'),
    ('20000000-0000-0000-0000-000000000001', 'Clubhouse', 'clubhouse', 'Available for private events', TRUE, 100, 'open');

-- ============================================================
-- License plans
-- ============================================================

INSERT INTO plans (id, name, description, plan_type, price_cents, is_active) VALUES
    ('50000000-0000-0000-0000-000000000001', 'HOA Starter', 'Basic HOA management', 'hoa', 4900, TRUE),
    ('50000000-0000-0000-0000-000000000002', 'HOA Professional', 'Full HOA management with AI', 'hoa', 14900, TRUE),
    ('50000000-0000-0000-0000-000000000003', 'Firm Bundle', 'Management firm with bundled HOA coverage', 'firm_bundle', 49900, TRUE);

INSERT INTO entitlements (plan_id, feature_key, limit_type, limit_value) VALUES
    ('50000000-0000-0000-0000-000000000001', 'units.max', 'numeric', 50),
    ('50000000-0000-0000-0000-000000000001', 'violations.tracking', 'boolean', NULL),
    ('50000000-0000-0000-0000-000000000002', 'units.max', 'numeric', 500),
    ('50000000-0000-0000-0000-000000000002', 'violations.tracking', 'boolean', NULL),
    ('50000000-0000-0000-0000-000000000002', 'ai.context_lake', 'boolean', NULL),
    ('50000000-0000-0000-0000-000000000003', 'units.max', 'numeric', 5000),
    ('50000000-0000-0000-0000-000000000003', 'violations.tracking', 'boolean', NULL),
    ('50000000-0000-0000-0000-000000000003', 'ai.context_lake', 'boolean', NULL);

-- Subscriptions
INSERT INTO org_subscriptions (org_id, plan_id, status) VALUES
    ('10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000003', 'active'),
    ('20000000-0000-0000-0000-000000000002', '50000000-0000-0000-0000-000000000001', 'active');
-- HOA 1 (Lakewood) is covered by Acme's firm_bundle plan

COMMIT;
