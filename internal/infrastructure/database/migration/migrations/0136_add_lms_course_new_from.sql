-- 0136_add_lms_course_new_from.sql
-- Adds the missing new_from column to LmsCourse that is referenced by the
-- Go model (lms.go) but was omitted from migration 0112.

ALTER TABLE "LmsCourse"
    ADD COLUMN IF NOT EXISTS "new_from" TIMESTAMPTZ;
