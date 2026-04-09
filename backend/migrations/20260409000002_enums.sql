-- Migration: 20260409000002_enums.sql
-- Description: Create all PostgreSQL enum types for the Quorant platform

-- Organization / Tenancy
CREATE TYPE org_type AS ENUM ('firm', 'hoa');
CREATE TYPE membership_status AS ENUM ('active', 'inactive', 'invited');
CREATE TYPE subscription_status AS ENUM ('active', 'trial', 'suspended', 'cancelled');
CREATE TYPE plan_type AS ENUM ('firm', 'hoa', 'firm_bundle');
CREATE TYPE limit_type AS ENUM ('boolean', 'numeric', 'rate');

-- Billing / Invoicing
CREATE TYPE invoice_status AS ENUM ('draft', 'issued', 'paid', 'overdue', 'void');
CREATE TYPE payment_status AS ENUM ('pending', 'completed', 'failed', 'refunded');

-- Compliance / Violations / ARB
CREATE TYPE violation_status AS ENUM ('open', 'acknowledged', 'resolved', 'escalated', 'closed');
CREATE TYPE arb_status AS ENUM ('submitted', 'under_review', 'revision_requested', 'approved', 'conditionally_approved', 'denied', 'withdrawn');

-- Governance
CREATE TYPE ballot_status AS ENUM ('draft', 'open', 'closed', 'cancelled');
CREATE TYPE meeting_status AS ENUM ('scheduled', 'in_progress', 'completed', 'cancelled');

-- Financial
CREATE TYPE ledger_entry_type AS ENUM ('charge', 'payment', 'credit', 'adjustment', 'late_fee');
CREATE TYPE budget_status AS ENUM ('draft', 'proposed', 'approved', 'amended', 'closed');
CREATE TYPE expense_status AS ENUM ('draft', 'submitted', 'approved', 'paid', 'rejected', 'void');
CREATE TYPE collection_status AS ENUM ('current', 'late', 'delinquent', 'demand_sent', 'attorney_referred', 'lien_filed', 'foreclosure', 'payment_plan', 'settled', 'written_off');

-- Communications / Notifications
CREATE TYPE notification_channel AS ENUM ('push', 'email', 'sms');
CREATE TYPE comm_direction AS ENUM ('outbound', 'inbound');
CREATE TYPE comm_channel AS ENUM ('email', 'sms', 'push', 'physical_mail', 'phone', 'in_person', 'platform', 'certified_mail');
CREATE TYPE comm_status AS ENUM ('draft', 'queued', 'sent', 'delivered', 'opened', 'bounced', 'failed', 'returned_to_sender');

-- Units / Residents
CREATE TYPE unit_relationship AS ENUM ('owner', 'tenant', 'resident', 'emergency_contact');
CREATE TYPE reservation_status AS ENUM ('pending', 'confirmed', 'cancelled', 'completed', 'no_show');

-- Policies / Documents
CREATE TYPE policy_domain AS ENUM ('financial', 'governance', 'compliance', 'use_restrictions', 'operational');
CREATE TYPE extraction_review_status AS ENUM ('pending', 'approved', 'rejected', 'modified');
CREATE TYPE indexing_status AS ENUM ('pending', 'processing', 'indexed', 'failed');

-- AI / Context
CREATE TYPE context_source_type AS ENUM (
    'governing_document', 'document', 'announcement', 'thread_message',
    'meeting_minutes', 'meeting_motion', 'task_narrative', 'violation_narrative',
    'collection_narrative', 'arb_narrative', 'budget_summary', 'event_summary'
);
CREATE TYPE context_scope AS ENUM ('global', 'jurisdiction', 'firm', 'org');

-- Tasks
CREATE TYPE task_status AS ENUM ('open', 'assigned', 'in_progress', 'blocked', 'review', 'completed', 'cancelled');
CREATE TYPE task_priority AS ENUM ('low', 'normal', 'high', 'urgent');
