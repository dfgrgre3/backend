-- Normalize legacy Prisma Book timestamp columns to the names used by GORM.
-- Keep this safe for databases that have already completed the rename.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'Book' AND column_name = 'createdAt'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'Book' AND column_name = 'created_at'
    ) THEN
        ALTER TABLE public."Book" RENAME COLUMN "createdAt" TO created_at;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'Book' AND column_name = 'updatedAt'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'Book' AND column_name = 'updated_at'
    ) THEN
        ALTER TABLE public."Book" RENAME COLUMN "updatedAt" TO updated_at;
    END IF;
END $$;
