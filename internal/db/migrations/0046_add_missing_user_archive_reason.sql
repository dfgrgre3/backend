ALTER TABLE IF EXISTS public."User"
    ADD COLUMN IF NOT EXISTS archive_reason text;
