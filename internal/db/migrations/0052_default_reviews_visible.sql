-- Migration: 0052_default_reviews_visible.sql
-- Description: Update existing course reviews to be visible

UPDATE public."CourseReview" 
SET is_visible = true, 
    "isVisible" = true 
WHERE is_visible = false OR "isVisible" = false OR is_visible IS NULL OR "isVisible" IS NULL;
