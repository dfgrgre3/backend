-- 0124_add_instructor_specialties_and_languages.sql
-- Add missing instructor-related columns to User table

-- Add instructor_specialties column (JSONB array for storing specialties)
ALTER TABLE "User" 
ADD COLUMN IF NOT EXISTS instructor_specialties JSONB DEFAULT '[]'::jsonb;

-- Add instructor_languages column (JSONB array for storing languages)
ALTER TABLE "User" 
ADD COLUMN IF NOT EXISTS instructor_languages JSONB DEFAULT '[]'::jsonb;

-- Add commission_rate column (numeric with 4 decimal places)
ALTER TABLE "User" 
ADD COLUMN IF NOT EXISTS commission_rate NUMERIC(5,4) DEFAULT 0;