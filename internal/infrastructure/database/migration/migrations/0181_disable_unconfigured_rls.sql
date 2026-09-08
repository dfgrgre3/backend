-- 0181: Disable RLS where no policies exist
--
-- setup-db-roles.sql historically enabled RLS on every public table, even
-- when that table had no policy. PostgreSQL then applies its default-deny
-- behavior, which breaks normal application reads and writes (including
-- registration with SQLSTATE 42501 on public."User").
--
-- Keep RLS enabled for tables that have an actual policy. For all other
-- tables, disable it until a policy-aware access model is implemented.
-- This is idempotent and also repairs databases provisioned by the old
-- setup script.

DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT c.oid, c.relname
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relkind IN ('r', 'p')
          AND c.relrowsecurity
          AND NOT EXISTS (
              SELECT 1
              FROM pg_policy p
              WHERE p.polrelid = c.oid
          )
    LOOP
        EXECUTE format('ALTER TABLE public.%I DISABLE ROW LEVEL SECURITY', r.relname);
        RAISE NOTICE 'Disabled unconfigured RLS on public.%', r.relname;
    END LOOP;
END;
$$;
