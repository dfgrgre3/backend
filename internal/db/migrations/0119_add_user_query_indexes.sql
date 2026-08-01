-- ============================================================
-- Migration 0119: Add additional indexes for User queries
-- ============================================================
-- Optimizes common query patterns for user listing, filtering, and lookups

-- Helper functions (re-used from 0044)
CREATE OR REPLACE FUNCTION pg_temp.has_columns(target_table regclass, required_columns text[])
RETURNS boolean
LANGUAGE sql
AS $$
    SELECT NOT EXISTS (
        SELECT 1
        FROM unnest(required_columns) AS required(column_name)
        WHERE NOT EXISTS (
            SELECT 1
            FROM pg_attribute
            WHERE attrelid = target_table
              AND attname = required.column_name
              AND NOT attisdropped
        )
    );
$$;

CREATE OR REPLACE FUNCTION pg_temp.create_index_if_columns(
    target_table_name text,
    required_columns text[],
    index_sql text
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    target_table regclass;
BEGIN
    target_table := to_regclass(target_table_name);
    IF target_table IS NULL THEN
        RETURN;
    END IF;

    IF pg_temp.has_columns(target_table, required_columns) THEN
        EXECUTE index_sql;
    END IF;
END;
$$;

-- 1. Composite index for user listing with role + status filter (most common admin query)
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['role', 'status', 'deleted_at', 'created_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_role_status_created ON public."User" (role, status, created_at DESC) WHERE deleted_at IS NULL'
);

-- 2. Composite index for user listing with email_verified + role (common filter)
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['email_verified', 'role', 'deleted_at', 'created_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_email_verified_role ON public."User" (email_verified, role, created_at DESC) WHERE deleted_at IS NULL'
);

-- 3. Composite index for user listing with country + role (geo filtering)
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['country', 'role', 'deleted_at', 'created_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_country_role ON public."User" (country, role, created_at DESC) WHERE deleted_at IS NULL'
);

-- 4. Composite index for user listing with subscription status (active_subscription_id + subscription_expires_at)
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['active_subscription_id', 'subscription_expires_at', 'deleted_at', 'created_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_subscription_active ON public."User" (active_subscription_id, subscription_expires_at) WHERE deleted_at IS NULL AND active_subscription_id IS NOT NULL'
);

-- 5. Composite index for user listing with two_factor_enabled filter
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['two_factor_enabled', 'role', 'deleted_at', 'created_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_2fa_role ON public."User" (two_factor_enabled, role, created_at DESC) WHERE deleted_at IS NULL'
);

-- 6. Composite index for instructor filtering (instructor_status)
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['instructor_status', 'role', 'deleted_at', 'created_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_instructor_status ON public."User" (instructor_status, role, created_at DESC) WHERE deleted_at IS NULL'
);

-- 7. Composite index for grade_level + education_type (student filtering)
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['grade_level', 'education_type', 'role', 'deleted_at', 'created_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_grade_education ON public."User" (grade_level, education_type, role, created_at DESC) WHERE deleted_at IS NULL'
);

-- 8. Covering index for user list projection (avoids heap lookup)
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['id', 'email', 'name', 'username', 'avatar', 'phone', 'phone_verified', 'two_factor_enabled', 'role', 'status', 'email_verified', 'country', 'grade_level', 'created_at', 'updated_at', 'last_login', 'total_xp', 'level', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_list_covering ON public."User" (created_at DESC) INCLUDE (id, email, name, username, avatar, phone, phone_verified, two_factor_enabled, role, status, email_verified, country, grade_level, updated_at, last_login, total_xp, level) WHERE deleted_at IS NULL'
);

-- 9. Index for user search by name/username (text search optimization)
SELECT pg_temp.create_index_if_columns(
    'public."User"',
    ARRAY['name', 'username', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_name_username_trgm ON public."User" USING GIN (name gin_trgm_ops, username gin_trgm_ops) WHERE deleted_at IS NULL'
);

-- 10. Index for login history lookups (common in admin security views)
SELECT pg_temp.create_index_if_columns(
    'public."LoginHistory"',
    ARRAY['user_id', 'created_at', 'status'],
    'CREATE INDEX IF NOT EXISTS idx_login_history_user_created_status ON public."LoginHistory" (user_id, created_at DESC, status)'
);

-- 11. Index for security logs by user + event_type + created_at
SELECT pg_temp.create_index_if_columns(
    'public."SecurityLog"',
    ARRAY['user_id', 'event_type', 'created_at', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_security_log_user_event_created ON public."SecurityLog" (user_id, event_type, created_at DESC) WHERE deleted_at IS NULL'
);

-- 12. Index for verification codes by user + type + is_used (auth flow)
SELECT pg_temp.create_index_if_columns(
    'public."VerificationCode"',
    ARRAY['user_id', 'type', 'is_used', 'expires_at', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_verification_code_user_type_used ON public."VerificationCode" (user_id, type, is_used, expires_at) WHERE deleted_at IS NULL'
);

-- 13. Index for OAuth accounts by user + provider (social auth)
SELECT pg_temp.create_index_if_columns(
    'public."OAuthAccount"',
    ARRAY['user_id', 'provider', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_oauth_account_user_provider ON public."OAuthAccount" (user_id, provider) WHERE deleted_at IS NULL'
);

-- 14. Index for user credentials lookup
SELECT pg_temp.create_index_if_columns(
    'public."UserCredential"',
    ARRAY['user_id', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_credential_user ON public."UserCredential" (user_id) WHERE deleted_at IS NULL'
);

-- 15. Index for study sessions by user + subject + date (dashboard queries)
SELECT pg_temp.create_index_if_columns(
    'public."StudySession"',
    ARRAY['user_id', 'subject_id', 'start_time', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_study_session_user_subject_start ON public."StudySession" (user_id, subject_id, start_time DESC) WHERE deleted_at IS NULL'
);

-- Run ANALYZE on affected tables
DO $$
DECLARE
    target_table text;
BEGIN
    FOREACH target_table IN ARRAY ARRAY[
        'public."User"',
        'public."LoginHistory"',
        'public."SecurityLog"',
        'public."VerificationCode"',
        'public."OAuthAccount"',
        'public."UserCredential"',
        'public."StudySession"'
    ] LOOP
        IF to_regclass(target_table) IS NOT NULL THEN
            EXECUTE format('ANALYZE %s', target_table);
        END IF;
    END LOOP;
END $$;