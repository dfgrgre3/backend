-- ============================================================
-- Migration 0141: Fix remaining slow SQL queries from production logs
-- ============================================================
-- Addresses slow queries still observed after migrations 0127-0137:
--
--   1. SystemSetting key lookup (2733ms)
--      Query: WHERE key = $1 AND deleted_at IS NULL LIMIT 1
--      The existing partial index (0127) may not be used because
--      the query planner chooses a seq scan on small tables.
--      Force index usage with a covering index.
--
--   2. User email login lookup (8715ms)
--      Query: WHERE email = $1 AND deleted_at IS NULL ORDER BY id LIMIT 1
--      The existing partial index (0127) exists but the ORDER BY id
--      may cause a sort. Add a covering index with id included.
--
--   3. Notification fetch (1119ms)
--      Query: WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT n
--      The covering partial index from 0137 exists but the model
--      lacks DeletedAt so GORM doesn't add the filter. The query
--      in the handler DOES add deleted_at IS NULL manually.
--      Add a composite index on (user_id, created_at DESC) with
--      deleted_at filter to match the exact query pattern.
--
--   4. Admin dashboard: LmsCourse status count (1384ms)
--      Query: WHERE deleted_at IS NULL AND status = $1
--      Add composite index on (status, deleted_at).
--
--   5. Admin dashboard: Payment revenue by date (2039ms)
--      Query: WHERE deleted_at IS NULL AND created_at >= $1
--      Add composite index on (created_at, deleted_at).
--
--   6. Admin dashboard: UserSubscription pending revenue (618ms)
--      Query: LEFT JOIN SubscriptionPlan sp ON UserSubscription.plan_id = sp.id
--             WHERE UserSubscription.status = 'PENDING'
--      Add index on (status, plan_id).
--
--   7. Admin dashboard: SecurityLog recent (532ms)
--      Query: ORDER BY created_at DESC LIMIT n
--      Add index on (created_at DESC).
--
--   8. Admin users list: batch counts (1657ms)
--      Query: WHERE user_id IN (...) AND deleted_at IS NULL GROUP BY user_id
--      on Task, StudySession, UserAchievement, SubjectEnrollment
--      Existing partial indexes from 0127 cover these. Add ANALYZE
--      to ensure the planner picks them up.
--
--   9. http_metric_buckets upsert (4777ms)
--      The ON CONFLICT upsert is slow because the batch is large.
--      Add an index on (bucket_start) to speed up conflict detection.
--
-- NOTE: CREATE INDEX CONCURRENTLY cannot be used inside a transaction.
--       For zero-downtime production deployments, run these indexes
--       manually with CONCURRENTLY outside the migration tool first.
-- ============================================================

-- ============================================================
-- 1. SystemSetting: covering index for key lookup
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_system_setting_key_covering
ON public."SystemSetting" ("key")
INCLUDE ("value", "updated_at")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 2. User: covering index for email login lookup
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_email_covering
ON public."User" ("email")
INCLUDE ("id")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 3. Notification: composite index matching exact query pattern
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_notification_user_created_deleted
ON public."Notification" ("user_id", "created_at" DESC)
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 4. LmsCourse: status + deleted_at composite
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_lms_course_status_deleted
ON public."LmsCourse" ("status")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 5. Payment: created_at + deleted_at composite for revenue queries
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_payment_created_deleted
ON public."Payment" ("created_at" DESC)
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 6. UserSubscription: status + plan_id for pending revenue
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_subscription_status_plan
ON public."UserSubscription" ("status", "plan_id");

-- ============================================================
-- 7. SecurityLog: created_at DESC for recent alerts
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_security_log_created_desc
ON public."SecurityLog" ("created_at" DESC);

-- ============================================================
-- 8. http_metric_buckets: bucket_start index for upsert conflict
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_http_metric_buckets_bucket_start
ON public.http_metric_buckets ("bucket_start" DESC);

-- ============================================================
-- Update optimizer statistics so the planner picks the new indexes
-- immediately after this migration runs.
-- ============================================================
ANALYZE public."SystemSetting";
ANALYZE public."User";
ANALYZE public."Notification";
ANALYZE public."LmsCourse";
ANALYZE public."Payment";
ANALYZE public."UserSubscription";
ANALYZE public."SecurityLog";
ANALYZE public.http_metric_buckets;
ANALYZE public."Task";
ANALYZE public."StudySession";
ANALYZE public."UserAchievement";
ANALYZE public."SubjectEnrollment";