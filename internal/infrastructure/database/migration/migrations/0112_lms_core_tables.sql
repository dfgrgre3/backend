-- 0112_lms_core_tables.sql
-- Comprehensive LMS core tables: courses, sections, lessons, pricing, bundles,
-- instructors, categories, tags, changelog, versions, certificates, reviews,
-- enrollments, attachments, subtitles, interactive quizzes, video notes,
-- lesson interactions, and join tables.

-- Use IF NOT EXISTS for idempotency.

-- ============================================
-- 1. LmsCategory (category tree)
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsCategory" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "name"       TEXT NOT NULL,
    "slug"       TEXT NOT NULL,
    "parent_id"  UUID,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at" TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS "LmsCategory_slug_key" ON "LmsCategory" ("slug") WHERE "deleted_at" IS NULL;
CREATE INDEX IF NOT EXISTS "LmsCategory_parent_id_idx" ON "LmsCategory" ("parent_id");
CREATE INDEX IF NOT EXISTS "LmsCategory_deleted_at_idx" ON "LmsCategory" ("deleted_at");

-- ============================================
-- 2. LmsTag
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsTag" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "name"       TEXT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at" TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS "LmsTag_name_key" ON "LmsTag" ("name") WHERE "deleted_at" IS NULL;
CREATE INDEX IF NOT EXISTS "LmsTag_deleted_at_idx" ON "LmsTag" ("deleted_at");

-- ============================================
-- 3. LmsCourse
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsCourse" (
    "id"                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "title"                  TEXT NOT NULL,
    "slug"                   TEXT NOT NULL,
    "short_description"      TEXT,
    "long_description"       TEXT,
    "cover_image_url"        TEXT,
    "promo_video_url"        TEXT,
    "status"                 TEXT NOT NULL DEFAULT 'DRAFT',
    "level"                  TEXT NOT NULL DEFAULT 'BEGINNER',
    "language"               TEXT NOT NULL DEFAULT 'ar',
    "estimated_duration_mins" INTEGER NOT NULL DEFAULT 0,
    "has_certificate"        BOOLEAN NOT NULL DEFAULT false,
    "certificate_template"   TEXT,
    "max_students"           INTEGER,
    "version"                INTEGER NOT NULL DEFAULT 1,
    "is_featured"            BOOLEAN NOT NULL DEFAULT false,
    "is_trending"            BOOLEAN NOT NULL DEFAULT false,
    "is_new"                 BOOLEAN NOT NULL DEFAULT false,
    "new_until"              TIMESTAMPTZ,
    "seo_title"              TEXT,
    "seo_description"        TEXT,
    "seo_keywords"           TEXT[],
    "prerequisites_text"     TEXT,
    "target_audience"        TEXT,
    "learning_outcomes"      TEXT[],
    "primary_instructor_id"  UUID NOT NULL,
    "available_from"         TIMESTAMPTZ,
    "available_until"        TIMESTAMPTZ,
    "created_at"             TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"             TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"             TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS "LmsCourse_slug_key" ON "LmsCourse" ("slug") WHERE "deleted_at" IS NULL;
CREATE INDEX IF NOT EXISTS "LmsCourse_title_idx" ON "LmsCourse" ("title");
CREATE INDEX IF NOT EXISTS "LmsCourse_status_idx" ON "LmsCourse" ("status");
CREATE INDEX IF NOT EXISTS "LmsCourse_level_idx" ON "LmsCourse" ("level");
CREATE INDEX IF NOT EXISTS "LmsCourse_language_idx" ON "LmsCourse" ("language");
CREATE INDEX IF NOT EXISTS "LmsCourse_is_featured_idx" ON "LmsCourse" ("is_featured");
CREATE INDEX IF NOT EXISTS "LmsCourse_is_trending_idx" ON "LmsCourse" ("is_trending");
CREATE INDEX IF NOT EXISTS "LmsCourse_is_new_idx" ON "LmsCourse" ("is_new");
CREATE INDEX IF NOT EXISTS "LmsCourse_primary_instructor_id_idx" ON "LmsCourse" ("primary_instructor_id");
CREATE INDEX IF NOT EXISTS "LmsCourse_created_at_idx" ON "LmsCourse" ("created_at");
CREATE INDEX IF NOT EXISTS "LmsCourse_deleted_at_idx" ON "LmsCourse" ("deleted_at");

