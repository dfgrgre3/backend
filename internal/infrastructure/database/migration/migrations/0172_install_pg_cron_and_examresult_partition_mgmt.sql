-- =============================================================
-- 0172: ExamResult partition management (pg_cron-optional)
-- -------------------------------------------------------------
-- Audit §8: ExamResult is monthly-partitioned with a DEFAULT
-- catch-all. Baseline migration created partitions named
-- 'examresult_pYYYY_MM' with Africa/Cairo (UTC+2/+3) timezone-
-- aware bounds, plus a 'examresult_pdefault' DEFAULT partition.
--
-- This migration:
--   1) Defines an idempotent helper function that follows the
--      baseline naming + TZ-aware convention.
--   2) Schedules via pg_cron IF the extension is available.
--   3) Does NOT fail if pg_cron is missing — the function is
--      available for the application to call on a schedule
--      (Asynq scheduler or external cron).
-- =============================================================

-- 1) Idempotent function: call from anywhere to ensure the next
--    N months of partitions exist. Safe to re-run.
--    Uses Africa/Cairo TZ to match the baseline partitions and
--    the 'examresult_pYYYY_MM' naming convention.
CREATE OR REPLACE FUNCTION ensure_examresult_partitions(months_ahead int DEFAULT 2)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    -- Use Cairo local time so bounds match the baseline partitions
    -- (Africa/Cairo is UTC+2 in winter, UTC+3 in summer — DST).
    tz          text := 'Africa/Cairo';
    start_local date;
    cur_local   date;
    i int;
    p_start timestamp;
    p_end   timestamp;
    p_name text;
    exists_already bool;
BEGIN
    -- Anchor to the 1st of the CURRENT month at 00:00 in Cairo TZ.
    cur_local   := (CURRENT_TIMESTAMP AT TIME ZONE tz)::date;
    start_local := date_trunc('month', cur_local)::date;

    FOR i IN 1..months_ahead LOOP
        -- Compute month-start and next-month-start timestamps in
        -- Cairo local time, expressed as UTC timestamps with the
        -- +02/+03 offset string so pg_get_expr comparison matches
        -- the baseline output.
        p_start := ((start_local + (i || ' months')::interval)::timestamp
                    AT TIME ZONE tz) AT TIME ZONE tz;
        p_end   := ((start_local + ((i+1) || ' months')::interval)::timestamp
                    AT TIME ZONE tz) AT TIME ZONE tz;
        p_name  := 'examresult_p' || to_char(p_start AT TIME ZONE tz, 'YYYY_MM');

        -- Skip if a partition with this exact canonical name already
        -- exists (covers baseline naming AND idempotency).
        SELECT EXISTS (
            SELECT 1 FROM pg_inherits inh
            JOIN pg_class c ON c.oid = inh.inhrelid
            WHERE inh.inhparent = 'public."ExamResult"'::regclass
              AND c.relname = p_name
        ) INTO exists_already;

        IF NOT exists_already THEN
            -- Also skip if ANY partition covers the same range
            -- (e.g. baseline partition already exists with same
            -- bound but a different name).
            SELECT EXISTS (
                SELECT 1 FROM pg_inherits inh
                JOIN pg_class c ON c.oid = inh.inhrelid
                WHERE inh.inhparent = 'public."ExamResult"'::regclass
                  AND pg_get_expr(c.relpartbound, c.oid) =
                      format('FOR VALUES FROM (%L) TO (%L)', p_start, p_end)
            ) INTO exists_already;

            IF NOT exists_already THEN
                EXECUTE format(
                    'CREATE TABLE IF NOT EXISTS public.%I
                     PARTITION OF public."ExamResult"
                     FOR VALUES FROM (%L) TO (%L)',
                    p_name, p_start, p_end
                );
            END IF;
        END IF;
    END LOOP;
END $$;

GRANT EXECUTE ON FUNCTION ensure_examresult_partitions TO app_user;

-- 2) Run the function once now to provision the next 2 months.
SELECT ensure_examresult_partitions(2);

-- 3) Schedule via pg_cron IF the extension is available.
--    We DO NOT fail the migration if pg_cron is missing.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.schedule(
            'examresult_partition_rollup',
            '0 3 25 * *',
            $cmd$ SELECT ensure_examresult_partitions(2); $cmd$
        );
        RAISE NOTICE 'pg_cron job scheduled: examresult_partition_rollup';
    ELSE
        RAISE NOTICE 'pg_cron not installed — function available for manual/scheduler invocation';
        RAISE NOTICE 'Install with: apk add postgresql-cron && echo "shared_preload_libraries = ''pg_cron''" >> postgresql.conf && restart';
    END IF;
END $$;

-- 4) Sanity check: list current partitions.
DO $$
DECLARE
    part_count int;
BEGIN
    SELECT count(*) INTO part_count
    FROM pg_inherits
    WHERE inhparent = 'public."ExamResult"'::regclass;

    RAISE NOTICE 'ExamResult now has % partition(s)', part_count;
END $$;

-- Done.