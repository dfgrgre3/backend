-- Migration to add MFA fields to User table
ALTER TABLE IF EXISTS public."User"
    ADD COLUMN IF NOT EXISTS two_factor_secret text,
    ADD COLUMN IF NOT EXISTS backup_codes text;
