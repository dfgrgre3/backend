-- Migration: 0054_fix_user_permissions_jsonb.sql
-- Description: Convert public."User".permissions from text to jsonb with standard defaults

-- Drop ALL views that might depend on the User table
DO $$ 
DECLARE 
    view_record RECORD;
BEGIN 
    FOR view_record IN 
        SELECT table_name 
        FROM information_schema.views 
        WHERE table_schema = 'public' 
        AND view_definition ILIKE '%User%' 
    LOOP 
        EXECUTE 'DROP VIEW IF EXISTS public."' || view_record.table_name || '" CASCADE'; 
        RAISE NOTICE 'Dropped view: %', view_record.table_name;
    END LOOP; 
END $$;

ALTER TABLE public."User" ALTER COLUMN "permissions" DROP DEFAULT;
ALTER TABLE public."User" ALTER COLUMN "permissions" TYPE jsonb USING to_jsonb("permissions");
ALTER TABLE public."User" ALTER COLUMN "permissions" SET DEFAULT '[]'::jsonb;
