-- Migration: 0054_fix_user_permissions_jsonb.sql
-- Description: Convert public."User".permissions from text to jsonb with standard defaults

ALTER TABLE public."User" ALTER COLUMN "permissions" DROP DEFAULT;
ALTER TABLE public."User" ALTER COLUMN "permissions" TYPE jsonb USING to_jsonb("permissions");
ALTER TABLE public."User" ALTER COLUMN "permissions" SET DEFAULT '[]'::jsonb;
