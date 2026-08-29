-- ============================================================
-- Migration 0160: Relax NOT NULL constraints on Schedule table
-- ============================================================
-- The Schedule table (0000_baseline_schema.sql) was created for a
-- per-event calendar model: title, startTime, endTime, type, active
-- are all NOT NULL with no defaults on title/startTime/endTime.
--
-- The application (GetSchedule/UpdateSchedule in
-- internal/infrastructure/api/handlers/protected/activity_handler.go,
-- using models.Schedule in internal/domain/common/activity.go) instead
-- uses this table as a single row per user holding a JSON blob
-- (user_id, plan_json). It never sets title/startTime/endTime, which
-- caused every POST /api/schedule to fail its INSERT with:
--   ERROR: null value in column "title" of relation "Schedule"
--          violates not-null constraint (SQLSTATE 23502)
-- silently swallowed by SafeCreate (error return not checked), so the
-- handler still responded 200 with no row ever persisted, and every
-- schedule fetch/save was retried from scratch.
--
-- This migration relaxes the NOT NULL constraints that the current
-- application code cannot satisfy, aligning the schema with actual
-- usage. No data is dropped; existing rows are untouched.
-- ============================================================

ALTER TABLE public."Schedule" ALTER COLUMN title DROP NOT NULL;
ALTER TABLE public."Schedule" ALTER COLUMN "startTime" DROP NOT NULL;
ALTER TABLE public."Schedule" ALTER COLUMN "endTime" DROP NOT NULL;
ALTER TABLE public."Schedule" ALTER COLUMN type DROP NOT NULL;
ALTER TABLE public."Schedule" ALTER COLUMN active DROP NOT NULL;

-- Also drop the redundant not-null-name constraint on user_id if it
-- duplicates the column's own NOT NULL (kept as-is; user_id is always
-- provided by the app, so no change needed there).
