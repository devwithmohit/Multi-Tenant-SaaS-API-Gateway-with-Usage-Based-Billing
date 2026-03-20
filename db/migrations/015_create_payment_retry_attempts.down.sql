-- Migration 015: Drop payment_retry_attempts table (rollback)
DROP TABLE IF EXISTS payment_retry_attempts;
