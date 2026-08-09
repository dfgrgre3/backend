-- 0121_add_instructor_status_column.sql
-- Add instructor_status column to User table for instructor management workflow

-- Add instructor_status column with default value
ALTER TABLE "User" 
ADD COLUMN IF NOT EXISTS instructor_status VARCHAR(50) NOT NULL DEFAULT 'PENDING';

-- Add check constraint for valid instructor statuses
ALTER TABLE "User" 
ADD CONSTRAINT chk_instructor_status 
CHECK (instructor_status IN ('PENDING', 'UNDER_REVIEW', 'APPROVED', 'REJECTED', 'SUSPENDED'));

-- Create index on instructor_status for filtering
CREATE INDEX IF NOT EXISTS idx_user_instructor_status ON "User"(instructor_status);

-- Create composite index for instructor filtering with role
CREATE INDEX IF NOT EXISTS idx_user_instructor_role_status ON "User"(role, instructor_status) WHERE deleted_at IS NULL;