-- ============================================
-- 4. LmsSection
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsSection" (
    "id"              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"       UUID NOT NULL,
    "title"           TEXT NOT NULL,
    "order_index"     INTEGER NOT NULL DEFAULT 0,
    "available_from"  TIMESTAMPTZ,
    "drip_delay_days" INTEGER,
    "created_at"      TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"      TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"      TIMESTAMPTZ,
    CONSTRAINT "LmsSection_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsSection_course_id_idx" ON "LmsSection" ("course_id");
CREATE INDEX IF NOT EXISTS "LmsSection_order_index_idx" ON "LmsSection" ("order_index");
CREATE INDEX IF NOT EXISTS "LmsSection_deleted_at_idx" ON "LmsSection" ("deleted_at");

-- ============================================
-- 5. LmsLesson
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsLesson" (
    "id"                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "section_id"        UUID NOT NULL,
    "title"             TEXT NOT NULL,
    "type"              TEXT NOT NULL DEFAULT 'VIDEO',
    "content"           TEXT,
    "media_url"         TEXT,
    "duration_seconds"  INTEGER NOT NULL DEFAULT 0,
    "is_free_preview"   BOOLEAN NOT NULL DEFAULT false,
    "order_index"       INTEGER NOT NULL DEFAULT 0,
    "availability_type" TEXT NOT NULL DEFAULT 'CALENDAR_DATE',
    "available_from"    TIMESTAMPTZ,
    "drip_delay_days"   INTEGER,
    "created_at"        TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"        TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"        TIMESTAMPTZ,
    CONSTRAINT "LmsLesson_section_id_fkey" FOREIGN KEY ("section_id") REFERENCES "LmsSection"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsLesson_section_id_idx" ON "LmsLesson" ("section_id");
CREATE INDEX IF NOT EXISTS "LmsLesson_type_idx" ON "LmsLesson" ("type");
CREATE INDEX IF NOT EXISTS "LmsLesson_is_free_preview_idx" ON "LmsLesson" ("is_free_preview");
CREATE INDEX IF NOT EXISTS "LmsLesson_order_index_idx" ON "LmsLesson" ("order_index");
CREATE INDEX IF NOT EXISTS "LmsLesson_deleted_at_idx" ON "LmsLesson" ("deleted_at");

-- ============================================
-- 6. LmsAttachment
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsAttachment" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "lesson_id"  UUID NOT NULL,
    "title"      TEXT NOT NULL,
    "file_url"   TEXT NOT NULL,
    "file_type"  TEXT,
    "file_size"  BIGINT,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT "LmsAttachment_lesson_id_fkey" FOREIGN KEY ("lesson_id") REFERENCES "LmsLesson"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsAttachment_lesson_id_idx" ON "LmsAttachment" ("lesson_id");
CREATE INDEX IF NOT EXISTS "LmsAttachment_deleted_at_idx" ON "LmsAttachment" ("deleted_at");

-- ============================================
-- 7. LmsSubtitle
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsSubtitle" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "lesson_id"  UUID NOT NULL,
    "language"   TEXT NOT NULL,
    "vtt_url"    TEXT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT "LmsSubtitle_lesson_id_fkey" FOREIGN KEY ("lesson_id") REFERENCES "LmsLesson"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsSubtitle_lesson_id_idx" ON "LmsSubtitle" ("lesson_id");
CREATE INDEX IF NOT EXISTS "LmsSubtitle_language_idx" ON "LmsSubtitle" ("language");
CREATE INDEX IF NOT EXISTS "LmsSubtitle_deleted_at_idx" ON "LmsSubtitle" ("deleted_at");

