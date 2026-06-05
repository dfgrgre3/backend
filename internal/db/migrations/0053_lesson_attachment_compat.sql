-- Migration: 0053_lesson_attachment_compat.sql
-- Description: Add snake_case columns to LessonAttachment and sync with camelCase columns using a trigger

-- 1. Add snake_case columns if they don't exist
ALTER TABLE IF EXISTS public."LessonAttachment"
    ADD COLUMN IF NOT EXISTS sub_topic_id uuid,
    ADD COLUMN IF NOT EXISTS file_url text,
    ADD COLUMN IF NOT EXISTS file_type text,
    ADD COLUMN IF NOT EXISTS file_size bigint,
    ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at timestamptz DEFAULT now();

-- 2. Drop NOT NULL constraints on legacy camelCase columns
ALTER TABLE IF EXISTS public."LessonAttachment" ALTER COLUMN "subTopicId" DROP NOT NULL;
ALTER TABLE IF EXISTS public."LessonAttachment" ALTER COLUMN "fileUrl" DROP NOT NULL;
ALTER TABLE IF EXISTS public."LessonAttachment" ALTER COLUMN "updatedAt" DROP NOT NULL;

-- 3. Populate existing rows
UPDATE public."LessonAttachment"
SET sub_topic_id = COALESCE(sub_topic_id, CASE WHEN "subTopicId" IS NOT NULL AND "subTopicId" <> '' THEN "subTopicId"::uuid ELSE NULL END),
    file_url = COALESCE(file_url, "fileUrl"),
    file_type = COALESCE(file_type, "fileType"),
    file_size = COALESCE(file_size, "fileSize"),
    created_at = COALESCE(created_at, "createdAt");

-- 4. Create trigger function to synchronize both sets of columns
CREATE OR REPLACE FUNCTION public.sync_lesson_attachment_cols()
RETURNS TRIGGER AS $$
BEGIN
    -- Sync GORM (snake_case) -> Prisma (camelCase)
    IF NEW.sub_topic_id IS NOT NULL THEN
        NEW."subTopicId" := NEW.sub_topic_id::text;
    END IF;
    IF NEW.file_url IS NOT NULL THEN
        NEW."fileUrl" := NEW.file_url;
    END IF;
    IF NEW.file_type IS NOT NULL THEN
        NEW."fileType" := NEW.file_type;
    END IF;
    IF NEW.file_size IS NOT NULL THEN
        NEW."fileSize" := NEW.file_size;
    END IF;
    IF NEW.created_at IS NOT NULL THEN
        NEW."createdAt" := NEW.created_at;
    END IF;

    -- Sync Prisma (camelCase) -> GORM (snake_case)
    IF NEW."subTopicId" IS NOT NULL AND NEW.sub_topic_id IS NULL THEN
        NEW.sub_topic_id := NEW."subTopicId"::uuid;
    END IF;
    IF NEW."fileUrl" IS NOT NULL AND NEW.file_url IS NULL THEN
        NEW.file_url := NEW."fileUrl";
    END IF;
    IF NEW."fileType" IS NOT NULL AND NEW.file_type IS NULL THEN
        NEW.file_type := NEW."fileType";
    END IF;
    IF NEW."fileSize" IS NOT NULL AND NEW.file_size IS NULL THEN
        NEW.file_size := NEW."fileSize";
    END IF;
    IF NEW."createdAt" IS NOT NULL AND NEW.created_at IS NULL THEN
        NEW.created_at := NEW."createdAt";
    END IF;

    -- Default updatedAt/updated_at
    NEW."updatedAt" := NOW();
    NEW.updated_at := NOW();
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 5. Attach the trigger
DROP TRIGGER IF EXISTS trigger_sync_lesson_attachment_cols ON public."LessonAttachment";
CREATE TRIGGER trigger_sync_lesson_attachment_cols
BEFORE INSERT OR UPDATE ON public."LessonAttachment"
FOR EACH ROW
EXECUTE FUNCTION public.sync_lesson_attachment_cols();
