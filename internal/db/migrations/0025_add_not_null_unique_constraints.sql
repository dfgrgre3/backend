-- Migration 0025: NOT NULL and UNIQUE constraints
-- Strengthens data integrity at the database level

BEGIN;

-- ============================================================
-- User: ensure critical fields are never null
-- ============================================================

DO $$
DECLARE
  schema_name CONSTANT TEXT := 'public';
  user_id_column CONSTANT TEXT := 'userId';
  subject_id_column CONSTANT TEXT := 'subjectId';
  user_table CONSTANT TEXT := 'User';
  subject_table CONSTANT TEXT := 'Subject';
  topic_table CONSTANT TEXT := 'Topic';
  subtopic_table CONSTANT TEXT := 'SubTopic';
  exam_table CONSTANT TEXT := 'Exam';
  question_table CONSTANT TEXT := 'Question';
  exam_result_table CONSTANT TEXT := 'ExamResult';
  enrollment_table CONSTANT TEXT := 'SubjectEnrollment';
  progress_table CONSTANT TEXT := 'TopicProgress';
  payment_table CONSTANT TEXT := 'Payment';
  notification_table CONSTANT TEXT := 'Notification';
  task_table CONSTANT TEXT := 'Task';
  study_session_table CONSTANT TEXT := 'StudySession';
