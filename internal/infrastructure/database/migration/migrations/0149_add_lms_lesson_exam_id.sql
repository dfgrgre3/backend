-- Migration 0149: Add exam_id to LmsLesson.
--
-- Links a lesson (hexagonal Course/Section/Lesson model) to a single exam
-- from the legacy Subject-scoped Exam table. This is a loose reference (no
-- FK constraint, matching the equivalent SubTopic.exam_id column) since Exam
-- rows live outside the Course/Section/Lesson hierarchy entirely.

ALTER TABLE "LmsLesson"
    ADD COLUMN IF NOT EXISTS "exam_id" UUID;

CREATE INDEX IF NOT EXISTS "LmsLesson_exam_id_idx" ON "LmsLesson" ("exam_id");
