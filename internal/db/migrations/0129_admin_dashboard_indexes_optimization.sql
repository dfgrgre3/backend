-- ============================================================
-- Migration 0129: Admin Dashboard & User Settings Performance Optimization
-- ============================================================
-- Optimizes:
-- 1. GET /api/settings lookup & creation
--    Index on UserSettings (user_id) where deleted_at IS NULL
-- 2. User growth & metrics filtering: GET /api/admin/dashboard
--    Query: User role, deleted_at, and created_at filtering
-- 3. Study Session aggregations:
--    Query: StudySession start_time + duration_min + user_id daily aggregations
--    Covering index avoids heap fetch for dashboard chart queries.
-- 4. Subscription revenue calculation:
--    Query: UserSubscription status, plan_id, and created_at range joins
--
-- NOTE: idx_user_created_at_month_agg is intentionally omitted here.
--       It is already covered by idx_user_created_at_desc_deleted created
--       in migration 0127. Adding it again would be redundant and waste
--       storage + write amplification on every INSERT/UPDATE to User.
--
-- NOTE: CREATE INDEX CONCURRENTLY is intentionally NOT used here because
--       the migration framework wraps statements in a transaction.
--       For zero-downtime production deployments, run these indexes manually
--       with CONCURRENTLY outside the migration tool after deploying.
-- ============================================================

-- ============================================================
-- 1. UserSettings: lookup by user_id (GET /api/settings)
--    Query: WHERE user_id = $1 AND deleted_at IS NULL LIMIT 1
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_settings_user_id_active
ON public."UserSettings" ("user_id")
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_user_settings_user_id_active
IS 'Partial index for UserSettings lookup by user_id, excluding soft-deleted rows.';

-- ============================================================
-- 2. User: dashboard role + created_at composite filter
--    Query: WHERE role = $1 AND deleted_at IS NULL ORDER BY created_at DESC
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_role_created_at
ON public."User" ("role", "created_at" DESC)
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_user_role_created_at
IS 'Composite partial index for admin dashboard user growth queries filtered by role.';

-- ============================================================
-- 3. StudySession: dashboard chart aggregation (covering index)
--    Query: WHERE deleted_at IS NULL GROUP BY DATE(start_time)
--           SUM(duration_min), COUNT(*) per user
--    INCLUDE avoids heap fetch for the dashboard SELECT columns.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_studysession_start_time_covering
ON public."StudySession" ("start_time")
INCLUDE ("user_id", "duration_min")
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_studysession_start_time_covering
IS 'Covering partial index for dashboard daily study aggregation. INCLUDE prevents heap fetch for duration_min and user_id.';

-- ============================================================
-- 4. UserSubscription: revenue calculations (status + plan + time range)
--    Query: WHERE status = $1 AND plan_id = $2
--           AND created_at BETWEEN $3 AND $4
--    Adding created_at as a third column supports date-range slicing
--    for monthly/quarterly revenue reports on the admin dashboard.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_subscription_status_plan_created
ON public."UserSubscription" ("status", "plan_id", "created_at" DESC);

COMMENT ON INDEX public.idx_user_subscription_status_plan_created
IS 'Composite index for subscription revenue queries. Includes created_at for date-range revenue reports.';

-- ============================================================
-- Update PostgreSQL optimizer statistics so the planner picks
-- the new indexes immediately after this migration runs.
-- ============================================================
ANALYZE public."UserSettings";
ANALYZE public."User";
ANALYZE public."StudySession";
ANALYZE public."UserSubscription";
