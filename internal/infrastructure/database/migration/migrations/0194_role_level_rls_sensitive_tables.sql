-- 0194_role_level_rls_sensitive_tables.sql
--
-- Continues the DB review (2026-09-11), finding #1: RLS was enabled on
-- some tables historically but every policy-less table got RLS disabled
-- again by 0181, so nothing was actually protected -- including
-- "Payment", "User", "Installment", "Invoice".
--
-- The application connects through a single pooled role (app_user) with
-- no per-request session context (no SET LOCAL app.current_user_id
-- anywhere in the Go codebase), so per-end-user row isolation is not
-- achievable here without an app-layer change (tracked separately).
--
-- What this migration DOES provide: role-level RLS on the four sensitive
-- tables so that only app_user and migration_user can read/write them at
-- all -- any other Postgres role (a BI/reporting role, a future
-- read-replica analytics user, an admin tool connecting with its own
-- role, or a misconfigured credential) is denied by default instead of
-- getting default table-grant access. This is a real boundary: it does
-- not depend on GRANT hygiene being perfect everywhere else, and closes
-- the "any role that can log in can SELECT * FROM Payment" gap.
--
-- Idempotent and safe if app_user/migration_user don't exist yet (e.g.
-- a fresh local DB provisioned before setup-db-roles.sql runs) -- skips
-- enabling RLS in that case rather than locking everyone out.

DO $$
DECLARE
    v_app_role   text := 'app_user';
    v_migrate_role text := 'migration_user';
    v_table      text;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = v_app_role) THEN
        RAISE NOTICE 'Role % does not exist -- skipping RLS setup (fresh/unprovisioned DB)', v_app_role;
        RETURN;
    END IF;

    FOREACH v_table IN ARRAY ARRAY['Payment', 'User', 'Installment', 'Invoice']
    LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', v_table);
        EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY', v_table);

        EXECUTE format('DROP POLICY IF EXISTS app_role_only ON public.%I', v_table);
        EXECUTE format(
            'CREATE POLICY app_role_only ON public.%I
                FOR ALL
                TO public
                USING (pg_has_role(current_user, %L, ''member'') OR pg_has_role(current_user, %L, ''member''))
                WITH CHECK (pg_has_role(current_user, %L, ''member'') OR pg_has_role(current_user, %L, ''member''))',
            v_table, v_app_role, v_migrate_role, v_app_role, v_migrate_role
        );
    END LOOP;
END;
$$;

-- Table owners bypass RLS by default; keep it that way only for the
-- migration role (needed for ALTER/backfill scripts), not for app_user,
-- since app_user is the role FORCE ROW LEVEL SECURITY is meant to
-- constrain. If app_user happens to own these tables, RLS still applies
-- because FORCE was set above (FORCE applies RLS to the owner too).