BEGIN
  -- Email is already uniqueIndex; ensure NOT NULL if not already
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = user_table
    AND column_name = 'email' AND is_nullable = 'YES'
  ) THEN
    -- First set any null emails to a placeholder (should not exist but safety first)
    UPDATE "User" SET "email" = CONCAT('deleted_', id, '@placeholder.local') WHERE "email" IS NULL;
    ALTER TABLE "User" ALTER COLUMN "email" SET NOT NULL;
  END IF;

  -- Password hash must never be null
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = user_table
    AND column_name = 'passwordHash' AND is_nullable = 'YES'
  ) THEN
    UPDATE "User" SET "passwordHash" = '' WHERE "passwordHash" IS NULL;
    ALTER TABLE "User" ALTER COLUMN "passwordHash" SET NOT NULL;
  END IF;

  -- ============================================================
  -- Subject: ensure name is never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = subject_table
    AND column_name = 'name' AND is_nullable = 'YES'
  ) THEN
    UPDATE "Subject" SET "name" = CONCAT('Untitled_', id) WHERE "name" IS NULL;
    ALTER TABLE "Subject" ALTER COLUMN "name" SET NOT NULL;
  END IF;

  -- ============================================================
  -- Topic: ensure subjectId and title are never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = topic_table
    AND column_name = subject_id_column AND is_nullable = 'YES'
  ) THEN
    -- Cannot fix orphaned topics; they should have been deleted by FK cascade
    DELETE FROM "Topic" WHERE "subjectId" IS NULL;
    ALTER TABLE "Topic" ALTER COLUMN "subjectId" SET NOT NULL;
  END IF;

  -- ============================================================
  -- SubTopic: ensure topicId and title are never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = subtopic_table
    AND column_name = 'topicId' AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "SubTopic" WHERE "topicId" IS NULL;
    ALTER TABLE "SubTopic" ALTER COLUMN "topicId" SET NOT NULL;
  END IF;

  -- ============================================================
  -- Exam: ensure subjectId, title, duration are never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = exam_table
    AND column_name = subject_id_column AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "Exam" WHERE "subjectId" IS NULL;
    ALTER TABLE "Exam" ALTER COLUMN "subjectId" SET NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = exam_table
    AND column_name = 'title' AND is_nullable = 'YES'
  ) THEN
    UPDATE "Exam" SET "title" = CONCAT('Untitled_', id) WHERE "title" IS NULL;
    ALTER TABLE "Exam" ALTER COLUMN "title" SET NOT NULL;
  END IF;

  -- ============================================================
  -- Question: ensure examId, text, answer are never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = question_table
    AND column_name = 'examId' AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "Question" WHERE "examId" IS NULL;
    ALTER TABLE "Question" ALTER COLUMN "examId" SET NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = question_table
    AND column_name = 'text' AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "Question" WHERE "text" IS NULL;
    ALTER TABLE "Question" ALTER COLUMN "text" SET NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = question_table
    AND column_name = 'answer' AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "Question" WHERE "answer" IS NULL;
    ALTER TABLE "Question" ALTER COLUMN "answer" SET NOT NULL;
  END IF;

  -- ============================================================
  -- ExamResult: ensure examId, userId, score are never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = exam_result_table
    AND column_name = 'exam_id' AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "ExamResult" WHERE "exam_id" IS NULL;
    ALTER TABLE "ExamResult" ALTER COLUMN "exam_id" SET NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = exam_result_table
    AND column_name = 'user_id' AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "ExamResult" WHERE "user_id" IS NULL;
    ALTER TABLE "ExamResult" ALTER COLUMN "user_id" SET NOT NULL;
  END IF;

  -- ============================================================
  -- Enrollment (SubjectEnrollment): ensure userId, subjectId never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = enrollment_table
    AND column_name = user_id_column AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "SubjectEnrollment" WHERE "userId" IS NULL;
    ALTER TABLE "SubjectEnrollment" ALTER COLUMN "userId" SET NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = enrollment_table
    AND column_name = subject_id_column AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "SubjectEnrollment" WHERE "subjectId" IS NULL;
    ALTER TABLE "SubjectEnrollment" ALTER COLUMN "subjectId" SET NOT NULL;
  END IF;

  -- ============================================================
  -- LessonProgress (TopicProgress): ensure userId, sub_topic_id never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = progress_table
    AND column_name = user_id_column AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "TopicProgress" WHERE "userId" IS NULL;
    ALTER TABLE "TopicProgress" ALTER COLUMN "userId" SET NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = progress_table
    AND column_name = 'sub_topic_id' AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "TopicProgress" WHERE "sub_topic_id" IS NULL;
    ALTER TABLE "TopicProgress" ALTER COLUMN "sub_topic_id" SET NOT NULL;
  END IF;

  -- ============================================================
  -- Payment: ensure userId, amount, status never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = payment_table
    AND column_name = user_id_column AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "Payment" WHERE "userId" IS NULL;
    ALTER TABLE "Payment" ALTER COLUMN "userId" SET NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = payment_table
    AND column_name = 'amount' AND is_nullable = 'YES'
  ) THEN
    UPDATE "Payment" SET "amount" = 0 WHERE "amount" IS NULL;
    ALTER TABLE "Payment" ALTER COLUMN "amount" SET NOT NULL;
  END IF;

  -- ============================================================
  -- Notification: ensure userId, title never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = notification_table
    AND column_name = user_id_column AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "Notification" WHERE "userId" IS NULL;
    ALTER TABLE "Notification" ALTER COLUMN "userId" SET NOT NULL;
  END IF;

  -- ============================================================
  -- Task: ensure userId, title never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = task_table
    AND column_name = user_id_column AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "Task" WHERE "userId" IS NULL;
    ALTER TABLE "Task" ALTER COLUMN "userId" SET NOT NULL;
  END IF;

  -- ============================================================
  -- StudySession: ensure userId never null
  -- ============================================================

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = schema_name AND table_name = study_session_table
    AND column_name = user_id_column AND is_nullable = 'YES'
  ) THEN
    DELETE FROM "StudySession" WHERE "userId" IS NULL;
    ALTER TABLE "StudySession" ALTER COLUMN "userId" SET NOT NULL;
  END IF;
END $$;

-- ============================================================
-- UNIQUE constraints for data that must be unique
-- ============================================================

DO $$
DECLARE
  schema_name CONSTANT TEXT := 'public';