-- ============================================
-- 8. LmsInteractiveQuiz
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsInteractiveQuiz" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "lesson_id"     UUID NOT NULL,
    "timestamp_sec" INTEGER NOT NULL,
    "question"      TEXT NOT NULL,
    "options"       JSONB NOT NULL DEFAULT '[]'::jsonb,
    "correct_index" INTEGER NOT NULL,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"    TIMESTAMPTZ,
    CONSTRAINT "LmsInteractiveQuiz_lesson_id_fkey" FOREIGN KEY ("lesson_id") REFERENCES "LmsLesson"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsInteractiveQuiz_lesson_id_idx" ON "LmsInteractiveQuiz" ("lesson_id");
CREATE INDEX IF NOT EXISTS "LmsInteractiveQuiz_deleted_at_idx" ON "LmsInteractiveQuiz" ("deleted_at");

-- ============================================
-- 9. LmsVideoNote
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsVideoNote" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "lesson_id"     UUID NOT NULL,
    "user_id"       UUID NOT NULL,
    "timestamp_sec" INTEGER NOT NULL,
    "note"          TEXT NOT NULL,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT "LmsVideoNote_lesson_id_fkey" FOREIGN KEY ("lesson_id") REFERENCES "LmsLesson"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsVideoNote_lesson_id_idx" ON "LmsVideoNote" ("lesson_id");
CREATE INDEX IF NOT EXISTS "LmsVideoNote_user_id_idx" ON "LmsVideoNote" ("user_id");
CREATE INDEX IF NOT EXISTS "LmsVideoNote_created_at_idx" ON "LmsVideoNote" ("created_at");

-- ============================================
-- 10. LmsLessonInteraction
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsLessonInteraction" (
    "id"                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "lesson_id"            UUID NOT NULL,
    "user_id"              UUID NOT NULL,
    "watched_duration_sec" INTEGER NOT NULL DEFAULT 0,
    "last_position_sec"    INTEGER NOT NULL DEFAULT 0,
    "play_count"           INTEGER NOT NULL DEFAULT 0,
    "is_completed"         BOOLEAN NOT NULL DEFAULT false,
    "quiz_answers"         JSONB,
    "created_at"           TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT "LmsLessonInteraction_lesson_id_fkey" FOREIGN KEY ("lesson_id") REFERENCES "LmsLesson"("id") ON DELETE CASCADE,
    CONSTRAINT "idx_lms_interaction_user_lesson" UNIQUE ("lesson_id", "user_id")
);
CREATE INDEX IF NOT EXISTS "LmsLessonInteraction_is_completed_idx" ON "LmsLessonInteraction" ("is_completed");

-- ============================================
-- 11. LmsCourseCategory (join)
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsCourseCategory" (
    "course_id"    UUID NOT NULL,
    "category_id" UUID NOT NULL,
    PRIMARY KEY ("course_id", "category_id"),
    CONSTRAINT "LmsCourseCategory_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE,
    CONSTRAINT "LmsCourseCategory_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "LmsCategory"("id") ON DELETE CASCADE
);

-- ============================================
-- 12. LmsCourseTag (join)
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsCourseTag" (
    "course_id" UUID NOT NULL,
    "tag_id"    UUID NOT NULL,
    PRIMARY KEY ("course_id", "tag_id"),
    CONSTRAINT "LmsCourseTag_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE,
    CONSTRAINT "LmsCourseTag_tag_id_fkey" FOREIGN KEY ("tag_id") REFERENCES "LmsTag"("id") ON DELETE CASCADE
);

