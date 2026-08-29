-- Migration 0156: Add CourseQuestion / CourseAnswer tables.
--
-- Student-facing Q&A, scoped to the legacy Subject/SubTopic model (the one
-- the education frontend actually reads courses/lessons/reviews from), not
-- the newer hexagonal LmsCourse system used only by the admin course
-- builder. Mirrors CourseReview's shape/conventions.

CREATE TABLE IF NOT EXISTS "CourseQuestion" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "subject_id"    UUID NOT NULL REFERENCES "Subject"("id") ON DELETE CASCADE,
    "sub_topic_id"  UUID REFERENCES "SubTopic"("id") ON DELETE CASCADE,
    "user_id"       UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "title"         TEXT NOT NULL,
    "body"          TEXT,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "CourseQuestion_subject_id_idx" ON "CourseQuestion" ("subject_id");
CREATE INDEX IF NOT EXISTS "CourseQuestion_sub_topic_id_idx" ON "CourseQuestion" ("sub_topic_id");
CREATE INDEX IF NOT EXISTS "CourseQuestion_user_id_idx" ON "CourseQuestion" ("user_id");
CREATE INDEX IF NOT EXISTS "CourseQuestion_created_at_idx" ON "CourseQuestion" ("created_at");
CREATE INDEX IF NOT EXISTS "CourseQuestion_deleted_at_idx" ON "CourseQuestion" ("deleted_at");

CREATE TABLE IF NOT EXISTS "CourseAnswer" (
    "id"                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "question_id"           UUID NOT NULL REFERENCES "CourseQuestion"("id") ON DELETE CASCADE,
    "user_id"               UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "body"                  TEXT,
    "is_instructor_answer"  BOOLEAN NOT NULL DEFAULT false,
    "created_at"            TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"            TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"            TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "CourseAnswer_question_id_idx" ON "CourseAnswer" ("question_id");
CREATE INDEX IF NOT EXISTS "CourseAnswer_user_id_idx" ON "CourseAnswer" ("user_id");
CREATE INDEX IF NOT EXISTS "CourseAnswer_deleted_at_idx" ON "CourseAnswer" ("deleted_at");
