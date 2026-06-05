-- Migration: 0051_fix_course_review_schema.sql
-- Description: Drop NOT NULL constraint on camelCase columns in CourseReview for GORM compatibility

ALTER TABLE IF EXISTS public."CourseReview" ALTER COLUMN "subjectId" DROP NOT NULL;
ALTER TABLE IF EXISTS public."CourseReview" ALTER COLUMN "userId" DROP NOT NULL;
