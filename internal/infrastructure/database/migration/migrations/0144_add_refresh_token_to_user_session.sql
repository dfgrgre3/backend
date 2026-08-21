-- Add refresh_token column to UserSession table.
-- The original table was created with only refresh_token_hash (for security).
-- The Go model stores a UUID placeholder in refresh_token before each INSERT/UPDATE
-- so the raw token is never persisted, but the column must exist.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name   = 'UserSession'
          AND column_name  = 'refresh_token'
    ) THEN
        ALTER TABLE public."UserSession"
            ADD COLUMN refresh_token text;
    END IF;
END $$;
