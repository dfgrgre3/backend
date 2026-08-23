-- Migration 0147: Add LessonNoteContent table.
--
-- The video player's notes panel (src/components/video/player/hooks/useTimelineNotes.ts)
-- persists ALL of a user's notes for one lesson as a single serialized text
-- blob (free-form text + a delimited block of "[hh:mm:ss] note text" lines),
-- fetched and overwritten whole on every save — not one row per note.
--
-- "LmsVideoNote" (0112) has the opposite shape (one row per timestamped
-- note) and is FK'd into a different, currently-unused lesson schema, so it
-- doesn't fit this feature. This is a small dedicated table matching what
-- the frontend actually sends.

CREATE TABLE IF NOT EXISTS "LessonNoteContent" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id"    UUID NOT NULL,
    "lesson_id"  UUID NOT NULL,
    "content"    TEXT NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS "LessonNoteContent_user_lesson_key"
    ON "LessonNoteContent" ("user_id", "lesson_id");
