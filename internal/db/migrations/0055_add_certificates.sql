-- Migration: Add Certificate table for course completion certificates
-- Creates the Certificate table and indexes for fast lookups

CREATE TABLE IF NOT EXISTS "Certificate" (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    subject_id  UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Ensure one certificate per user per course
    UNIQUE(user_id, subject_id)
);

-- Indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_certificate_user_id ON "Certificate"(user_id);
CREATE INDEX IF NOT EXISTS idx_certificate_subject_id ON "Certificate"(subject_id);
CREATE INDEX IF NOT EXISTS idx_certificate_issued_at ON "Certificate"(issued_at DESC);

-- Add certificate count to UserCourse stats view (if exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_matviews WHERE matviewname = 'mv_user_course_stats') THEN
        -- Refresh the materialized view to include certificate data
        REFRESH MATERIALIZED VIEW mv_user_course_stats;
    END IF;
END $$;