-- ============================================
-- 13. LmsCoursePrerequisite (join)
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsCoursePrerequisite" (
    "course_id"              UUID NOT NULL,
    "prerequisite_course_id" UUID NOT NULL,
    PRIMARY KEY ("course_id", "prerequisite_course_id"),
    CONSTRAINT "LmsCoursePrerequisite_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE,
    CONSTRAINT "LmsCoursePrerequisite_prerequisite_course_id_fkey" FOREIGN KEY ("prerequisite_course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);

-- ============================================
-- 14. LmsRelatedCourse (join)
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsRelatedCourse" (
    "course_id"        UUID NOT NULL,
    "related_course_id" UUID NOT NULL,
    PRIMARY KEY ("course_id", "related_course_id"),
    CONSTRAINT "LmsRelatedCourse_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE,
    CONSTRAINT "LmsRelatedCourse_related_course_id_fkey" FOREIGN KEY ("related_course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);

-- ============================================
-- 15. LmsInstructor
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsInstructor" (
    "course_id"     UUID NOT NULL,
    "instructor_id" UUID NOT NULL,
    "role"          TEXT NOT NULL DEFAULT 'INSTRUCTOR',
    "permissions"  JSONB,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY ("course_id", "instructor_id"),
    CONSTRAINT "LmsInstructor_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsInstructor_instructor_id_idx" ON "LmsInstructor" ("instructor_id");

-- ============================================
-- 16. LmsPricing
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsPricing" (
    "id"                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"                 UUID NOT NULL,
    "type"                      TEXT NOT NULL DEFAULT 'FREE',
    "amount"                    DOUBLE PRECISION NOT NULL DEFAULT 0,
    "currency_code"             TEXT NOT NULL DEFAULT 'USD',
    "subscription_duration_days" INTEGER,
    "is_active"                 BOOLEAN NOT NULL DEFAULT true,
    "created_at"                TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"                TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"                TIMESTAMPTZ,
    CONSTRAINT "LmsPricing_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsPricing_course_id_idx" ON "LmsPricing" ("course_id");
CREATE INDEX IF NOT EXISTS "LmsPricing_type_idx" ON "LmsPricing" ("type");
CREATE INDEX IF NOT EXISTS "LmsPricing_currency_code_idx" ON "LmsPricing" ("currency_code");
CREATE INDEX IF NOT EXISTS "LmsPricing_deleted_at_idx" ON "LmsPricing" ("deleted_at");

-- ============================================
-- 17. LmsBundle
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsBundle" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "title"         TEXT NOT NULL,
    "slug"          TEXT NOT NULL,
    "description"   TEXT,
    "cover_url"     TEXT,
    "price"         DOUBLE PRECISION NOT NULL DEFAULT 0,
    "currency_code" TEXT NOT NULL DEFAULT 'USD',
    "is_active"     BOOLEAN NOT NULL DEFAULT true,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"    TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS "LmsBundle_slug_key" ON "LmsBundle" ("slug") WHERE "deleted_at" IS NULL;
CREATE INDEX IF NOT EXISTS "LmsBundle_title_idx" ON "LmsBundle" ("title");
CREATE INDEX IF NOT EXISTS "LmsBundle_is_active_idx" ON "LmsBundle" ("is_active");
CREATE INDEX IF NOT EXISTS "LmsBundle_created_at_idx" ON "LmsBundle" ("created_at");
CREATE INDEX IF NOT EXISTS "LmsBundle_deleted_at_idx" ON "LmsBundle" ("deleted_at");

-- ============================================
-- 18. LmsBundleCourse (join)
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsBundleCourse" (
    "bundle_id" UUID NOT NULL,
    "course_id" UUID NOT NULL,
    PRIMARY KEY ("bundle_id", "course_id"),
    CONSTRAINT "LmsBundleCourse_bundle_id_fkey" FOREIGN KEY ("bundle_id") REFERENCES "LmsBundle"("id") ON DELETE CASCADE,
    CONSTRAINT "LmsBundleCourse_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);

-- ============================================
-- 19. LmsCourseChangelog
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsCourseChangelog" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"  UUID NOT NULL,
    "user_id"    UUID NOT NULL,
    "field"      TEXT NOT NULL,
    "old_value"  TEXT,
    "new_value"  TEXT,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT "LmsCourseChangelog_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsCourseChangelog_course_id_idx" ON "LmsCourseChangelog" ("course_id");
CREATE INDEX IF NOT EXISTS "LmsCourseChangelog_user_id_idx" ON "LmsCourseChangelog" ("user_id");
CREATE INDEX IF NOT EXISTS "LmsCourseChangelog_created_at_idx" ON "LmsCourseChangelog" ("created_at");

-- ============================================
-- 20. LmsCourseVersion
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsCourseVersion" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"     UUID NOT NULL,
    "version_number" INTEGER NOT NULL,
    "snapshot"      JSONB NOT NULL,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT "LmsCourseVersion_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsCourseVersion_course_id_idx" ON "LmsCourseVersion" ("course_id");
CREATE INDEX IF NOT EXISTS "LmsCourseVersion_created_at_idx" ON "LmsCourseVersion" ("created_at");

-- ============================================
-- 21. LmsCertificate
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsCertificate" (
    "id"             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"      UUID NOT NULL,
    "user_id"        UUID NOT NULL,
    "certificate_no" TEXT NOT NULL,
    "qr_code_url"   TEXT,
    "pdf_url"       TEXT NOT NULL,
    "issued_at"      TIMESTAMPTZ NOT NULL DEFAULT now(),
    "created_at"     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT "LmsCertificate_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE,
    CONSTRAINT "idx_lms_cert_user_course" UNIQUE ("course_id", "user_id")
);
CREATE UNIQUE INDEX IF NOT EXISTS "LmsCertificate_certificate_no_key" ON "LmsCertificate" ("certificate_no");
CREATE INDEX IF NOT EXISTS "LmsCertificate_course_id_idx" ON "LmsCertificate" ("course_id");
CREATE INDEX IF NOT EXISTS "LmsCertificate_issued_at_idx" ON "LmsCertificate" ("issued_at");

-- ============================================
-- 22. LmsReview
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsReview" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"  UUID NOT NULL,
    "user_id"    UUID NOT NULL,
    "rating"     INTEGER NOT NULL DEFAULT 5,
    "comment"    TEXT,
    "status"     TEXT NOT NULL DEFAULT 'APPROVED',
    "reply"      TEXT,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT "LmsReview_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE,
    CONSTRAINT "idx_lms_review_user_course" UNIQUE ("course_id", "user_id")
);
CREATE INDEX IF NOT EXISTS "LmsReview_status_idx" ON "LmsReview" ("status");
CREATE INDEX IF NOT EXISTS "LmsReview_created_at_idx" ON "LmsReview" ("created_at");
CREATE INDEX IF NOT EXISTS "LmsReview_deleted_at_idx" ON "LmsReview" ("deleted_at");

