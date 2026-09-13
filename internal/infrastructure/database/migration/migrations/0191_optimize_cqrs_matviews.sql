-- 0191: Avoid per-user LATERAL rescans during CQRS materialized-view refreshes.
--
-- The previous definitions executed one aggregate subquery per user for each
-- source table. These definitions aggregate each source table once and join
-- the results to User, which scales with the source-table cardinality rather
-- than with users multiplied by source-table scans.

DROP MATERIALIZED VIEW IF EXISTS public.mv_user_progress_summary;
DROP MATERIALIZED VIEW IF EXISTS public.mv_user_weekly_analytics;

CREATE MATERIALIZED VIEW public.mv_user_progress_summary AS
WITH topic_progress AS (
    SELECT
        user_id,
        COUNT(*) FILTER (WHERE completed) AS lessons_completed,
        COALESCE(SUM(time_spent_seconds), 0) AS total_time_seconds,
        COUNT(*) FILTER (WHERE status = 'IN_PROGRESS') AS in_progress_count
    FROM public."TopicProgress"
    WHERE deleted_at IS NULL
    GROUP BY user_id
), study_sessions AS (
    SELECT
        user_id,
        COALESCE(SUM(duration_min), 0) AS weekly_minutes,
        COUNT(*) AS weekly_sessions,
        COALESCE(AVG(focus_score), 0) AS weekly_avg_focus
    FROM public."StudySession"
    WHERE start_time >= NOW() - INTERVAL '7 days'
    GROUP BY user_id
), exam_results AS (
    SELECT
        user_id,
        COUNT(*) AS total_exams_taken,
        COUNT(*) FILTER (WHERE passed) AS total_exams_passed
    FROM public."ExamResult"
    GROUP BY user_id
), enrollments AS (
    SELECT
        user_id,
        COUNT(*) AS active_subjects
    FROM public."SubjectEnrollment"
    WHERE deleted_at IS NULL
    GROUP BY user_id
)
SELECT
    u.id AS user_id,
    u.total_xp AS "totalXP",
    u.level,
    u.current_streak AS "currentStreak",
    u.longest_streak AS "longestStreak",
    u.total_study_time AS "totalStudyTime",
    u.tasks_completed AS "tasksCompleted",
    u.exams_passed AS "examsPassed",
    COALESCE(tp.lessons_completed, 0) AS lessons_completed,
    COALESCE(tp.total_time_seconds, 0) AS total_time_seconds,
    COALESCE(tp.in_progress_count, 0) AS in_progress_count,
    COALESCE(ss.weekly_minutes, 0) AS weekly_study_minutes,
    COALESCE(ss.weekly_sessions, 0) AS weekly_sessions,
    COALESCE(ss.weekly_avg_focus, 0) AS weekly_avg_focus,
    COALESCE(er.total_exams_taken, 0) AS total_exams_taken,
    COALESCE(er.total_exams_passed, 0) AS total_exams_passed,
    COALESCE(enr.active_subjects, 0) AS active_subjects,
    NOW() AS computed_at
FROM public."User" u
LEFT JOIN topic_progress tp ON tp.user_id = u.id
LEFT JOIN study_sessions ss ON ss.user_id = u.id
LEFT JOIN exam_results er ON er.user_id = u.id
LEFT JOIN enrollments enr ON enr.user_id = u.id
WHERE u.deleted_at IS NULL;

CREATE UNIQUE INDEX idx_mv_progress_user_id
    ON public.mv_user_progress_summary (user_id);

CREATE MATERIALIZED VIEW public.mv_user_weekly_analytics AS
WITH study_sessions AS (
    SELECT
        user_id,
        COALESCE(SUM(duration_min), 0) AS total_study_minutes,
        COUNT(*) AS total_sessions,
        COUNT(DISTINCT DATE(start_time)) AS active_days
    FROM public."StudySession"
    WHERE start_time >= NOW() - INTERVAL '7 days'
    GROUP BY user_id
), tasks AS (
    SELECT
        user_id,
        COUNT(*) AS total_tasks,
        COUNT(*) FILTER (WHERE status = 'COMPLETED') AS completed_tasks
    FROM public."Task"
    WHERE created_at >= NOW() - INTERVAL '7 days'
       OR updated_at >= NOW() - INTERVAL '7 days'
    GROUP BY user_id
)
SELECT
    u.id AS user_id,
    COALESCE(ss.total_study_minutes, 0) AS total_study_minutes,
    COALESCE(ss.total_sessions, 0) AS total_sessions,
    COALESCE(ss.active_days, 0) AS active_days,
    COALESCE(tsk.total_tasks, 0) AS total_tasks,
    COALESCE(tsk.completed_tasks, 0) AS completed_tasks,
    CASE
        WHEN COALESCE(tsk.total_tasks, 0) > 0
        THEN ROUND(tsk.completed_tasks::numeric / tsk.total_tasks * 100, 1)
        ELSE 0
    END AS completion_rate,
    0 AS weekly_xp_earned,
    NOW() AS computed_at
FROM public."User" u
LEFT JOIN study_sessions ss ON ss.user_id = u.id
LEFT JOIN tasks tsk ON tsk.user_id = u.id
WHERE u.deleted_at IS NULL;

CREATE UNIQUE INDEX idx_mv_weekly_user_id
    ON public.mv_user_weekly_analytics (user_id);

