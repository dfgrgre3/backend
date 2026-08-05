-- Migration 0133: Add missing ActivityLog table
-- This table is used by the teaching handler to track user activities

CREATE TABLE IF NOT EXISTS "ActivityLog" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    resource_id UUID,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_activity_log_user_created ON "ActivityLog" (user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_activity_log_resource ON "ActivityLog" (resource, resource_id);
CREATE INDEX IF NOT EXISTS idx_activity_log_action ON "ActivityLog" (action);

-- Add foreign key constraint to User table
ALTER TABLE "ActivityLog" 
ADD CONSTRAINT fk_activity_log_user 
FOREIGN KEY (user_id) REFERENCES "User"(id) ON DELETE CASCADE;