-- ============================================
-- 23. LmsEnrollment
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsEnrollment" (
    "id"          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"   UUID NOT NULL,
    "user_id"     UUID NOT NULL,
    "progress"    DOUBLE PRECISION NOT NULL DEFAULT 0,
    "enrolled_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "completed_at" TIMESTAMPTZ,
    "bundle_id"   UUID,
    "created_at"  TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"  TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"  TIMESTAMPTZ,
    CONSTRAINT "LmsEnrollment_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE,
    CONSTRAINT "idx_lms_enroll_user_course" UNIQUE ("course_id", "user_id")
);
CREATE INDEX IF NOT EXISTS "LmsEnrollment_enrolled_at_idx" ON "LmsEnrollment" ("enrolled_at");
CREATE INDEX IF NOT EXISTS "LmsEnrollment_bundle_id_idx" ON "LmsEnrollment" ("bundle_id");
CREATE INDEX IF NOT EXISTS "LmsEnrollment_deleted_at_idx" ON "LmsEnrollment" ("deleted_at");

-- ============================================
-- 24. LmsReviewComment (workflow review comments)
-- ============================================
CREATE TABLE IF NOT EXISTS "LmsReviewComment" (
    "id"         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "course_id"  UUID NOT NULL,
    "reviewer_id" UUID NOT NULL,
    "comment"    TEXT NOT NULL,
    "status"     TEXT NOT NULL DEFAULT 'pending',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT "LmsReviewComment_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "LmsCourse"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "LmsReviewComment_course_id_idx" ON "LmsReviewComment" ("course_id");
CREATE INDEX IF NOT EXISTS "LmsReviewComment_reviewer_id_idx" ON "LmsReviewComment" ("reviewer_id");