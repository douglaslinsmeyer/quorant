-- Jurisdiction rules seed data for FL, CA, TX, AZ, CO.
-- Source: current state statutes as of 2025.

-- Clear existing seed data (idempotent)
DELETE FROM jurisdiction_rules WHERE created_by IS NULL;

-- FLORIDA
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes) VALUES
('FL', 'meeting_notice', 'board_meeting_notice_days', 'integer', '2', 'FS 720.303(2)(c)', '2024-01-01', '48 hours notice required'),
('FL', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '14', 'FS 720.306(5)', '2024-01-01', '14 days mailed or posted notice'),
('FL', 'meeting_notice', 'special_meeting_notice_days', 'integer', '7', 'FS 720.306(5)', '2024-01-01', NULL),
('FL', 'meeting_notice', 'emergency_meeting_notice_days', 'integer', '0', 'FS 720.303(2)(c)', '2024-01-01', 'Reasonable notice; no minimum for emergencies'),
('FL', 'fine_limits', 'hearing_required', 'boolean', 'true', 'FS 720.305(2)', '2024-01-01', 'Committee hearing required before fines'),
('FL', 'fine_limits', 'daily_aggregate_cap_cents', 'integer', '10000', 'FS 720.305(2)(b)', '2024-01-01', '$100/day aggregate cap'),
('FL', 'fine_limits', 'per_violation_cap_cents', 'text', '"none"', 'FS 720.305(2)', '2024-01-01', 'No statutory per-violation cap for HOAs'),
('FL', 'reserve_study', 'sirs_required', 'boolean', 'true', 'SB 4-D (2022)', '2025-01-01', 'Structural Integrity Reserve Study required'),
('FL', 'reserve_study', 'sirs_interval_years', 'integer', '10', 'SB 4-D (2022)', '2025-01-01', NULL),
('FL', 'reserve_study', 'sirs_min_stories', 'integer', '3', 'SB 4-D (2022)', '2025-01-01', 'Applies to buildings 3+ stories'),
('FL', 'reserve_study', 'waiver_allowed', 'boolean', 'false', 'SB 4-D (2022)', '2025-01-01', 'Reserve waivers eliminated for SIRS components'),
('FL', 'website_requirements', 'required_for_unit_count', 'integer', '100', 'FL HB 1203 (2025)', '2025-01-01', 'Website required for 100+ unit communities'),
('FL', 'record_retention', 'financial_records_years', 'integer', '7', 'FS 720.303(4)', '2024-01-01', NULL),
('FL', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'FS 720.303(4)', '2024-01-01', NULL),
('FL', 'record_retention', 'meeting_minutes_years', 'integer', '7', 'FS 720.303(4)', '2024-01-01', NULL),
('FL', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'FS 720.306(8)', '2024-01-01', NULL),
('FL', 'voting_rules', 'electronic_voting_allowed', 'boolean', 'true', 'FS 720.317', '2024-01-01', 'With proper authorization'),
('FL', 'estoppel', 'turnaround_business_days', 'integer', '10', 'FS 720.30851', '2024-01-01', NULL),
('FL', 'estoppel', 'fee_cap_cents', 'integer', '25000', 'FS 720.30851', '2024-01-01', '$250 statutory cap');

-- CALIFORNIA
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes) VALUES
('CA', 'meeting_notice', 'board_meeting_notice_days', 'integer', '4', 'Civil Code 4920(a)', '2024-01-01', NULL),
('CA', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '30', 'Civil Code 5115(b)', '2024-01-01', '10-90 day range; 30 typical'),
('CA', 'meeting_notice', 'special_meeting_notice_days', 'integer', '10', 'Civil Code 5115(b)', '2024-01-01', NULL),
('CA', 'meeting_notice', 'emergency_meeting_notice_days', 'integer', '0', 'Civil Code 4923', '2024-01-01', NULL),
('CA', 'fine_limits', 'hearing_required', 'boolean', 'true', 'Civil Code 5855', '2024-01-01', 'IDR/ADR process before fine enforcement'),
('CA', 'fine_limits', 'per_violation_cap_cents', 'text', '"reasonable"', 'Civil Code 5850', '2024-01-01', 'Must be reasonable; no specific cap'),
('CA', 'reserve_study', 'sirs_required', 'boolean', 'true', 'Civil Code 5550', '2024-01-01', 'Reserve study every 3 years'),
('CA', 'reserve_study', 'sirs_interval_years', 'integer', '3', 'Civil Code 5550', '2024-01-01', 'With annual review update'),
('CA', 'record_retention', 'financial_records_years', 'integer', '4', 'Civil Code 5200', '2024-01-01', NULL),
('CA', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'Civil Code 5200', '2024-01-01', NULL),
('CA', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'Civil Code 5130', '2024-01-01', NULL),
('CA', 'estoppel', 'turnaround_business_days', 'integer', '10', 'Civil Code 4530', '2024-01-01', NULL);

