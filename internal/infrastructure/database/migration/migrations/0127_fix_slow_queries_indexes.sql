-- ============================================================
-- Migration 0127: Fix slow SQL queries reported in production logs
-- ============================================================
-- Addresses the following slow queries (>= 500ms):
--   1. MailTask worker poll (4511ms)   - status IN ('pending','retry') AND deleted_at IS NULL ORDER BY created_at ASC
--   2. SystemSetting key lookup (581ms) - WHERE key = $1 AND deleted_at IS NULL
--   3. User email login lookup (1444ms) - WHERE email = $1 AND deleted_at IS NULL ORDER BY id LIMIT 1
--   4. UserSession active count (1014ms) - WHERE user_id = $1 AND is_active = $2 AND deleted_at IS NULL
--   5. UserSession INSERT (1311ms)      - triggered by BeforeCreate hook count query
--   6. login_history INSERT (627ms)     - missing user_id index
--   7. Notification fetch (1214ms)      - WHERE user_id = $1 ORDER BY created_at DESC LIMIT n
--   8. Task batch count (766ms)         - WHERE user_id IN (...) AND deleted_at IS NULL GROUP BY user_id
--   9. StudySession batch count (1117ms)- WHERE user_id IN (...) AND deleted_at IS NULL GROUP BY user_id
--
-- NOTE: CREATE INDEX CONCURRENTLY is intentionally NOT used here because the
-- migration framework wraps statements in a transaction. For zero-downtime
-- production deployments, create these indexes manually with CONCURRENTLY
-- outside of the migration tool.

-- ============================================================
-- 1. MailTask: worker poll query
--    Query: WHERE status IN ('pending','retry') AND deleted_at IS NULL ORDER BY created_at ASC LIMIT 100
--    Existing index from 0118/0126 may not have been applied; re-create with IF NOT EXISTS.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_mail_task_status_deleted_created
ON public."MailTask" ("status", "created_at" ASC)
WHERE "deleted_at" IS NULL AND "status" IN ('pending', 'retry');

-- Covering index to avoid heap fetch for the worker's SELECT columns
CREATE INDEX IF NOT EXISTS idx_mail_task_worker_covering
ON public."MailTask" ("created_at" ASC)
INCLUDE ("id", "to", "subject", "body", "status", "attempts", "max_retry", "last_error", "updated_at")
WHERE "deleted_at" IS NULL AND "status" IN ('pending', 'retry');

-- ============================================================
-- 2. SystemSetting: key lookup
--    Query: WHERE key = $1 AND deleted_at IS NULL LIMIT 1
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_system_setting_key_deleted
ON public."SystemSetting" ("key")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 3. User: email login lookup
--    Query: WHERE email = $1 AND deleted_at IS NULL ORDER BY id LIMIT 1
--    The unique index on email does not include the deleted_at filter.
--    A partial index allows the planner to skip soft-deleted rows.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_email_active
ON public."User" ("email")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 4. UserSession: active session count (used in BeforeCreate hook + login)
--    Query: WHERE user_id = $1 AND is_active = $2 AND deleted_at IS NULL
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_session_user_active
ON public."UserSession" ("user_id", "is_active", "last_accessed" ASC)
WHERE "deleted_at" IS NULL AND "is_active" = true;

-- ============================================================
-- 5. UserSession: refresh token hash lookup (auth token rotation)
--    Query: WHERE refresh_token_hash = $1 AND is_active = $2
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_session_refresh_active
ON public."UserSession" ("refresh_token_hash", "is_active")
WHERE "deleted_at" IS NULL AND "is_active" = true;

-- ============================================================
-- 6. UserSession: user_id + created_at (for replacement session lookup in refresh flow)
--    Query: WHERE user_id = $1 AND is_active = $2 AND created_at >= $3 ORDER BY created_at DESC
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_session_user_created
ON public."UserSession" ("user_id", "is_active", "created_at" DESC)
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 7. login_history: user_id index (INSERT performance + login history lookups)
--    The table is named "login_history" (lowercase) per the model.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_login_history_user_id
ON public."login_history" ("user_id");

CREATE INDEX IF NOT EXISTS idx_login_history_user_created
ON public."login_history" ("user_id", "created_at" DESC);

-- ============================================================
-- 8. Notification: user fetch with ORDER BY created_at DESC
--    Query: WHERE user_id = $1 ORDER BY created_at DESC LIMIT n
--    Add a covering index for the handler's selected columns.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_notification_user_created
ON public."Notification" ("user_id", "created_at" DESC);

CREATE INDEX IF NOT EXISTS idx_notification_user_created_covering
ON public."Notification" ("user_id", "created_at" DESC)
INCLUDE ("id", "title", "message", "type", "is_read", "link", "icon");

