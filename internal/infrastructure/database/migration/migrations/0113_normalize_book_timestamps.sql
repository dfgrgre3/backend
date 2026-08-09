-- Normalize legacy Prisma Book timestamp columns to the names used by GORM.
-- These ALTER TABLE statements will fail gracefully if columns don't exist
ALTER TABLE public."Book" RENAME COLUMN "createdAt" TO created_at;
ALTER TABLE public."Book" RENAME COLUMN "updatedAt" TO updated_at;