-- TEXAS
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes) VALUES
('TX', 'meeting_notice', 'board_meeting_notice_days', 'integer', '3', 'Tex. Prop. Code 209.0051', '2024-01-01', '72 hours notice'),
('TX', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '10', 'Tex. Prop. Code 209.0051', '2024-01-01', '10-60 day range'),
('TX', 'fine_limits', 'hearing_required', 'boolean', 'true', 'Tex. Prop. Code 209.007', '2024-01-01', 'Notice and hearing required'),
('TX', 'record_retention', 'financial_records_years', 'integer', '4', 'Tex. Prop. Code 209.005', '2024-01-01', NULL),
('TX', 'record_retention', 'election_records_years', 'integer', '7', 'Tex. Prop. Code 209.0058', '2024-01-01', NULL),
('TX', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'Tex. Prop. Code 209.005', '2024-01-01', NULL),
('TX', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'Tex. Prop. Code 209.00592', '2024-01-01', NULL);

-- ARIZONA
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes) VALUES
('AZ', 'meeting_notice', 'board_meeting_notice_days', 'integer', '2', 'ARS 33-1804', '2024-01-01', '48 hours notice'),
('AZ', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '10', 'ARS 33-1804', '2024-01-01', '10-50 day range'),
('AZ', 'fine_limits', 'per_violation_cap_cents', 'integer', '5000', 'ARS 33-1803', '2024-01-01', '$50 per violation cap'),
('AZ', 'fine_limits', 'hearing_required', 'boolean', 'false', 'ARS 33-1803', '2024-01-01', 'Written notice required; no committee hearing mandate'),
('AZ', 'record_retention', 'financial_records_years', 'integer', '7', 'ARS 33-1805', '2024-01-01', NULL),
('AZ', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'ARS 33-1805', '2024-01-01', NULL),
('AZ', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'ARS 33-1812', '2024-01-01', NULL),
('AZ', 'voting_rules', 'quorum_percent', 'decimal', '25.0', 'ARS 33-1803', '2024-01-01', '25% quorum for member meetings'),
('AZ', 'estoppel', 'turnaround_business_days', 'integer', '10', 'ARS 33-1806', '2024-01-01', NULL),
('AZ', 'estoppel', 'fee_cap_cents', 'integer', '40000', 'ARS 33-1806', '2024-01-01', '$400 cap');

-- COLORADO
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes) VALUES
('CO', 'meeting_notice', 'board_meeting_notice_days', 'integer', '1', 'CRS 38-33.3-308', '2024-01-01', '24 hours notice'),
('CO', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '10', 'CRS 38-33.3-308', '2024-01-01', '10-50 day range'),
('CO', 'fine_limits', 'per_violation_cap_cents', 'integer', '50000', 'CCIOA 38-33.3-315', '2024-01-01', '$500 per violation typical cap'),
('CO', 'fine_limits', 'hearing_required', 'boolean', 'true', 'CCIOA 38-33.3-315', '2024-01-01', 'Written notice and hearing required'),
('CO', 'record_retention', 'financial_records_years', 'integer', '7', 'CRS 38-33.3-317', '2024-01-01', NULL),
('CO', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'CRS 38-33.3-317', '2024-01-01', NULL),
('CO', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'CRS 38-33.3-310', '2024-01-01', NULL);