-- Index for unread notification count: WHERE user_id = $1 AND is_read = false
CREATE INDEX IF NOT EXISTS idx_notification_user_unread
ON public."Notification" ("user_id")
WHERE "is_read" = false;

-- ============================================================
-- 9. Task: batch count by user_id (admin users list)
--    Query: WHERE user_id IN (...) AND deleted_at IS NULL GROUP BY user_id
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_task_user_deleted
ON public."Task" ("user_id")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 10. StudySession: batch count by user_id (admin users list)
--     Query: WHERE user_id IN (...) AND deleted_at IS NULL GROUP BY user_id
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_study_session_user_deleted
ON public."StudySession" ("user_id")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 11. UserAchievement: batch count by user_id (admin users list)
--     Query: WHERE user_id IN (...) AND deleted_at IS NULL GROUP BY user_id
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_achievement_user_deleted
ON public."UserAchievement" ("user_id")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 12. SubjectEnrollment: batch count by user_id (admin users list)
--     Query: WHERE user_id IN (...) AND deleted_at IS NULL GROUP BY user_id
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_enrollment_user_deleted
ON public."SubjectEnrollment" ("user_id")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 13. UserCredential: user_id lookup (login flow)
--     Query: WHERE user_id = $1
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_credential_user
ON public."UserCredential" ("user_id")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- 14. VerificationCode: user + type + is_used lookup (auth flow)
--     Query: WHERE user_id = $1 AND type = $2 AND code = $3 AND is_used = $4
--     NOTE: verification_codes table does not have a deleted_at column.
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_verification_code_user_type_used
ON public."verification_codes" ("user_id", "type", "is_used", "expires_at");

-- ============================================================
-- 15. PasswordResetToken: token hash lookup (if table exists)
--     Query: WHERE token_hash = $1 AND is_used = $2 AND expires_at > $3
-- ============================================================
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'password_reset_tokens') THEN
        CREATE INDEX IF NOT EXISTS idx_password_reset_token_hash_active
        ON public."password_reset_tokens" ("token_hash", "is_used", "expires_at")
        WHERE "is_used" = false;
    END IF;
END $$;

-- ============================================================
-- 16. Additional performance indexes for slow queries logged in admin endpoints:
--     User (status, created_at, deleted_at)
--     Task (status, deleted_at)
--     Exam, Subject, ExamResult, UserAchievement (deleted_at)
--     SubTopic (type, deleted_at)
--     StudySession (duration_min, deleted_at)
--     Notification (is_read)
--     Challenge (is_active, deleted_at)
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_user_status_deleted
ON public."User" ("status")
WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_created_at_desc_deleted
ON public."User" ("created_at" DESC)
WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_deleted_at
ON public."User" ("deleted_at");

CREATE INDEX IF NOT EXISTS idx_task_status_deleted
ON public."Task" ("status")
WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_exam_deleted_at
ON public."Exam" ("deleted_at");

CREATE INDEX IF NOT EXISTS idx_subtopic_type_deleted
ON public."SubTopic" ("type")
WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_subject_deleted_at
ON public."Subject" ("deleted_at");

CREATE INDEX IF NOT EXISTS idx_examresult_deleted_at
ON public."ExamResult" ("deleted_at");

CREATE INDEX IF NOT EXISTS idx_userachievement_deleted_at
ON public."UserAchievement" ("deleted_at");

CREATE INDEX IF NOT EXISTS idx_studysession_duration_deleted
ON public."StudySession" ("duration_min")
WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_notification_is_read
ON public."Notification" ("is_read");

CREATE INDEX IF NOT EXISTS idx_challenge_is_active_deleted
ON public."Challenge" ("is_active")
WHERE "deleted_at" IS NULL;

-- ============================================================
-- Update optimizer stats for all affected tables so the planner
-- picks the new indexes immediately.
-- ============================================================
ANALYZE public."MailTask";
ANALYZE public."SystemSetting";
ANALYZE public."User";
ANALYZE public."UserSession";
ANALYZE public."login_history";
ANALYZE public."Notification";
ANALYZE public."Task";
ANALYZE public."StudySession";
ANALYZE public."UserAchievement";
ANALYZE public."SubjectEnrollment";
ANALYZE public."UserCredential";
ANALYZE public."verification_codes";
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'password_reset_tokens') THEN
        EXECUTE 'ANALYZE public."password_reset_tokens"';
    END IF;
END $$;
ANALYZE public."Exam";
ANALYZE public."SubTopic";
ANALYZE public."Subject";
ANALYZE public."ExamResult";
ANALYZE public."Challenge";