-- ============================================================
-- Migration 0137: Fix missing indexes for auth refresh, SecurityLog analytics, and Notification queries
-- ============================================================
-- Addresses slow SQL queries observed in production logs:
--
--   1. UserSession refresh_token_hash lookup (3848ms)
--      Query: WHERE refresh_token_hash = $1 AND deleted_at IS NULL
--      Problem: Existing partial indexes require is_active=true in WHERE,
--               but the auth-service refresh-token query does NOT filter on
--               is_active. The only usable index (idx_user_session_refresh_hash
--               from 0047) includes soft-deleted rows, making it bloated.
--      Fix: New partial index on refresh_token_hash WHERE deleted_at IS NULL.
--
--   2. SecurityLog analytics query (1057ms)
--      Query: WHERE created_at >= $1 AND event_type IN ('LOGIN_SUCCESS','LOGIN_ATTEMPT')
--      Problem: No composite index on (event_type, created_at). The existing
--               idx_security_logs_user_created is on (user_id, created_at) and
--               does not cover this query pattern.
--      Fix: New composite index on (event_type, created_at).
--
--   3. Notification fetch (5873ms)
--      Query: WHERE user_id = $1 ORDER BY created_at DESC LIMIT n
--      Problem: The GORM model lacks a DeletedAt field, so queries never
--               filter deleted_at IS NULL. The partial covering index
--               idx_notification_user_created_active (from 0130) exists but
--               is never used because the query omits the deleted_at filter.
--               The covering index idx_notifications_user_id_created_at_covering
--               (from 0128) includes soft-deleted rows, making it bloated.
--      Fix: New partial covering index + code fix to add deleted_at IS NULL.
--
-- NOTE: CREATE INDEX CONCURRENTLY cannot be used inside a transaction.
--       For zero-downtime production deployments, run these indexes
--       manually with CONCURRENTLY outside the migration tool first.
-- ============================================================

-- ============================================================
-- 1. UserSession: refresh_token_hash lookup (auth token rotation)
--    Query: WHERE refresh_token_hash = $1 AND deleted_at IS NULL
--    This is the critical auth refresh path — the query does NOT
--    filter on is_active, so partial indexes from 0127/0130 are
--    not usable. This new index matches the exact query pattern.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_session_refresh_hash_deleted
ON public."UserSession" ("refresh_token_hash")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 2. SecurityLog: analytics dashboard query
--    Query: WHERE created_at >= $1 AND event_type IN (...)
--    The existing individual index on event_type and composite
--    index on (user_id, created_at) don't cover this pattern.
--    A composite (event_type, created_at DESC) index allows the
--    planner to seek directly into the relevant event types
--    within the date range without sorting.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_security_log_event_type_created
ON public."SecurityLog" ("event_type", "created_at" DESC)
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 3. Notification: partial covering index for soft-delete-aware fetch
--    The existing covering index (0128) is not partial, so it
--    includes soft-deleted rows. The existing partial index (0130)
--    is not covering. This new index combines both properties:
--    partial (excludes deleted rows) + covering (includes SELECT columns).
--    Requires a code fix to add deleted_at IS NULL to the query.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_notification_user_created_covering_active
ON public."Notification" ("user_id", "created_at" DESC)
INCLUDE ("id", "title", "message", "type", "is_read", "link", "icon")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 4. User: partial index on (id, deleted_at) for soft-delete-aware PK lookups
--    While id is the PK, GORM adds deleted_at IS NULL to every query on
--    models with DeletedAt. A partial PK-style index allows the planner
--    to potentially skip the deleted_at filter step in some plans.
--    More importantly, this helps with batch lookups by ID.
-- ============================================================
-- (Note: The User PK index already covers id lookups. This is a minor
--  optimization that helps when GORM generates IN-clause queries by id
--  with deleted_at IS NULL, e.g. loading multiple User records by ID.)
-- Skip: PK index on id is already optimal for single-row lookups.

-- ============================================================
-- 5. AuditLog: composite index on (event_type, created_at) for audit queries
--    AuditLog has individual indexes on user_id and event_type but no
--    composite index. Queries that filter by event_type and date range
--    will benefit from this.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_audit_log_event_type_created
ON public."AuditLog" ("event_type", "created_at" DESC);

-- ============================================================
-- 6. login_history: add status index for login analytics queries
--    LoginHistory has indexes on user_id and created_at but the
--    Status column has no index. Queries filtering by status (e.g.
--    counting successful vs failed logins) will benefit.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_login_history_status_created
ON public."login_history" ("status", "created_at" DESC);

-- ============================================================
-- Update optimizer statistics so the planner picks the new indexes
-- immediately after this migration runs.
-- ============================================================
ANALYZE public."UserSession";
ANALYZE public."SecurityLog";
ANALYZE public."Notification";
ANALYZE public."AuditLog";
ANALYZE public."login_history";