-- Migration 0145: Fix UserSession ip column name mismatch.
-- The table was created with 'ip_address' (inet type) in 0001, but the Go
-- model maps to column 'ip' (text). This migration adds the 'ip' text column
-- and copies existing data from 'ip_address', then drops 'ip_address'.

DO $$
BEGIN
    -- Step 1: Add 'ip' column if it doesn't exist yet.
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name   = 'UserSession'
          AND column_name  = 'ip'
    ) THEN
        ALTER TABLE public."UserSession" ADD COLUMN ip text;
    END IF;

    -- Step 2: Copy data from ip_address -> ip (if ip_address still exists).
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name   = 'UserSession'
          AND column_name  = 'ip_address'
    ) THEN
        UPDATE public."UserSession"
        SET ip = ip_address::text
        WHERE ip IS NULL AND ip_address IS NOT NULL;

        ALTER TABLE public."UserSession" DROP COLUMN ip_address;
    END IF;
END $$;
