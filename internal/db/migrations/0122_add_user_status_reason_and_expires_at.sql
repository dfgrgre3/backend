-- 0122_add_user_status_reason_and_expires_at.sql
-- Add status_reason and status_expires_at columns to User table

ALTER TABLE "User"
ADD COLUMN IF NOT EXISTS status_reason TEXT,
ADD COLUMN IF NOT EXISTS status_expires_at TIMESTAMPTZ;
