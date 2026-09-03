-- =============================================================
-- 0174: Per-table autovacuum tuning for hot tables
-- -------------------------------------------------------------
-- Audit §6: the "roles" table had 90% dead tuples because the
-- default autovacuum threshold (scale_factor=0.2) is too loose
-- for small/medium tables. With 50 dead tuples the table never
-- triggers. We switch to threshold-based triggering so dead-tuple
-- removal runs predictably regardless of table size.
--
-- Per-table storage parameters beat cluster-wide ALTER SYSTEM
-- because they only affect the rows that actually need attention.
-- =============================================================

-- Hot write/read tables identified in the audit:
--   - "roles"        (tiny, almost no traffic but 90% dead)
--   - "User"         (the user table — frequent updates)
--   - "Notification" (delete-heavy, large)
--   - "ExamResult"   (insert-only on partitions, delete via DEFAULT)
--   - "Coupon" / "CouponUsage" (insert/delete mix)
--   - "UserSession"  (delete-heavy — refresh tokens expire)
--   - "AuditLog"     (insert-only, but should be analyzed)
--
-- The pattern below is (scale_factor=0.05, threshold=50) — keep
-- the GLOBAL fallback but require vacuum to fire when EITHER
-- condition is hit. This is the standard production tuning.

-- 1) roles — small table, threshold-driven.
DO $$
BEGIN
    EXECUTE 'ALTER TABLE public."roles" SET (
        autovacuum_vacuum_scale_factor = 0.05,
        autovacuum_vacuum_threshold    = 50,
        autovacuum_analyze_scale_factor = 0.02,
        autovacuum_analyze_threshold    = 50,
        autovacuum_vacuum_cost_delay   = 5
    )';
END $$;

-- 2) User — medium-large, scale-driven 0.05.
DO $$
BEGIN
    EXECUTE 'ALTER TABLE public."User" SET (
        autovacuum_vacuum_scale_factor = 0.05,
        autovacuum_vacuum_threshold    = 100,
        autovacuum_analyze_scale_factor = 0.02,
        autovacuum_analyze_threshold    = 100
    )';
END $$;

-- 3) Notification — very high write volume, aggressive.
DO $$
BEGIN
    EXECUTE 'ALTER TABLE public."Notification" SET (
        autovacuum_vacuum_scale_factor = 0.03,
        autovacuum_vacuum_threshold    = 500,
        autovacuum_analyze_scale_factor = 0.02,
        autovacuum_analyze_threshold    = 500,
        autovacuum_vacuum_cost_delay   = 5
    )';
END $$;

-- 4) ExamResult (parent of partitions; storage params propagate).
DO $$
BEGIN
    EXECUTE 'ALTER TABLE public."ExamResult" SET (
        autovacuum_vacuum_scale_factor = 0.05,
        autovacuum_vacuum_threshold    = 1000,
        autovacuum_analyze_scale_factor = 0.02,
        autovacuum_analyze_threshold    = 1000
    )';
END $$;

-- 5) Coupon / CouponUsage — small, threshold-driven.
DO $$
BEGIN
    EXECUTE 'ALTER TABLE public."Coupon" SET (
        autovacuum_vacuum_scale_factor = 0.1,
        autovacuum_vacuum_threshold    = 50
    )';
END $$;

DO $$
BEGIN
    EXECUTE 'ALTER TABLE public."CouponUsage" SET (
        autovacuum_vacuum_scale_factor = 0.1,
        autovacuum_vacuum_threshold    = 50
    )';
END $$;

-- 6) UserSession — TTL-heavy table, threshold-driven.
DO $$
BEGIN
    EXECUTE 'ALTER TABLE public."UserSession" SET (
        autovacuum_vacuum_scale_factor = 0.05,
        autovacuum_vacuum_threshold    = 200,
        autovacuum_analyze_scale_factor = 0.02
    )';
END $$;

-- 7) AuditLog — insert-only, but make sure analyze fires so the
--    planner stats are fresh for the security dashboard.
DO $$
BEGIN
    EXECUTE 'ALTER TABLE public."AuditLog" SET (
        autovacuum_analyze_scale_factor = 0.02,
        autovacuum_analyze_threshold    = 500
    )';
END $$;

-- 8) Refresh materialized view cron job — daily at 04:00 UTC.
DO $$
DECLARE
    jobid bigint;
BEGIN
    SELECT c.jobid INTO jobid
    FROM cron.job c
    WHERE c.jobname = 'refresh_materialized_views'
    LIMIT 1;
    IF jobid IS NOT NULL THEN
        PERFORM cron.unschedule(jobid);
    END IF;

    PERFORM cron.schedule(
        'refresh_materialized_views',
        '0 4 * * *',
        $cmd$
            REFRESH MATERIALIZED VIEW CONCURRENTLY public.mv_user_weekly_analytics;
            REFRESH MATERIALIZED VIEW CONCURRENTLY public.mv_user_progress_summary;
            REFRESH MATERIALIZED VIEW CONCURRENTLY public.mv_user_watch_time;
        $cmd$
    );
END $$;

-- 9) Sanity reporting.
DO $$
DECLARE
    rec record;
BEGIN
    RAISE NOTICE 'Per-table autovacuum settings:';
    FOR rec IN
        SELECT c.relname,
               c.reloptions
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.reloptions IS NOT NULL
          AND c.relname IN ('roles', 'User', 'Notification', 'ExamResult',
                            'Coupon', 'CouponUsage', 'UserSession', 'AuditLog')
    LOOP
        RAISE NOTICE '  % = %', rec.relname, rec.reloptions;
    END LOOP;
END $$;

-- Done.