BEGIN
  -- UserSettings: one settings record per user (if not already enforced)
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_user_settings_user_id'
  ) THEN
    ALTER TABLE "UserSettings" ADD CONSTRAINT uq_user_settings_user_id UNIQUE ("userId");
  END IF;

  -- TwoFactorSettings: one 2FA record per user
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_2fa_settings_user_id'
  ) THEN
    ALTER TABLE "TwoFactorSettings" ADD CONSTRAINT uq_2fa_settings_user_id UNIQUE ("userId");
  END IF;

  -- Schedule: one schedule per user
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_schedule_user_id'
  ) THEN
    ALTER TABLE "Schedule" ADD CONSTRAINT uq_schedule_user_id UNIQUE ("userId");
  END IF;

  -- Invoice: payment_id must be unique (one invoice per payment)
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_invoice_payment_id'
  ) THEN
    ALTER TABLE "Invoice" ADD CONSTRAINT uq_invoice_payment_id UNIQUE ("paymentId");
  END IF;

  -- PushToken: token must be unique globally
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_push_token_token'
  ) THEN
    -- First, remove duplicate tokens (keep the most recent)
    DELETE FROM "PushToken" a USING (
      SELECT MIN(ctid) as ctid, token
      FROM "PushToken"
      GROUP BY token HAVING COUNT(*) > 1
    ) b
    WHERE a.token = b.token AND a.ctid <> b.ctid;
    ALTER TABLE "PushToken" ADD CONSTRAINT uq_push_token_token UNIQUE ("token");
  END IF;

  -- Coupon: code must be unique (should already be, but enforce)
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_coupon_code'
  ) THEN
    ALTER TABLE "Coupon" ADD CONSTRAINT uq_coupon_code UNIQUE ("code");
  END IF;

  -- Category: slug + type must be unique
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_category_slug_type'
  ) THEN
    ALTER TABLE "Category" ADD CONSTRAINT uq_category_slug_type UNIQUE ("slug", "type");
  END IF;

  -- UserAchievement: user can only unlock an achievement once
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_user_achievement_user_achievement'
  ) THEN
    -- Remove duplicates first
    DELETE FROM "UserAchievement" a USING (
      SELECT MIN(ctid) as ctid, "userId", "achievementId"
      FROM "UserAchievement"
      GROUP BY "userId", "achievementId" HAVING COUNT(*) > 1
    ) b
    WHERE a."userId" = b."userId" AND a."achievementId" = b."achievementId" AND a.ctid <> b.ctid;
    ALTER TABLE "UserAchievement" ADD CONSTRAINT uq_user_achievement_user_achievement UNIQUE ("userId", "achievementId");
  END IF;

  -- UserChallenge: user can only have one entry per challenge
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_user_challenge_user_challenge'
  ) THEN
    DELETE FROM "UserChallenge" a USING (
      SELECT MIN(ctid) as ctid, "userId", "challengeId"
      FROM "UserChallenge"
      GROUP BY "userId", "challengeId" HAVING COUNT(*) > 1
    ) b
    WHERE a."userId" = b."userId" AND a."challengeId" = b."challengeId" AND a.ctid <> b.ctid;
    ALTER TABLE "UserChallenge" ADD CONSTRAINT uq_user_challenge_user_challenge UNIQUE ("userId", "challengeId");
  END IF;

  -- BlogPost: slug must be unique (should already be uniqueIndex, but enforce at DB level)
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_blog_post_slug'
  ) THEN
    ALTER TABLE "BlogPost" ADD CONSTRAINT uq_blog_post_slug UNIQUE ("slug");
  END IF;

  -- SystemSetting: key must be unique
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_system_setting_key'
  ) THEN
    ALTER TABLE "SystemSetting" ADD CONSTRAINT uq_system_setting_key UNIQUE ("key");
  END IF;

  -- SupportTicket: ticket_number must be unique
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_support_ticket_number'
  ) THEN
    ALTER TABLE "SupportTicket" ADD CONSTRAINT uq_support_ticket_number UNIQUE ("ticket_number");
  END IF;

  -- Payment: reference must be unique
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_payment_reference'
  ) THEN
    ALTER TABLE "Payment" ADD CONSTRAINT uq_payment_reference UNIQUE ("reference");
  END IF;

  -- Invoice: invoice_number must be unique
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_invoice_number'
  ) THEN
    ALTER TABLE "Invoice" ADD CONSTRAINT uq_invoice_number UNIQUE ("invoice_number");
  END IF;

  -- Achievement: key must be unique
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_achievement_key'
  ) THEN
    ALTER TABLE "Achievement" ADD CONSTRAINT uq_achievement_key UNIQUE ("key");
  END IF;

  -- Subject: code must be unique (if not already)
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'uq_subject_code'
  ) THEN
    -- Remove null codes first (they can't be in a unique constraint)
    -- Use a partial unique index instead
    CREATE UNIQUE INDEX idx_subject_code_unique ON "Subject" ("code") WHERE "code" IS NOT NULL;
  END IF;

  -- Subject: slug must be unique (partial for nulls)
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes WHERE indexname = 'idx_subject_slug_unique'
  ) THEN
    CREATE UNIQUE INDEX idx_subject_slug_unique ON "Subject" ("slug") WHERE "slug" IS NOT NULL;
  END IF;

  -- Username must be unique (partial for nulls)
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes WHERE indexname = 'idx_user_username_unique'
  ) THEN
    CREATE UNIQUE INDEX idx_user_username_unique ON "User" ("username") WHERE "username" IS NOT NULL;
  END IF;
