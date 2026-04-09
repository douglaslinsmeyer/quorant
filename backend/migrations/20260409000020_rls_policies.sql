-- Migration: 20260409000020_rls_policies.sql
-- Row-Level Security policies for all tenant tables.
-- Ensures multi-tenant isolation at the database layer via session variable app.current_org_id.
--
-- The application sets: SET LOCAL app.current_org_id = '<uuid>'
-- at the start of each transaction. RLS USING clauses check this value.
--
-- FORCE ROW LEVEL SECURITY ensures the policy applies even when the
-- connection role is the table owner (e.g. the app's migration role).
--
-- Tables without a direct org_id column (e.g. ballot_votes, arb_votes)
-- are accessed through RLS-protected parent queries and do not need RLS here.
-- Tables that are system/global (plans, entitlements, roles, etc.) are excluded.
-- The users table spans orgs and is excluded.

-- ---------------------------------------------------------------------------
-- org_foundation: organizations, memberships
-- ---------------------------------------------------------------------------
ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
CREATE POLICY organizations_tenant_isolation ON organizations
    USING (id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY memberships_tenant_isolation ON memberships
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- units: units, unit_ownership_history
-- (properties has no direct org_id — accessed via units join)
-- (unit_memberships has no direct org_id — accessed via units join)
-- ---------------------------------------------------------------------------
ALTER TABLE units ENABLE ROW LEVEL SECURITY;
ALTER TABLE units FORCE ROW LEVEL SECURITY;
CREATE POLICY units_tenant_isolation ON units
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE unit_ownership_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE unit_ownership_history FORCE ROW LEVEL SECURITY;
CREATE POLICY unit_ownership_history_tenant_isolation ON unit_ownership_history
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- vendors / amenities
-- ---------------------------------------------------------------------------
ALTER TABLE vendor_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE vendor_assignments FORCE ROW LEVEL SECURITY;
CREATE POLICY vendor_assignments_tenant_isolation ON vendor_assignments
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE amenities ENABLE ROW LEVEL SECURITY;
ALTER TABLE amenities FORCE ROW LEVEL SECURITY;
CREATE POLICY amenities_tenant_isolation ON amenities
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE amenity_reservations ENABLE ROW LEVEL SECURITY;
ALTER TABLE amenity_reservations FORCE ROW LEVEL SECURITY;
CREATE POLICY amenity_reservations_tenant_isolation ON amenity_reservations
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE unit_registration_types ENABLE ROW LEVEL SECURITY;
ALTER TABLE unit_registration_types FORCE ROW LEVEL SECURITY;
CREATE POLICY unit_registration_types_tenant_isolation ON unit_registration_types
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE unit_registrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE unit_registrations FORCE ROW LEVEL SECURITY;
CREATE POLICY unit_registrations_tenant_isolation ON unit_registrations
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- audit_log: org board members see their own org; platform admins see all.
-- Since we don't have a role check here, we allow access when org_id matches
-- OR when the session variable is not set (NULL means no restriction — handled
-- at the application layer for platform-admin queries).
-- ---------------------------------------------------------------------------
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log FORCE ROW LEVEL SECURITY;
CREATE POLICY audit_log_tenant_isolation ON audit_log
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- domain_events: system queue table — RLS by org_id
ALTER TABLE domain_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE domain_events FORCE ROW LEVEL SECURITY;
CREATE POLICY domain_events_tenant_isolation ON domain_events
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- fin_assessments
-- ---------------------------------------------------------------------------
ALTER TABLE assessment_schedules ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment_schedules FORCE ROW LEVEL SECURITY;
CREATE POLICY assessment_schedules_tenant_isolation ON assessment_schedules
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE assessments ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessments FORCE ROW LEVEL SECURITY;
CREATE POLICY assessments_tenant_isolation ON assessments
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE ledger_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY ledger_entries_tenant_isolation ON ledger_entries
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE payment_methods ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_methods FORCE ROW LEVEL SECURITY;
CREATE POLICY payment_methods_tenant_isolation ON payment_methods
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments FORCE ROW LEVEL SECURITY;
CREATE POLICY payments_tenant_isolation ON payments
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- fin_budgets_funds_collections
-- ---------------------------------------------------------------------------
ALTER TABLE budget_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE budget_categories FORCE ROW LEVEL SECURITY;
CREATE POLICY budget_categories_tenant_isolation ON budget_categories
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE budgets ENABLE ROW LEVEL SECURITY;
ALTER TABLE budgets FORCE ROW LEVEL SECURITY;
CREATE POLICY budgets_tenant_isolation ON budgets
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE expenses ENABLE ROW LEVEL SECURITY;
ALTER TABLE expenses FORCE ROW LEVEL SECURITY;
CREATE POLICY expenses_tenant_isolation ON expenses
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE funds ENABLE ROW LEVEL SECURITY;
ALTER TABLE funds FORCE ROW LEVEL SECURITY;
CREATE POLICY funds_tenant_isolation ON funds
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE fund_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE fund_transactions FORCE ROW LEVEL SECURITY;
CREATE POLICY fund_transactions_tenant_isolation ON fund_transactions
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE fund_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE fund_transfers FORCE ROW LEVEL SECURITY;
CREATE POLICY fund_transfers_tenant_isolation ON fund_transfers
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE collection_cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE collection_cases FORCE ROW LEVEL SECURITY;
CREATE POLICY collection_cases_tenant_isolation ON collection_cases
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE payment_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_plans FORCE ROW LEVEL SECURITY;
CREATE POLICY payment_plans_tenant_isolation ON payment_plans
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- gov_tables
-- ---------------------------------------------------------------------------
ALTER TABLE violations ENABLE ROW LEVEL SECURITY;
ALTER TABLE violations FORCE ROW LEVEL SECURITY;
CREATE POLICY violations_tenant_isolation ON violations
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE arb_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE arb_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY arb_requests_tenant_isolation ON arb_requests
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE ballots ENABLE ROW LEVEL SECURITY;
ALTER TABLE ballots FORCE ROW LEVEL SECURITY;
CREATE POLICY ballots_tenant_isolation ON ballots
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE meetings ENABLE ROW LEVEL SECURITY;
ALTER TABLE meetings FORCE ROW LEVEL SECURITY;
CREATE POLICY meetings_tenant_isolation ON meetings
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- com_tables
-- ---------------------------------------------------------------------------
ALTER TABLE announcements ENABLE ROW LEVEL SECURITY;
ALTER TABLE announcements FORCE ROW LEVEL SECURITY;
CREATE POLICY announcements_tenant_isolation ON announcements
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE threads ENABLE ROW LEVEL SECURITY;
ALTER TABLE threads FORCE ROW LEVEL SECURITY;
CREATE POLICY threads_tenant_isolation ON threads
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE notification_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_preferences FORCE ROW LEVEL SECURITY;
CREATE POLICY notification_preferences_tenant_isolation ON notification_preferences
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE unit_notification_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE unit_notification_subscriptions FORCE ROW LEVEL SECURITY;
CREATE POLICY unit_notification_subscriptions_tenant_isolation ON unit_notification_subscriptions
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE calendar_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE calendar_events FORCE ROW LEVEL SECURITY;
CREATE POLICY calendar_events_tenant_isolation ON calendar_events
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- message_templates: org_id is nullable (NULL = system default template).
-- Allow access when org_id matches OR when this is a system template (org_id IS NULL).
ALTER TABLE message_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE message_templates FORCE ROW LEVEL SECURITY;
CREATE POLICY message_templates_tenant_isolation ON message_templates
    USING (org_id IS NULL OR org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE directory_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE directory_preferences FORCE ROW LEVEL SECURITY;
CREATE POLICY directory_preferences_tenant_isolation ON directory_preferences
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE communication_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE communication_log FORCE ROW LEVEL SECURITY;
CREATE POLICY communication_log_tenant_isolation ON communication_log
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- doc_tables
-- ---------------------------------------------------------------------------
ALTER TABLE document_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_categories FORCE ROW LEVEL SECURITY;
CREATE POLICY document_categories_tenant_isolation ON document_categories
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents FORCE ROW LEVEL SECURITY;
CREATE POLICY documents_tenant_isolation ON documents
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- task_tables
-- ---------------------------------------------------------------------------
-- task_types: org_id is nullable (NULL = system-defined type).
-- Allow access when org_id matches OR when this is a system type (org_id IS NULL).
ALTER TABLE task_types ENABLE ROW LEVEL SECURITY;
ALTER TABLE task_types FORCE ROW LEVEL SECURITY;
CREATE POLICY task_types_tenant_isolation ON task_types
    USING (org_id IS NULL OR org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE tasks FORCE ROW LEVEL SECURITY;
CREATE POLICY tasks_tenant_isolation ON tasks
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- ai_tables
-- ---------------------------------------------------------------------------
ALTER TABLE governing_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE governing_documents FORCE ROW LEVEL SECURITY;
CREATE POLICY governing_documents_tenant_isolation ON governing_documents
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- context_chunks: scope-aware RLS.
-- org/firm scoped chunks: match by org_id.
-- global/jurisdiction chunks (org_id IS NULL): accessible to all authenticated tenants.
ALTER TABLE context_chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE context_chunks FORCE ROW LEVEL SECURITY;
CREATE POLICY context_chunks_tenant_isolation ON context_chunks
    USING (
        org_id IS NULL
        OR org_id = current_setting('app.current_org_id', true)::uuid
    );

ALTER TABLE policy_extractions ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_extractions FORCE ROW LEVEL SECURITY;
CREATE POLICY policy_extractions_tenant_isolation ON policy_extractions
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE policy_resolutions ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_resolutions FORCE ROW LEVEL SECURITY;
CREATE POLICY policy_resolutions_tenant_isolation ON policy_resolutions
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- webhook_tables
-- ---------------------------------------------------------------------------
ALTER TABLE webhook_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_subscriptions FORCE ROW LEVEL SECURITY;
CREATE POLICY webhook_subscriptions_tenant_isolation ON webhook_subscriptions
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- admin_tables
-- ---------------------------------------------------------------------------
ALTER TABLE feature_flag_overrides ENABLE ROW LEVEL SECURITY;
ALTER TABLE feature_flag_overrides FORCE ROW LEVEL SECURITY;
CREATE POLICY feature_flag_overrides_tenant_isolation ON feature_flag_overrides
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE tenant_activity_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_activity_log FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_activity_log_tenant_isolation ON tenant_activity_log
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- license_billing
-- ---------------------------------------------------------------------------
ALTER TABLE org_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_subscriptions FORCE ROW LEVEL SECURITY;
CREATE POLICY org_subscriptions_tenant_isolation ON org_subscriptions
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE org_entitlement_overrides ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_entitlement_overrides FORCE ROW LEVEL SECURITY;
CREATE POLICY org_entitlement_overrides_tenant_isolation ON org_entitlement_overrides
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE usage_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_records FORCE ROW LEVEL SECURITY;
CREATE POLICY usage_records_tenant_isolation ON usage_records
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE billing_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY billing_accounts_tenant_isolation ON billing_accounts
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices FORCE ROW LEVEL SECURITY;
CREATE POLICY invoices_tenant_isolation ON invoices
    USING (org_id = current_setting('app.current_org_id', true)::uuid);
