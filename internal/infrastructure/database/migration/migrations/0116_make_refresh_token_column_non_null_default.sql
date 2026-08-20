-- Ensure the legacy refresh_token column has a safe default so inserts
-- do not fail when the application stores only the hash.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'UserSession' AND column_name = 'refresh_token'
    ) THEN
        ALTER TABLE public."UserSession" ALTER COLUMN refresh_token SET DEFAULT '';
        UPDATE public."UserSession" SET refresh_token = COALESCE(refresh_token, '') WHERE refresh_token IS NULL;
    END IF;
END $$;
