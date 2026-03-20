-- Migration: Add missing columns to api_keys table
-- Version: 008
-- Description: Adds last_used_at and created_by columns that were originally
--   defined in migration 007's duplicate CREATE TABLE block (now removed).
--   These columns are referenced by dashboard-api code.

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS created_by UUID;
