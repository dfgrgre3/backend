-- Make refresh_token nullable and remove unique index to be compatible with hashed-token model
BEGIN;

-- Allow refresh_token to be NULL (new codebase stores only refresh_token_hash)
ALTER TABLE IF EXISTS public."UserSession"
    ALTER COLUMN refresh_token DROP NOT NULL;

-- Remove unique index on refresh_token if present (now using refresh_token_hash for uniqueness)
DROP INDEX IF EXISTS idx_usersession_refresh_token;

COMMIT;
