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
    RAISE NOTICE ' - RLS has been enabled on public tables';
    RAISE NOTICE '=============================================';

END;
$$;