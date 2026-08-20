-- Make refresh_token nullable and remove unique index to be compatible with hashed-token model

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'UserSession' AND column_name = 'refresh_token'
    ) THEN
        ALTER TABLE public."UserSession" ALTER COLUMN refresh_token DROP NOT NULL;
    END IF;
END $$;

-- Remove unique index on refresh_token if present (now using refresh_token_hash for uniqueness)
DROP INDEX IF EXISTS idx_usersession_refresh_token;
