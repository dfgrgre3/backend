-- Migration 0139: Safety net for RLS on http_metric_buckets
--
-- This migration is a follow-up to 0138. If 0138 was already recorded as applied
-- in schema_migrations but the ALTER TABLE did not actually take effect (e.g., the
-- migration was applied through a tool that swallowed the BEGIN/COMMIT conflict
-- error, or RLS was re-enabled by a subsequent setup-db-roles.sql run), this
-- migration ensures RLS is disabled on http_metric_buckets at the next migration run.
--
-- The statement is idempotent: if RLS is already disabled, it is a no-op.
-- ALTER TABLE IF EXISTS guards against the table not existing yet.
--
-- No BEGIN/COMMIT wrapper — the migration runner wraps each migration in
-- a database.Transaction() automatically.

ALTER TABLE IF EXISTS public.http_metric_buckets DISABLE ROW LEVEL SECURITY;
