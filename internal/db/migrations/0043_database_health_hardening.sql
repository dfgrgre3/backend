-- ============================================================
-- Migration 0043: Database health hardening
-- ============================================================
-- Applies safe, idempotent indexes for the current snake_case schema.
-- Every block checks table/column existence first so older databases with
-- partial schemas can still migrate forward.

BEGIN;

DO $$
BEGIN
    IF to_regclass('public."Subject"') IS NOT NULL THEN
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'Subject' AND column_name = 'deleted_at'
        ) THEN
            CREATE INDEX IF NOT EXISTS idx_subject_active_rows
                ON public."Subject" (id)
                WHERE deleted_at IS NULL;

            CREATE INDEX IF NOT EXISTS idx_subject_created_active
                ON public."Subject" (created_at DESC, id)
                WHERE deleted_at IS NULL;

            CREATE INDEX IF NOT EXISTS idx_subject_public_catalog_active
                ON public."Subject" (is_published, is_active, level, category_id, created_at DESC)
                WHERE deleted_at IS NULL;

            CREATE INDEX IF NOT EXISTS idx_subject_category_created_active
                ON public."Subject" (category_id, created_at DESC)
                WHERE deleted_at IS NULL;
        END IF;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public."Payment"') IS NOT NULL
       AND EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'Payment' AND column_name = 'user_id'
       ) THEN
        CREATE INDEX IF NOT EXISTS idx_payment_user_created_covering
            ON public."Payment" (user_id, created_at DESC)
            INCLUDE (amount, status)
            WHERE deleted_at IS NULL;

        CREATE INDEX IF NOT EXISTS idx_payment_user_status_active
            ON public."Payment" (user_id, status, created_at DESC)
            WHERE deleted_at IS NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public."Notification"') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_notification_user_created_covering
            ON public."Notification" (user_id, created_at DESC)
            INCLUDE (id, type, title, message, "isRead")
            WHERE deleted_at IS NULL;

        CREATE INDEX IF NOT EXISTS idx_notification_user_unread_active
            ON public."Notification" (user_id, created_at DESC)
            WHERE deleted_at IS NULL AND "isRead" = false;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public."ExamResult"') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_exam_result_user_taken_active
            ON public."ExamResult" (user_id, taken_at DESC)
            WHERE deleted_at IS NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public."UserAchievement"') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_achievement_user_unlocked_active
            ON public."UserAchievement" (user_id, unlocked_at DESC)
            INCLUDE ("achievementKey")
            WHERE deleted_at IS NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public."UserSession"') IS NOT NULL THEN
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'UserSession' AND column_name = 'refresh_token_hash'
        ) THEN
            CREATE INDEX IF NOT EXISTS idx_user_sessions_active_refresh_hash
                ON public."UserSession" (id, refresh_token_hash, is_active)
                WHERE is_active = true AND deleted_at IS NULL;
        ELSIF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'UserSession' AND column_name = 'refresh_token'
        ) THEN
            CREATE INDEX IF NOT EXISTS idx_user_sessions_active_refresh_token
                ON public."UserSession" (id, refresh_token, is_active)
                WHERE is_active = true AND deleted_at IS NULL;
        END IF;

        CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active_recent
            ON public."UserSession" (user_id, is_active, last_accessed DESC)
            WHERE deleted_at IS NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public."ForumCategory"') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_forum_category_order_active
            ON public."ForumCategory" ("order" ASC, created_at DESC)
            WHERE deleted_at IS NULL;
    END IF;

    IF to_regclass('public."ForumTopic"') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_forum_topic_pinned_created_active
            ON public."ForumTopic" (is_pinned DESC, created_at DESC)
            INCLUDE (id, title, author_id, category_id, views, is_locked)
            WHERE deleted_at IS NULL;

        CREATE INDEX IF NOT EXISTS idx_forum_topic_category_created_active
            ON public."ForumTopic" (category_id, created_at DESC)
            WHERE deleted_at IS NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public."Task"') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_task_user_status_active_created
            ON public."Task" (user_id, status, created_at DESC)
            WHERE deleted_at IS NULL;
    END IF;

    IF to_regclass('public."StudySession"') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_study_session_user_start_active
            ON public."StudySession" (user_id, start_time DESC)
            WHERE deleted_at IS NULL;
    END IF;
END $$;

DO $$
DECLARE
    table_name text;
    tables text[] := ARRAY[
        'Subject',
        'Payment',
        'Notification',
        'ExamResult',
        'UserAchievement',
        'UserSession',
        'ForumCategory',
        'ForumTopic',
        'Task',
        'StudySession'
    ];
BEGIN
    FOREACH table_name IN ARRAY tables LOOP
        IF to_regclass(format('public.%I', table_name)) IS NOT NULL THEN
            EXECUTE format('ANALYZE public.%I', table_name);
        END IF;
    END LOOP;
END $$;

COMMIT;
