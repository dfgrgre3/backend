-- Migration 0150: Add LmsAssignment table.
--
-- A course-scoped assignment/homework catalog for the hexagonal Course model.
-- lesson_id is nullable: an assignment can exist unlinked (just part of the
-- course's assignment bank) or be linked to exactly one lesson at a time.

CREATE TABLE IF NOT EXISTS "LmsAssignment" (
    "id"          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"   UUID NOT NULL REFERENCES "LmsCourse"("id") ON DELETE CASCADE,
    "lesson_id"   UUID REFERENCES "LmsLesson"("id") ON DELETE SET NULL,
    "title"       TEXT NOT NULL,
    "description" TEXT,
    "due_date"    TIMESTAMPTZ,
    "max_score"   NUMERIC(10,2) NOT NULL DEFAULT 100,
    "created_at"  TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"  TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "LmsAssignment_course_id_idx" ON "LmsAssignment" ("course_id");
CREATE INDEX IF NOT EXISTS "LmsAssignment_lesson_id_idx" ON "LmsAssignment" ("lesson_id");
CREATE INDEX IF NOT EXISTS "LmsAssignment_deleted_at_idx" ON "LmsAssignment" ("deleted_at");
