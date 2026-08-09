-- ============================================================
-- Migration 0135: Admin dashboard query index optimizations
-- ============================================================
-- Adds indexes for dashboard list and aggregation queries that
-- commonly filter on created_at and soft-deleted rows.
--
-- These indexes help the admin dashboard avoid slow sequential
-- scans when returning the latest records from large tables.
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_lesson_created_at_deleted
    ON public."Lesson" ("created_at" DESC)
    WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_lesson_deleted_at
    ON public."Lesson" ("deleted_at");

CREATE INDEX IF NOT EXISTS idx_exam_created_at_deleted
    ON public."Exam" ("created_at" DESC)
    WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_lms_course_created_at_deleted
    ON public."LmsCourse" ("created_at" DESC)
    WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_payment_created_at_deleted
    ON public."Payment" ("created_at" DESC)
    WHERE "deleted_at" IS NULL;

ANALYZE public."Lesson";
ANALYZE public."Exam";
ANALYZE public."LmsCourse";
ANALYZE public."Payment";
