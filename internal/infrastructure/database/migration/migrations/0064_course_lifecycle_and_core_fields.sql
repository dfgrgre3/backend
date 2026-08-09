-- ============================================================
-- Course Lifecycle, Tags, Changelog, Related Courses
-- ============================================================

-- 1. Add status and core fields to Subject table
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'draft';
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS max_students INTEGER;
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS version TEXT DEFAULT '1.0.0';
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS is_trending BOOLEAN DEFAULT false;
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS is_new BOOLEAN DEFAULT false;
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS new_until TIMESTAMPTZ;
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS short_description TEXT;
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS long_description TEXT;
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS has_certificate BOOLEAN DEFAULT false;
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS available_from TIMESTAMPTZ;
ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS available_until TIMESTAMPTZ;

-- Migrate existing data: convert is_published/is_active into status
UPDATE "Subject" SET status = 'published' WHERE is_published = true AND is_active = true AND status = 'draft';
UPDATE "Subject" SET status = 'archived' WHERE is_active = false AND status = 'draft';
UPDATE "Subject" SET status = 'draft' WHERE is_published = false AND is_active = true AND status = 'draft';

-- 2. Course tags table
CREATE TABLE IF NOT EXISTS "CourseTag" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- 3. Join table: Subject <-> Tag (many-to-many)
CREATE TABLE IF NOT EXISTS "SubjectTag" (
    subject_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES "CourseTag"(id) ON DELETE CASCADE,
    PRIMARY KEY (subject_id, tag_id)
);

-- 4. Course changelog (audit trail for every field change)
CREATE TABLE IF NOT EXISTS "CourseChangelog" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    field_name TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT,
    action TEXT NOT NULL DEFAULT 'update',
    created_at TIMESTAMPTZ DEFAULT now()
);

-- 5. Review workflow comments
CREATE TABLE IF NOT EXISTS "CourseReviewComment" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    reviewer_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    comment TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- 6. Related / prerequisite courses (self-referencing many-to-many)
CREATE TABLE IF NOT EXISTS "RelatedCourse" (
    course_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    related_course_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    relation_type TEXT DEFAULT 'related',
    PRIMARY KEY (course_id, related_course_id),
    CHECK (course_id <> related_course_id)
);

-- 7. Indexes
CREATE INDEX IF NOT EXISTS idx_subject_status ON "Subject"(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_subject_tags ON "SubjectTag"(subject_id);
CREATE INDEX IF NOT EXISTS idx_changelog_subject ON "CourseChangelog"(subject_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_review_comments_subject ON "CourseReviewComment"(subject_id);
CREATE INDEX IF NOT EXISTS idx_related_courses ON "RelatedCourse"(course_id);
