-- ============================================================
-- Migration 0044: Safe database optimization pass
-- ============================================================
-- Adds high-value indexes defensively. Each index checks table and column
-- existence first so databases with partial or older schemas can still move
-- forward without failing the whole migration chain.

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

-- Public course/catalog screens.
SELECT pg_temp.create_index_if_columns(
    'public."Subject"',
    ARRAY['deleted_at', 'created_at', 'id'],
    'CREATE INDEX IF NOT EXISTS idx_subject_created_active_safe ON public."Subject" (created_at DESC, id) WHERE deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."Subject"',
    ARRAY['deleted_at', 'is_published', 'is_active', 'level', 'category_id', 'created_at'],
    'CREATE INDEX IF NOT EXISTS idx_subject_public_catalog_safe ON public."Subject" (is_published, is_active, level, category_id, created_at DESC) WHERE deleted_at IS NULL'
);

-- Billing and payment history.
SELECT pg_temp.create_index_if_columns(
    'public."Payment"',
    ARRAY['user_id', 'created_at', 'deleted_at', 'amount', 'status'],
    'CREATE INDEX IF NOT EXISTS idx_payment_user_created_covering_safe ON public."Payment" (user_id, created_at DESC) INCLUDE (amount, status) WHERE deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."Payment"',
    ARRAY['user_id', 'status', 'created_at', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_payment_user_status_active_safe ON public."Payment" (user_id, status, created_at DESC) WHERE deleted_at IS NULL'
);

-- Notifications and activity polling. Supports both old is_read and newer
-- camel-case isRead schemas without assuming either one exists.
SELECT pg_temp.create_index_if_columns(
    'public."Notification"',
    ARRAY['user_id', 'created_at', 'deleted_at', 'id', 'type', 'title', 'message', 'is_read'],
    'CREATE INDEX IF NOT EXISTS idx_notification_user_created_read_safe ON public."Notification" (user_id, created_at DESC) INCLUDE (id, type, title, message, is_read) WHERE deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."Notification"',
    ARRAY['user_id', 'created_at', 'deleted_at', 'id', 'type', 'title', 'message', 'isRead'],
    'CREATE INDEX IF NOT EXISTS idx_notification_user_created_isread_safe ON public."Notification" (user_id, created_at DESC) INCLUDE (id, type, title, message, "isRead") WHERE deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."Notification"',
    ARRAY['user_id', 'created_at', 'deleted_at', 'is_read'],
    'CREATE INDEX IF NOT EXISTS idx_notification_user_unread_safe ON public."Notification" (user_id, created_at DESC) WHERE deleted_at IS NULL AND is_read = false'
);

SELECT pg_temp.create_index_if_columns(
    'public."Notification"',
    ARRAY['user_id', 'created_at', 'deleted_at', 'isRead'],
    'CREATE INDEX IF NOT EXISTS idx_notification_user_unread_isread_safe ON public."Notification" (user_id, created_at DESC) WHERE deleted_at IS NULL AND "isRead" = false'
);

-- Auth session refresh and session lists.
SELECT pg_temp.create_index_if_columns(
    'public."UserSession"',
    ARRAY['id', 'refresh_token_hash', 'is_active', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_session_refresh_hash_safe ON public."UserSession" (id, refresh_token_hash, is_active) WHERE is_active = true AND deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."UserSession"',
    ARRAY['user_id', 'is_active', 'last_accessed', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_session_user_active_recent_safe ON public."UserSession" (user_id, is_active, last_accessed DESC) WHERE deleted_at IS NULL'
);

-- User-facing history screens.
SELECT pg_temp.create_index_if_columns(
    'public."ExamResult"',
    ARRAY['user_id', 'taken_at', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_exam_result_user_taken_safe ON public."ExamResult" (user_id, taken_at DESC) WHERE deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."UserAchievement"',
    ARRAY['user_id', 'unlocked_at', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_user_achievement_user_unlocked_safe ON public."UserAchievement" (user_id, unlocked_at DESC) WHERE deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."StudySession"',
    ARRAY['user_id', 'start_time', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_study_session_user_start_safe ON public."StudySession" (user_id, start_time DESC) WHERE deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."Task"',
    ARRAY['user_id', 'status', 'created_at', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_task_user_status_created_safe ON public."Task" (user_id, status, created_at DESC) WHERE deleted_at IS NULL'
);

-- Forum compatibility endpoints.
SELECT pg_temp.create_index_if_columns(
    'public."ForumCategory"',
    ARRAY['order', 'created_at', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_forum_category_order_safe ON public."ForumCategory" ("order" ASC, created_at DESC) WHERE deleted_at IS NULL'
);

SELECT pg_temp.create_index_if_columns(
    'public."ForumTopic"',
    ARRAY['category_id', 'created_at', 'deleted_at'],
    'CREATE INDEX IF NOT EXISTS idx_forum_topic_category_created_safe ON public."ForumTopic" (category_id, created_at DESC) WHERE deleted_at IS NULL'
);

DO $$
DECLARE
    target_table text;
BEGIN
    FOREACH target_table IN ARRAY ARRAY[
        'public."Subject"',
        'public."Payment"',
        'public."Notification"',
        'public."UserSession"',
        'public."ExamResult"',
        'public."UserAchievement"',
        'public."StudySession"',
        'public."Task"',
        'public."ForumCategory"',
        'public."ForumTopic"'
    ] LOOP
        IF to_regclass(target_table) IS NOT NULL THEN
            EXECUTE format('ANALYZE %s', target_table);
        END IF;
    END LOOP;
END $$;
