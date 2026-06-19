-- Add Lesson table for scheduled teacher bookings
CREATE TABLE IF NOT EXISTS "Lesson" (
    "id" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id" uuid NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "teacher_id" uuid NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "title" text NOT NULL,
    "location" text NOT NULL,
    "start_time" timestamptz NOT NULL,
    "end_time" timestamptz NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "updated_at" timestamptz NOT NULL DEFAULT now(),
    "deleted_at" timestamptz
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS "idx_lessons_user" ON "Lesson"("user_id");
CREATE INDEX IF NOT EXISTS "idx_lessons_teacher" ON "Lesson"("teacher_id");
CREATE INDEX IF NOT EXISTS "idx_lessons_start" ON "Lesson"("start_time");
CREATE INDEX IF NOT EXISTS "idx_lessons_end" ON "Lesson"("end_time");

-- Composite index for fetching user's upcoming lessons
CREATE INDEX IF NOT EXISTS "idx_lessons_user_start" ON "Lesson"("user_id", "start_time");