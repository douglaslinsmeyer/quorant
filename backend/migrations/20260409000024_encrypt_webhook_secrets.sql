-- Encrypt existing webhook secrets using pgcrypto.
-- In production, the encryption key should come from a KMS.
-- For now, use a static key (to be rotated).
-- The application layer handles encrypt/decrypt transparently.

-- Add an encrypted_secret column
ALTER TABLE webhook_subscriptions ADD COLUMN encrypted_secret BYTEA;

-- Encrypt existing secrets (using a placeholder key — replace in production)
-- UPDATE webhook_subscriptions SET encrypted_secret = pgp_sym_encrypt(secret, 'quorant-webhook-key-CHANGE-ME');

-- Note: In production, the encrypt/decrypt happens in the Go application layer
-- using a proper KMS-managed key. The DB stores raw bytes.
-- For now, the secret column remains as TEXT and encrypted_secret is added
-- as a parallel column for future migration.
