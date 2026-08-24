-- ============================================================
-- Migration 0153: Add city, gender, and school columns to User for profile settings
-- ============================================================

ALTER TABLE public."User" ADD COLUMN IF NOT EXISTS "city" varchar(120);
ALTER TABLE public."User" ADD COLUMN IF NOT EXISTS "gender" varchar(10);
ALTER TABLE public."User" ADD COLUMN IF NOT EXISTS "school" varchar(200);
