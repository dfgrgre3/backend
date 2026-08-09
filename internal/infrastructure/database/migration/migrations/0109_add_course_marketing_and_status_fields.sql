-- Course Marketing Fields and Status Workflow
-- Migration: 0109_add_course_marketing_and_status_fields.sql

-- ============================================================
-- 1. Course Status Workflow Fields on Subject
-- ============================================================

-- Add course status enum
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'UNDER_REVIEW', 'PUBLISHED', 'ARCHIVED', 'REJECTED'));

-- Workflow metadata
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS submitted_for_review_at TIMESTAMPTZ DEFAULT NULL;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ DEFAULT NULL;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS reviewed_by UUID DEFAULT NULL;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS rejection_reason TEXT DEFAULT NULL;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ DEFAULT NULL;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ DEFAULT NULL;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS archived_by UUID DEFAULT NULL;

-- ============================================================
-- 2. Marketing & Operational Fields
-- ============================================================

-- Short description (separate from long description)
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS short_description VARCHAR(500) DEFAULT NULL;

-- Tags (separate from meta_keywords - user-facing labels)
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}';

-- Trending and New flags
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS is_trending BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS is_new BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS featured_until TIMESTAMPTZ DEFAULT NULL;

-- Availability windows
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS available_from TIMESTAMPTZ DEFAULT NULL;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS available_until TIMESTAMPTZ DEFAULT NULL;

-- Max students (NULL = unlimited)
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS max_students INTEGER DEFAULT NULL
        CHECK (max_students IS NULL OR max_students > 0);

-- Enrollment type
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS enrollment_type VARCHAR(50) NOT NULL DEFAULT 'OPEN'
        CHECK (enrollment_type IN ('OPEN', 'LIMITED', 'BY_APPROVAL'));

-- Course language (extend existing)
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS secondary_languages TEXT[] DEFAULT '{}';

-- Course certificate config
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS certificate_enabled BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS certificate_template VARCHAR(255) DEFAULT NULL;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS certificate_issue_after_completion BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS certificate_min_completion_pct INTEGER NOT NULL DEFAULT 100
        CHECK (certificate_min_completion_pct BETWEEN 0 AND 100);

-- Version info (for versioning)
ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS current_version VARCHAR(50) DEFAULT '1.0.0';

ALTER TABLE "Subject"
    ADD COLUMN IF NOT EXISTS version_number INTEGER NOT NULL DEFAULT 1;

-- ============================================================
-- 3. Course Review Submissions (audit trail)
-- ============================================================

CREATE TABLE IF NOT EXISTS course_review_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    submitted_by UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'RESUBMITTED')),
    reviewer_id UUID DEFAULT NULL REFERENCES "User"(id) ON DELETE SET NULL,
    reviewer_notes TEXT DEFAULT NULL,
    rejection_reasons JSONB DEFAULT '[]',
    reviewed_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(subject_id, submitted_by)
);

COMMENT ON TABLE course_review_submissions IS 'Tracks course review/approval submissions with audit trail.';

-- ============================================================
-- 4. Course Assistant (co-instructor) management
-- ============================================================

CREATE TABLE IF NOT EXISTS course_assistants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'ASSISTANT'
        CHECK (role IN ('ASSISTANT', 'CO_INSTRUCTOR', 'TEACHING_ASSISTANT')),
    permissions JSONB DEFAULT '{"edit_content":false,"manage_students":false,"view_analytics":true,"manage_quizzes":false}',
    invited_by UUID DEFAULT NULL REFERENCES "User"(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'ACTIVE', 'REVOKED')),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ DEFAULT NULL,
    revoked_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(subject_id, user_id)
);

COMMENT ON TABLE course_assistants IS 'Manages co-instructors and teaching assistants for each course.';

-- ============================================================
-- 5. Course Changelog (immutable history)
-- ============================================================

CREATE TABLE IF NOT EXISTS course_changelogs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    change_type VARCHAR(50) NOT NULL DEFAULT 'UPDATE'
        CHECK (change_type IN ('CREATE', 'UPDATE', 'PUBLISH', 'ARCHIVE', 'RESTORE', 'DELETE')),
    changes JSONB NOT NULL DEFAULT '{}',
    changed_by UUID DEFAULT NULL REFERENCES "User"(id) ON DELETE SET NULL,
    ip_address INET DEFAULT NULL,
    user_agent TEXT DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(subject_id, version)
);

COMMENT ON TABLE course_changelogs IS 'Immutable audit log of all changes to course content and status.';

-- ============================================================
-- 6. Lesson Drip Schedule
-- ============================================================

CREATE TABLE IF NOT EXISTS lesson_drip_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_topic_id UUID NOT NULL REFERENCES "SubTopic"(id) ON DELETE CASCADE,
    drip_type VARCHAR(50) NOT NULL DEFAULT 'ABSOLUTE'
        CHECK (drip_type IN ('ABSOLUTE', 'RELATIVE')),
    release_date TIMESTAMPTZ DEFAULT NULL,
    days_after_enrollment INTEGER DEFAULT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID DEFAULT NULL REFERENCES "User"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(sub_topic_id)
);

COMMENT ON TABLE lesson_drip_schedules IS 'Controls when lessons become available: absolute date or relative to enrollment.';

-- ============================================================
-- 7. Video Chapters / Timestamped Notes
-- ============================================================

