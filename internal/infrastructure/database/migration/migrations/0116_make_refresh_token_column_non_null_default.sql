-- Ensure the legacy refresh_token column has a safe default so inserts
-- do not fail when the application stores only the hash.
BEGIN;

ALTER TABLE IF EXISTS public."UserSession"
    ALTER COLUMN refresh_token SET DEFAULT '';

UPDATE public."UserSession"
SET refresh_token = COALESCE(refresh_token, '')
WHERE refresh_token IS NULL;

COMMIT;
