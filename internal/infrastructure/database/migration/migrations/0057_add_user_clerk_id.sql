-- Migration to add clerk_id column to the User table
ALTER TABLE "User" ADD COLUMN IF NOT EXISTS clerk_id VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_clerk_id ON "User"(clerk_id);
