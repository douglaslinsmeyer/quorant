-- Migration: make document_id nullable on estoppel_certificates.
-- Phase 1 does not yet wire document storage; the column will be populated
-- once a real document record exists.
ALTER TABLE estoppel_certificates ALTER COLUMN document_id DROP NOT NULL;
