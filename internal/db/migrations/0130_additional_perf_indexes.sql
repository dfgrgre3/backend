-- ============================================================
-- Migration 0130: Additional Performance Indexes
-- ============================================================
-- Covers query patterns not addressed in earlier migrations:
--
--   1. User: composite (role, status, created_at) for admin user-list
--      with simultaneous role + status filters.
--   2. User: (status, role, created_at) for status-first admin queries.
--   3. SubjectEnrollment / Enrollment: reverse lookup by subject_id
--      to speed up "who is enrolled in subject X" queries.
--   4. ExamResult: (user_id, exam_id, created_at DESC) for per-user
--      exam history pages.
--   5. Notification: add deleted_at guard to the existing user+created_at
--      index so soft-deleted notifications are excluded automatically.
--   6. UserSession: token_hash unique partial index for O(1) session
--      lookup without a full table scan.
--   7. UserAchievement: (user_id, achievement_id) covering lookup.
--   8. StudySession: (user_id, start_time DESC) for user timeline queries.
--
-- NOTE: CREATE INDEX CONCURRENTLY cannot be used inside a transaction.
--       For zero-downtime production deployments, run these indexes
--       manually with CONCURRENTLY outside the migration tool first,
--       then let this migration record them as applied.
-- ============================================================

-- ============================================================
-- 1. User: role + status + created_at (admin user list with
--    combined role and status filters)
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_role_status_created
ON public."User" ("role", "status", "created_at" DESC)
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_user_role_status_created
IS 'Composite partial index for admin user-list filtered by role AND status simultaneously.';

-- ============================================================
-- 2. User: status + role + created_at (status-first variant for
--    queries that filter on status before role)
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_status_role_created
ON public."User" ("status", "role", "created_at" DESC)
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_user_status_role_created
IS 'Status-first composite partial index for admin user-list queries ordered by status then role.';

-- ============================================================
-- 3a. SubjectEnrollment: reverse lookup by subject_id
--     (who is enrolled in subject X?)
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_subject_enrollment_subject_user
ON public."SubjectEnrollment" ("subject_id", "user_id")
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_subject_enrollment_subject_user
IS 'Reverse enrollment lookup: subject_id → user_id for subject roster queries.';

-- ============================================================
-- 3b. Enrollment: reverse lookup by subject_id (generic table if present)
-- ============================================================
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'Enrollment') THEN
        CREATE INDEX IF NOT EXISTS idx_enrollment_subject_user
        ON public."Enrollment" ("subject_id", "user_id");
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'idx_enrollment_subject_user') THEN
        COMMENT ON INDEX public.idx_enrollment_subject_user IS 'Reverse enrollment lookup for generic Enrollment table.';
    END IF;
END $$;

-- ============================================================
-- 4. ExamResult: per-user exam history
--    Query: WHERE user_id = $1 ORDER BY created_at DESC
--           optionally AND exam_id = $2
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_exam_result_user_exam_created
ON public."ExamResult" ("user_id", "exam_id", "created_at" DESC)
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_exam_result_user_exam_created
IS 'Covering index for per-user exam history queries ordered by date.';

-- ============================================================
-- 5. Notification: soft-delete aware covering index
--    Supersedes the non-partial idx_notification_user_created
--    by adding a WHERE deleted_at IS NULL guard so soft-deleted
--    notifications are never returned without a filter step.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_notification_user_created_active
ON public."Notification" ("user_id", "created_at" DESC)
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_notification_user_created_active
IS 'Partial notification index excluding soft-deleted rows. Preferred over the non-partial variant.';

-- ============================================================
-- 6. UserSession: refresh_token_hash unique partial index
--    Allows O(1) session lookup without full table scan.
--    Only active, non-deleted sessions are indexed.
-- ============================================================
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_session_refresh_token_hash_active
ON public."UserSession" ("refresh_token_hash")
WHERE "deleted_at" IS NULL AND "is_active" = true;

COMMENT ON INDEX public.idx_user_session_refresh_token_hash_active
IS 'Unique partial index on refresh_token_hash for O(1) session validation.';

-- ============================================================
-- 7. UserAchievement: (user_id, achievement_id) lookup
--    Query: WHERE user_id = $1 AND achievement_id = $2
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_achievement_user_achievement
ON public."UserAchievement" ("user_id", "achievement_id")
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_user_achievement_user_achievement
IS 'Composite index for per-user achievement lookup.';

-- ============================================================
-- 8. StudySession: user timeline (user_id + start_time DESC)
--    Query: WHERE user_id = $1 ORDER BY start_time DESC LIMIT n
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_study_session_user_start_time
ON public."StudySession" ("user_id", "start_time" DESC)
WHERE "deleted_at" IS NULL;

COMMENT ON INDEX public.idx_study_session_user_start_time
IS 'Composite partial index for per-user study session timeline queries.';

-- ============================================================
-- Update PostgreSQL optimizer statistics so the planner picks
-- the new indexes immediately after this migration runs.
-- ============================================================
ANALYZE public."User";
ANALYZE public."SubjectEnrollment";
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'Enrollment') THEN
        EXECUTE 'ANALYZE public."Enrollment"';
    END IF;
END $$;
ANALYZE public."ExamResult";
ANALYZE public."Notification";
ANALYZE public."UserSession";
ANALYZE public."UserAchievement";
ANALYZE public."StudySession";
