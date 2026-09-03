-- =============================================================
-- 0177: CQRS read-model materialized view ownership
-- -------------------------------------------------------------
-- The CQRS background worker (internal/application/cqrs/
-- readmodel_refresher.go) refreshes these views over the app-role
-- connection (app_user). PostgreSQL requires the invoking role to
-- be the OWNER of a materialized view for
-- `REFRESH MATERIALIZED VIEW CONCURRENTLY`; otherwise it fails with
-- SQLSTATE 42501 (permission denied).
--
-- This migration transfers ownership of the read-model matviews
-- from the migration/superuser role to app_user. Read models are
-- application-managed derived data, so app_user ownership is safe:
--   - app_user implicitly gets SELECT on the view.
--   - Source tables are not affected and RLS stays intact.
--
-- Idempotent: re-running skips views already owned by app_user,
-- and skips entirely if the app_user role does not exist (e.g.
-- environments without the app-role split).
-- =============================================================

DO $$
DECLARE
    v_views    TEXT[] := ARRAY[
        'mv_user_progress_summary',
        'mv_user_weekly_analytics',
        'mv_user_watch_time'
    ];
    v_app_role TEXT   := 'app_user';
    v_view     TEXT;
    v_owner    TEXT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = v_app_role) THEN
        RAISE NOTICE 'Role % does not exist; skipping matview ownership transfer', v_app_role;
        RETURN;
    END IF;

    -- migration_user needs membership in app_user to execute
    -- ALTER ... OWNER TO app_user. Swallow the error when the
    -- executing role lacks ADMIN OPTION on app_user (superusers
    -- and the role creator do not).
    BEGIN
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'migration_user')
           AND NOT pg_has_role('migration_user', v_app_role, 'member') THEN
            GRANT app_user TO migration_user;
            RAISE NOTICE 'Granted app_user membership to migration_user';
        END IF;
    EXCEPTION WHEN insufficient_privilege THEN
        RAISE NOTICE 'Insufficient privilege to grant app_user to migration_user; run manually as superuser';
    END;

    FOREACH v_view IN ARRAY v_views LOOP
        SELECT matviewowner INTO v_owner
        FROM pg_matviews
        WHERE schemaname = 'public' AND matviewname = v_view;

        IF v_owner IS NULL THEN
            RAISE NOTICE 'Matview %.% does not exist; skipping', 'public', v_view;
        ELSIF v_owner <> v_app_role THEN
            EXECUTE format('ALTER MATERIALIZED VIEW public.%I OWNER TO %I', v_view, v_app_role);
            RAISE NOTICE 'Transferred ownership of % to %', v_view, v_app_role;
        ELSE
            RAISE NOTICE '% already owned by %', v_view, v_app_role;
        END IF;
    END LOOP;
END $$;
