-- Initial migration: install required PostgreSQL extensions
-- These extensions are used across multiple modules.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";    -- UUID generation functions
CREATE EXTENSION IF NOT EXISTS "ltree";        -- hierarchical tree data type (org hierarchy)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";     -- cryptographic functions
CREATE EXTENSION IF NOT EXISTS "vector";       -- pgvector for AI document embeddings
CREATE EXTENSION IF NOT EXISTS "btree_gist";   -- required for exclusion constraints (amenity reservations)
