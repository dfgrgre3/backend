-- Migration: 0052_course_review_comments.sql
-- Description: Public comments attached to student course reviews.

CREATE TABLE IF NOT EXISTS public."StudentCourseReviewComment" (
    "id" uuid PRIMARY KEY,
    "review_id" uuid NOT NULL REFERENCES public."CourseReview"("id") ON DELETE CASCADE,
    "user_id" uuid NOT NULL REFERENCES public."User"("id") ON DELETE CASCADE,
    "comment" text NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "updated_at" timestamptz NOT NULL DEFAULT now(),
    "deleted_at" timestamptz NULL
);

CREATE INDEX IF NOT EXISTS "idx_course_review_comment_review_id"
    ON public."StudentCourseReviewComment" ("review_id");
CREATE INDEX IF NOT EXISTS "idx_course_review_comment_user_id"
    ON public."StudentCourseReviewComment" ("user_id");