END $$;

-- ============================================================
-- Prevent future ExamResult with score exceeding max_score
-- ============================================================

-- This is a cross-table constraint; we implement it via a trigger
CREATE OR REPLACE FUNCTION validate_exam_result_score()
RETURNS TRIGGER AS $$
DECLARE
  v_max_score FLOAT;
BEGIN
  SELECT "maxScore" INTO v_max_score FROM "Exam" WHERE "id" = NEW."exam_id";
  IF v_max_score IS NOT NULL AND NEW."score" > v_max_score THEN
    RAISE EXCEPTION 'ExamResult score (%) exceeds exam max_score (%)', NEW."score", v_max_score;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_validate_exam_result_score ON "ExamResult";
CREATE TRIGGER trg_validate_exam_result_score
  BEFORE INSERT OR UPDATE ON "ExamResult"
  FOR EACH ROW
  EXECUTE FUNCTION validate_exam_result_score();

-- ============================================================
-- Prevent enrollment in non-active subjects
-- ============================================================

CREATE OR REPLACE FUNCTION validate_enrollment_subject_active()
RETURNS TRIGGER AS $$
DECLARE
  v_is_active BOOLEAN;
BEGIN
  SELECT "isActive" INTO v_is_active FROM "Subject" WHERE "id" = NEW."subjectId";
  IF v_is_active IS NOT NULL AND NOT v_is_active THEN
    RAISE EXCEPTION 'Cannot enroll in inactive subject (id: %)', NEW."subjectId";
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_validate_enrollment_subject ON "SubjectEnrollment";
CREATE TRIGGER trg_validate_enrollment_subject
  BEFORE INSERT ON "SubjectEnrollment"
  FOR EACH ROW
  EXECUTE FUNCTION validate_enrollment_subject_active();

-- ============================================================
-- Prevent lesson progress for non-enrolled users
-- ============================================================

CREATE OR REPLACE FUNCTION validate_progress_enrollment()
RETURNS TRIGGER AS $$
DECLARE
  v_enrolled BOOLEAN;
  v_subject_id UUID;
BEGIN
  -- Get the subject ID for this subtopic
  SELECT "subjectId" INTO v_subject_id
  FROM "SubTopic" st
  JOIN "Topic" t ON t."id" = st."topicId"
  WHERE st."id" = NEW."sub_topic_id";

  -- Check if user is enrolled in this subject
  SELECT EXISTS (
    SELECT 1 FROM "SubjectEnrollment"
    WHERE "userId" = NEW."userId" AND "subjectId" = v_subject_id
  ) INTO v_enrolled;

  IF NOT v_enrolled THEN
    RAISE EXCEPTION 'User (%) is not enrolled in subject (%) for sub_topic (%)', NEW."userId", v_subject_id, NEW."sub_topic_id";
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_validate_progress_enrollment ON "TopicProgress";
CREATE TRIGGER trg_validate_progress_enrollment
  BEFORE INSERT ON "TopicProgress"
  FOR EACH ROW
  EXECUTE FUNCTION validate_progress_enrollment();

COMMIT;
