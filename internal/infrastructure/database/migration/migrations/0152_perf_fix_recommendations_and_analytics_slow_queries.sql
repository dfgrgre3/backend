-- ============================================================
-- Migration 0152: Fix slow SQL queries for AI recommendations, gamification, analytics search history, and HTTP metrics upsert
-- ============================================================

-- 1. AnalyticsEvent: Fast lookup for search history & event querying by user & event type
CREATE INDEX IF NOT EXISTS idx_analytics_event_user_type_received
ON public."AnalyticsEvent" ("user_id", "event_type", "received_at" DESC);

-- 2. Subject: Covering/composite index for course suggestions & recommendations (sorting by rating & enrolled_count)
CREATE INDEX IF NOT EXISTS idx_subject_published_active_rating
ON public."Subject" ("deleted_at", "is_published", "is_active", "rating" DESC, "enrolled_count" DESC);

-- 3. SubjectEnrollment: Fast checking of enrolled user subjects for NOT EXISTS recommendation filter
CREATE INDEX IF NOT EXISTS idx_subject_enrollment_user_subject
ON public."SubjectEnrollment" ("user_id", "subject_id")
WHERE "deleted_at" IS NULL;

-- 4. http_metric_buckets: Unique composite index to ensure hyper-fast ON CONFLICT resolution during batch metric flushes
CREATE UNIQUE INDEX IF NOT EXISTS idx_http_metric_buckets_unique_bucket
ON public."http_metric_buckets" ("bucket_start", "route", "method", "status");

-- 5. User: High performance index for Leaderboard queries by XP
CREATE INDEX IF NOT EXISTS idx_user_status_total_xp
ON public."User" ("status", "total_xp" DESC)
WHERE "deleted_at" IS NULL;

-- 6. UserAchievement: Fast lookup for user achievements
CREATE INDEX IF NOT EXISTS idx_user_achievement_user_id
ON public."UserAchievement" ("user_id");

-- 7. Task: Fast lookup for overdue tasks in recommendations
CREATE INDEX IF NOT EXISTS idx_task_user_status_due
ON public."Task" ("user_id", "status", "due_at")
WHERE "deleted_at" IS NULL;

-- 8. ExamResult: Fast lookup for weak subjects calculation
CREATE INDEX IF NOT EXISTS idx_exam_result_user_taken
ON public."ExamResult" ("user_id", "taken_at" DESC)
WHERE "deleted_at" IS NULL;

-- 9. SubjectEnrollment: Fast lookup for stalled courses in recommendations
CREATE INDEX IF NOT EXISTS idx_subject_enrollment_user_progress_updated
ON public."SubjectEnrollment" ("user_id", "progress", "updated_at")
WHERE "deleted_at" IS NULL;

-- 10. StudySession: Fast lookup for user study consistency calculations
CREATE INDEX IF NOT EXISTS idx_study_session_user_start_time
ON public."StudySession" ("user_id", "start_time" DESC)
WHERE "deleted_at" IS NULL;
