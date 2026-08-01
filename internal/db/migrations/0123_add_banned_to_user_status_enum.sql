-- 0123_add_banned_to_user_status_enum.sql
-- Add BANNED value to UserStatus enum type

ALTER TYPE public."UserStatus" ADD VALUE IF NOT EXISTS 'BANNED';
