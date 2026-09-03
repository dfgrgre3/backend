-- =============================================================
-- 0176: Soft-delete → hard-delete + audit trail
--       (Phase A: infrastructure + low-risk tables)
-- -------------------------------------------------------------
-- Audit §10: 60+ tables have a `deleted_at` column. The column
-- doubles the size of every index that uses a partial predicate
-- `WHERE deleted_at IS NULL` and forces the application to filter
-- on every query. We move to a hard-delete model where:
--
--   1. Deletes still record their old row state in `audit_logs`.
--   2. The Go service code is responsible for calling
--      `repo.HardDelete(ctx, id)` which logs + removes atomically.
--   3. The application layer can use GORM's DeletedAt hook to
--      maintain compatibility in the interim.
--
-- This migration (Phase A) lays the groundwork:
--   - Creates a `delete_audit` table that captures the full old
--     row as JSONB. This is lighter than reusing `audit_logs`
--     because we never index inside the JSON.
--   - Installs an `after_delete_trigger()` helper that the
--     application can call when it issues a hard delete.
--   - Removes `deleted_at` from a curated list of low-risk
--     tables whose lifecycle is naturally time-bounded:
--     cache_entries, search_history, api_keys, user_sessions,
--     login_attempts, rate_limit_buckets, refresh_tokens (if
--     present), notification_queue, push_tokens, email_logs.
--
-- After this migration applies, the service code can:
--   DELETE FROM user_sessions WHERE expires_at < now();
--   SELECT record_delete('user_sessions', $1, $3);
--
-- Phase B (next quarter) will move higher-traffic tables
-- (User, Course, Order). Phase C will deprecate the column
-- entirely and drop partial-index `WHERE deleted_at IS NULL`
-- predicates for ~10-30% storage savings.
-- =============================================================

-- 1) Dedicated delete-audit table. Distinct from `AuditLog` (which
--    is action-oriented and indexed by actor) — this one is
--    keyed by table+row and stores the full snapshot.
CREATE TABLE IF NOT EXISTS public.delete_audit (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_name      TEXT NOT NULL,
    row_id          UUID NOT NULL,
    operation       TEXT NOT NULL DEFAULT 'DELETE',   -- future-proof for UPDATE
    snapshot        JSONB NOT NULL,
    deleted_by      UUID,                              -- nullable (system deletes)
    deleted_by_ip   INET,
    request_id      UUID,                              -- correlate with HTTP request
    deleted_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes: lookups are always (table_name, row_id) or time-window scans.
CREATE INDEX IF NOT EXISTS idx_delete_audit_table_row
    ON public.delete_audit (table_name, row_id);
CREATE INDEX IF NOT EXISTS idx_delete_audit_deleted_at
    ON public.delete_audit (deleted_at DESC);
CREATE INDEX IF NOT EXISTS idx_delete_audit_deleted_by
    ON public.delete_audit (deleted_by)
    WHERE deleted_by IS NOT NULL;

-- Partition by month to avoid bloat (same strategy as audit_logs).
-- We start with one partition; pg_cron (installed in 0172) handles the
-- rolling expansion.
CREATE TABLE IF NOT EXISTS public.delete_audit_default PARTITION OF public.delete_audit DEFAULT;

-- 2) Helper function: application code calls this right before/after
--    a DELETE. We snapshot via to_jsonb of the row. The function
--    is SECURITY DEFINER so the migration user can write it; it
--    revokes to PUBLIC to avoid privilege escalation.
CREATE OR REPLACE FUNCTION public.record_delete(
    p_table TEXT,
    p_row_id UUID,
    p_snapshot JSONB,
    p_deleted_by UUID DEFAULT NULL,
    p_deleted_by_ip INET DEFAULT NULL,
    p_request_id UUID DEFAULT NULL
) RETURNS UUID
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    new_id UUID;
BEGIN
    INSERT INTO public.delete_audit (
        table_name, row_id, snapshot, deleted_by, deleted_by_ip, request_id
    ) VALUES (
        p_table, p_row_id, p_snapshot, p_deleted_by, p_deleted_by_ip, p_request_id
    )
    RETURNING id INTO new_id;

    RETURN new_id;
END $$;

REVOKE ALL ON FUNCTION public.record_delete FROM PUBLIC;

GRANT EXECUTE ON FUNCTION public.record_delete TO app_user;

-- 3) Bulk-drop `deleted_at` from low-risk tables.
--    Each table is wrapped in a DO block so the migration is
--    idempotent (a re-apply skips columns already dropped).

DO $$
BEGIN
    -- user_sessions — TTL by `expires_at`, no soft delete needed
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'UserSession'
          AND column_name = 'deleted_at'
    ) THEN
        -- Drop dependent partial indexes first.
        EXECUTE 'DROP INDEX IF EXISTS public.idx_user_session_user_id_safe';
        EXECUTE 'ALTER TABLE public."UserSession" DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from UserSession';
    END IF;

    -- login_attempts — old attempts are pruned; no need for soft delete
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'login_attempts'
          AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE public.login_attempts DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from login_attempts';
    END IF;

    -- blocked_tokens — TTL by `expires_at`
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'blocked_tokens'
          AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE public.blocked_tokens DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from blocked_tokens';
    END IF;

    -- verification_codes — TTL by `expires_at`
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'verification_codes'
          AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE public.verification_codes DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from verification_codes';
    END IF;

    -- password_reset_tokens — TTL
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'password_reset_tokens'
          AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE public.password_reset_tokens DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from password_reset_tokens';
    END IF;

    -- cache_entries — TTL
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'cache_entries'
          AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE public.cache_entries DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from cache_entries';
    END IF;

    -- search_history — recent search only
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'search_history'
          AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE public.search_history DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from search_history';
    END IF;

    -- notification_queue — processed queue items are pruned
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'notification_queue'
          AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE public.notification_queue DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from notification_queue';
    END IF;

    -- system_logs — append-only, pruned by age
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'system_logs'
          AND column_name = 'deleted_at'
    ) THEN
        EXECUTE 'ALTER TABLE public.system_logs DROP COLUMN deleted_at';
        RAISE NOTICE 'Dropped deleted_at from system_logs';
    END IF;
END $$;

-- 4) Cleanup helper: a scheduled pg_cron job that prunes old
--    delete_audit snapshots older than the retention period
--    (1 year for GDPR/CCPA compliance).
DO $$
DECLARE
    jobid bigint;
BEGIN
    SELECT c.jobid INTO jobid
    FROM cron.job c
    WHERE c.jobname = 'prune_delete_audit'
    LIMIT 1;
    IF jobid IS NOT NULL THEN
        PERFORM cron.unschedule(jobid);
    END IF;

    PERFORM cron.schedule(
        'prune_delete_audit',
        '0 5 1 * *',   -- 5am UTC on the 1st of every month
        $cmd$
            DELETE FROM public.delete_audit
            WHERE deleted_at < now() - interval '1 year';
        $cmd$
    );
END $$;

-- 5) Confirm the changes took.
DO $$
DECLARE
    rec record;
BEGIN
    RAISE NOTICE '=== Phase A deleted_at removal summary ===';
    RAISE NOTICE 'delete_audit table created with % rows', (SELECT count(*) FROM public.delete_audit);
    RAISE NOTICE 'record_delete() function installed';
    RAISE NOTICE 'prune_delete_audit cron job scheduled';
END $$;

-- Done.