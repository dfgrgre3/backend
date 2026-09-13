-- P007 audit note: this filename shares the numeric prefix "0115" with
-- 0115_create_credentials_tables.sql. This is NOT a real id collision: the
-- migrator (migration_apply.go) keys schema_migrations by the FULL filename
-- minus ".sql", not by the numeric prefix, and applies migrations in
-- sort.Strings() order over that full name — so both files get distinct,
-- deterministic ids ("create_..." sorts before "make_..."). This migration
-- is already applied in existing environments; renaming it would change its
-- tracked id and cause the migrator to treat it as a new pending migration
-- (re-apply risk), so per migrations/README.md it is intentionally left
-- in place rather than renumbered. See migrations/README.md "Known
-- duplicate number prefixes".
--
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
