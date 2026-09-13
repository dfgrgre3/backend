-- 0193_fix_enrollment_cascades_and_matview_refresh.sql
--
-- Continues the DB review (2026-09-11):
--   6. SubjectEnrollment carried two parallel soft-delete markers
--      ("isDeleted" and deleted_at). Only deleted_at is used anywhere in
--      Go code or in the ActiveEnrollments view (0049); "isDeleted" is
--      dead. Drop it, and switch the Subject/User cascades on
--      SubjectEnrollment to RESTRICT so removing a Subject/User can no
--      longer silently wipe paid enrollment history.
--   4. Two independent, unsynchronized schedulers were refreshing the
--      same materialized views (pg_cron job from 0174 + the asynq
--      "refresh-materialized-views" worker in
--      internal/infrastructure/workers/scheduler.go, every 5m). Keep the
--      asynq worker (finer-grained, app-controlled, already the one
--      actually relied on) and drop the redundant pg_cron job.

BEGIN;

-- ── 6a. Drop dead "isDeleted" column + its indexes on SubjectEnrollment ─

DROP INDEX IF EXISTS public."SubjectEnrollment_active_enrollment_idx";
DROP INDEX IF EXISTS public."SubjectEnrollment_isDeleted_idx";

ALTER TABLE public."SubjectEnrollment"
    DROP COLUMN IF EXISTS "isDeleted";

-- Replacement for the unique-active-enrollment guard that isDeleted used
-- to provide, now expressed against deleted_at (the column actually used
-- by the ActiveEnrollments view and all application code).
CREATE UNIQUE INDEX IF NOT EXISTS "SubjectEnrollment_active_enrollment_idx"
    ON public."SubjectEnrollment" USING btree (user_id, subject_id)
    WHERE (deleted_at IS NULL);

-- ── 6b. Cascade -> Restrict on SubjectEnrollment's Subject/User FKs ─────
-- Also collapses the duplicate Subject FK (fk_Subject_enrollments and
-- fk_subject_enrollments_subject_id both target subject_id) into one.

ALTER TABLE public."SubjectEnrollment"
    DROP CONSTRAINT IF EXISTS "fk_Subject_enrollments",
    DROP CONSTRAINT IF EXISTS "fk_subject_enrollments_subject_id",
    DROP CONSTRAINT IF EXISTS "fk_User_enrollments";

ALTER TABLE public."SubjectEnrollment"
    ADD CONSTRAINT "fk_subject_enrollments_subject_id"
    FOREIGN KEY (subject_id) REFERENCES public."Subject"(id) ON DELETE RESTRICT;

ALTER TABLE public."SubjectEnrollment"
    ADD CONSTRAINT "fk_User_enrollments"
    FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE RESTRICT;

-- ── 4. Remove the redundant pg_cron matview refresh job ────────────────
-- The asynq "refresh-materialized-views" worker (every 5m, see
-- internal/infrastructure/workers/scheduler.go and
-- internal/application/cqrs/readmodel_refresher.go) already refreshes
-- mv_user_weekly_analytics, mv_user_progress_summary and
-- mv_user_watch_time. Unschedule the daily 04:00 UTC pg_cron duplicate
-- added in 0174_per_table_autovacuum_tuning.sql to stop the two jobs
-- from racing REFRESH CONCURRENTLY against each other.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.unschedule(jobid)
        FROM cron.job
        WHERE jobname = 'refresh_materialized_views';
    END IF;
END;
$$;

COMMIT;
