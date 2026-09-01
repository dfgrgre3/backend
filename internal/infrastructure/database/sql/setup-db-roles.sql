-- ============================================================================
-- Database Role Setup Script
-- PostgreSQL 16+
-- Safe to run multiple times
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Create application role
-- ----------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'app_user'
    ) THEN
        CREATE ROLE app_user NOLOGIN NOINHERIT;
        RAISE NOTICE 'Created role: app_user';
    ELSE
        RAISE NOTICE 'Role app_user already exists';
    END IF;
END;
$$;

-- ----------------------------------------------------------------------------
-- Create migration role
-- ----------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'migration_user'
    ) THEN
        CREATE ROLE migration_user NOLOGIN NOINHERIT;
        RAISE NOTICE 'Created role: migration_user';
    ELSE
        RAISE NOTICE 'Role migration_user already exists';
    END IF;
END;
$$;

-- ----------------------------------------------------------------------------
-- Application privileges
-- ----------------------------------------------------------------------------
GRANT USAGE ON SCHEMA public TO app_user;

GRANT
    SELECT,
    INSERT,
    UPDATE,
    DELETE,
    TRUNCATE,
    REFERENCES,
    TRIGGER
ON ALL TABLES IN SCHEMA public
TO app_user;

GRANT
    USAGE,
    SELECT
ON ALL SEQUENCES IN SCHEMA public
TO app_user;

GRANT
    EXECUTE
ON ALL FUNCTIONS IN SCHEMA public
TO app_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
GRANT
    SELECT,
    INSERT,
    UPDATE,
    DELETE,
    TRUNCATE,
    REFERENCES,
    TRIGGER
ON TABLES
TO app_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
GRANT
    USAGE,
    SELECT
ON SEQUENCES
TO app_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
GRANT
    EXECUTE
ON FUNCTIONS
TO app_user;

-- ----------------------------------------------------------------------------
-- Migration privileges
-- ----------------------------------------------------------------------------
GRANT USAGE, CREATE ON SCHEMA public TO migration_user;

GRANT
    SELECT,
    INSERT,
    UPDATE,
    DELETE,
    TRUNCATE,
    REFERENCES,
    TRIGGER
ON ALL TABLES IN SCHEMA public
TO migration_user;

GRANT
    USAGE,
    SELECT
ON ALL SEQUENCES IN SCHEMA public
TO migration_user;

GRANT
    EXECUTE
ON ALL FUNCTIONS IN SCHEMA public
TO migration_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
GRANT
    SELECT,
    INSERT,
    UPDATE,
    DELETE,
    TRUNCATE,
    REFERENCES,
    TRIGGER
ON TABLES
TO migration_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
GRANT
    USAGE,
    SELECT
ON SEQUENCES
TO migration_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
GRANT
    EXECUTE
ON FUNCTIONS
TO migration_user;

-- ----------------------------------------------------------------------------
-- Grant roles to current database owner
-- ----------------------------------------------------------------------------
DO $$
DECLARE
    v_current_user text := SESSION_USER;
BEGIN

    IF NOT EXISTS (
        SELECT 1
        FROM pg_auth_members m
        JOIN pg_roles r1 ON m.roleid=r1.oid
        JOIN pg_roles r2 ON m.member=r2.oid
        WHERE r1.rolname='app_user'
        AND r2.rolname=v_current_user
    ) THEN
        EXECUTE format(
            'GRANT app_user TO %I',
            v_current_user
        );

        RAISE NOTICE
            'Granted app_user to %',
            v_current_user;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_auth_members m
        JOIN pg_roles r1 ON m.roleid=r1.oid
        JOIN pg_roles r2 ON m.member=r2.oid
        WHERE r1.rolname='migration_user'
        AND r2.rolname=v_current_user
    ) THEN
        EXECUTE format(
            'GRANT migration_user TO %I',
            v_current_user
        );

        RAISE NOTICE
            'Granted migration_user to %',
            v_current_user;
    END IF;

END;
$$;

-- ----------------------------------------------------------------------------
-- Enable Row Level Security
-- ----------------------------------------------------------------------------
DO $$
DECLARE
    r record;
BEGIN

    FOR r IN

        SELECT
            schemaname,
            tablename
        FROM pg_tables
        WHERE schemaname='public'
        -- Exclude internal telemetry/metrics/system tables that don't require
        -- row-level security and are written to by background processes or the
        -- backend itself (no app_user policy defined for them).
        AND tablename NOT IN (
            'http_metric_buckets',
            'service_health_checks',
            'AnalyticsEvent',
            'AuditLog',
            'login_history'
        )

    LOOP

        BEGIN

            EXECUTE format(
                'ALTER TABLE %I.%I ENABLE ROW LEVEL SECURITY',
                r.schemaname,
                r.tablename
            );

            RAISE NOTICE
                'RLS enabled on %.%',
                r.schemaname,
                r.tablename;

        EXCEPTION
            WHEN OTHERS THEN
                RAISE NOTICE
                    'Skipped %.% (%).',
                    r.schemaname,
                    r.tablename,
                    SQLERRM;
        END;

    END LOOP;

END;
$$;

-- ----------------------------------------------------------------------------
-- Explicitly disable RLS on internal telemetry/analytics tables
-- ----------------------------------------------------------------------------
-- The loop above excludes these tables from ENABLE RLS, but if RLS was
-- enabled on them by an earlier run of this script (before the exclusion
-- was added) or by another provisioning step, we need to explicitly
-- disable it here to restore write access for app_user / migration_user.
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT tablename
        FROM pg_tables
        WHERE schemaname = 'public'
          AND tablename IN ('http_metric_buckets', 'service_health_checks', 'AnalyticsEvent', 'AuditLog', 'login_history')
          AND EXISTS (
              SELECT 1 FROM pg_class c
              JOIN pg_namespace n ON n.oid = c.relnamespace
              WHERE c.relname = pg_tables.tablename
                AND n.nspname = 'public'
                AND c.relrowsecurity
          )
    LOOP
        BEGIN
            EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', r.tablename);
            RAISE NOTICE 'RLS disabled on %.%', 'public', r.tablename;
        EXCEPTION
            WHEN OTHERS THEN
                RAISE NOTICE 'Could not disable RLS on %.% (%).', 'public', r.tablename, SQLERRM;
        END;
    END LOOP;
END;
$$;

-- ----------------------------------------------------------------------------
-- Final summary
-- ----------------------------------------------------------------------------
DO $$
BEGIN

    RAISE NOTICE '';
    RAISE NOTICE '=============================================';
    RAISE NOTICE 'Database role setup completed successfully';
    RAISE NOTICE '';
    RAISE NOTICE 'Application Role : app_user';
    RAISE NOTICE 'Migration Role   : migration_user';
    RAISE NOTICE 'Session User     : %', SESSION_USER;
    RAISE NOTICE '';
    RAISE NOTICE 'Remember:';
    RAISE NOTICE ' - Application should connect using app_user';
    RAISE NOTICE ' - Migrations should use migration_user';
    RAISE NOTICE ' - RLS enabled on public tables (disabled on telemetry/system tables: http_metric_buckets, AnalyticsEvent, AuditLog, login_history)';
    RAISE NOTICE '=============================================';

END;
$$;