CREATE TABLE IF NOT EXISTS video_chapters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_topic_id UUID NOT NULL REFERENCES "SubTopic"(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    title_ar VARCHAR(255) DEFAULT NULL,
    time_seconds INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE video_chapters IS 'Timestamped chapter markers for video lessons.';

-- ============================================================
-- 8. Lesson Subtitles / Captions
-- ============================================================

CREATE TABLE IF NOT EXISTS lesson_subtitles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_topic_id UUID NOT NULL REFERENCES "SubTopic"(id) ON DELETE CASCADE,
    language VARCHAR(10) NOT NULL,
    language_name VARCHAR(100) DEFAULT NULL,
    subtitle_url TEXT NOT NULL,
    subtitle_format VARCHAR(20) NOT NULL DEFAULT 'vtt'
        CHECK (subtitle_format IN ('vtt', 'srt', 'json')),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_for_hearing_impaired BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(sub_topic_id, language)
);

COMMENT ON TABLE lesson_subtitles IS 'Multiple subtitle tracks per lesson (language, VTT/SRT/JSON format).';

-- ============================================================
-- 9. Lesson View Statistics (per-user granular)
-- ============================================================

CREATE TABLE IF NOT EXISTS lesson_view_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_topic_id UUID NOT NULL REFERENCES "SubTopic"(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    watch_time_seconds INTEGER NOT NULL DEFAULT 0,
    last_position_seconds INTEGER NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ DEFAULT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_position_seconds INTEGER NOT NULL DEFAULT 0,
    device_type VARCHAR(50) DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(sub_topic_id, user_id)
);

COMMENT ON TABLE lesson_view_stats IS 'Per-user granular view tracking for video lessons.';

-- ============================================================
-- 10. Course Availability Windows (scheduled publish/unpublish)
-- ============================================================

CREATE TABLE IF NOT EXISTS course_availability_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    window_type VARCHAR(50) NOT NULL DEFAULT 'PUBLISH'
        CHECK (window_type IN ('ENROLLMENT', 'ACCESS', 'PUBLISH', 'UNPUBLISH')),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ DEFAULT NULL,
    is_repeating BOOLEAN NOT NULL DEFAULT FALSE,
    repeat_pattern VARCHAR(50) DEFAULT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID DEFAULT NULL REFERENCES "User"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE course_availability_windows IS 'Scheduled enrollment/access windows for courses.';

-- ============================================================
-- Indexes
-- ============================================================

-- Subject status indexes
CREATE INDEX IF NOT EXISTS idx_subject_status ON "Subject"(status);
CREATE INDEX IF NOT EXISTS idx_subject_status_active ON "Subject"(status, is_active, is_published)
    WHERE is_active = TRUE AND is_published = TRUE;

-- Review submissions
CREATE INDEX IF NOT EXISTS idx_course_review_submissions_subject ON course_review_submissions(subject_id);
CREATE INDEX IF NOT EXISTS idx_course_review_submissions_status ON course_review_submissions(status) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_course_review_submissions_submitted_by ON course_review_submissions(submitted_by);

-- Course assistants
CREATE INDEX IF NOT EXISTS idx_course_assistants_subject ON course_assistants(subject_id);
CREATE INDEX IF NOT EXISTS idx_course_assistants_user ON course_assistants(user_id);
CREATE INDEX IF NOT EXISTS idx_course_assistants_status ON course_assistants(status) WHERE status = 'ACTIVE';

-- Changelogs
CREATE INDEX IF NOT EXISTS idx_course_changelogs_subject ON course_changelogs(subject_id);
CREATE INDEX IF NOT EXISTS idx_course_changelogs_created ON course_changelogs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_course_changelogs_version ON course_changelogs(subject_id, version DESC);

-- Drip schedules
CREATE INDEX IF NOT EXISTS idx_lesson_drip_sub_topic ON lesson_drip_schedules(sub_topic_id);
CREATE INDEX IF NOT EXISTS idx_lesson_drip_active ON lesson_drip_schedules(is_active) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_lesson_drip_release ON lesson_drip_schedules(release_date) WHERE release_date IS NOT NULL;

-- Video chapters
CREATE INDEX IF NOT EXISTS idx_video_chapters_sub_topic ON video_chapters(sub_topic_id);
CREATE INDEX IF NOT EXISTS idx_video_chapters_order ON video_chapters(sub_topic_id, sort_order);

-- Subtitles
CREATE INDEX IF NOT EXISTS idx_lesson_subtitles_sub_topic ON lesson_subtitles(sub_topic_id);

-- Lesson view stats
CREATE INDEX IF NOT EXISTS idx_lesson_view_stats_sub_topic ON lesson_view_stats(sub_topic_id);
CREATE INDEX IF NOT EXISTS idx_lesson_view_stats_user ON lesson_view_stats(user_id);
CREATE INDEX IF NOT EXISTS idx_lesson_view_stats_completed ON lesson_view_stats(sub_topic_id, completed) WHERE completed = TRUE;

-- Availability windows
CREATE INDEX IF NOT EXISTS idx_course_availability_subject ON course_availability_windows(subject_id);
CREATE INDEX IF NOT EXISTS idx_course_availability_starts ON course_availability_windows(starts_at) WHERE is_active = TRUE;
