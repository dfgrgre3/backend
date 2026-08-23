-- Migration 0148: Add LessonTranscript table.
--
-- Stores an admin/instructor-uploaded SRT or VTT transcript per lesson, used
-- by the video player's searchable-transcript panel (click a line to seek).
-- One transcript per lesson — re-uploading replaces the previous content.

CREATE TABLE IF NOT EXISTS "LessonTranscript" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "lesson_id"  UUID NOT NULL,
    "format"     TEXT NOT NULL DEFAULT 'srt',
    "content"    TEXT NOT NULL,
    "language"   TEXT NOT NULL DEFAULT 'ar',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS "LessonTranscript_lesson_id_key" ON "LessonTranscript" ("lesson_id");
