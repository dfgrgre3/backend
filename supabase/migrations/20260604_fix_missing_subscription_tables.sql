-- ============================================================================
-- Migration: 20260604_fix_missing_subscription_tables.sql
-- Purpose:   Create the UserSubscription and SubscriptionPlan tables that
--            are referenced by the Go models (internal/models/subscription.go)
--            but were missing from the database, causing the runtime error
--            `relation "UserSubscription" does not exist (SQLSTATE 42P01)` in
--            handlers/user_handler.go:1411 (fetchActiveSubscription).
--
-- Also adds performance indexes that are missing in the current schema and
-- speeds up the slow queries reported in the application logs:
--   * `UserSession` UPDATE on (id, refresh_token_hash, is_active)
--   * `User` SELECT by id
--   * `Payment` lookups by user_id
--   * `Notification` lookups by user_id
--
-- Safe to run on a fresh install or on top of an existing schema:
-- uses `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`.
-- ============================================================================

BEGIN;

-- Rename camelCase columns to snake_case if they exist in SubscriptionPlan
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='SubscriptionPlan' AND column_name='nameAr') THEN
    ALTER TABLE "SubscriptionPlan" RENAME COLUMN "nameAr" TO "name_ar";
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='SubscriptionPlan' AND column_name='isActive') THEN
    ALTER TABLE "SubscriptionPlan" RENAME COLUMN "isActive" TO "is_active";
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='SubscriptionPlan' AND column_name='createdAt') THEN
    ALTER TABLE "SubscriptionPlan" RENAME COLUMN "createdAt" TO "created_at";
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='SubscriptionPlan' AND column_name='updatedAt') THEN
    ALTER TABLE "SubscriptionPlan" RENAME COLUMN "updatedAt" TO "updated_at";
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 1. SubscriptionPlan (must exist before UserSubscription due to FK)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "SubscriptionPlan" (
    id          UUID         PRIMARY KEY,
    name        TEXT         NOT NULL UNIQUE,
    name_ar     TEXT         NOT NULL,
    description TEXT,
    price       DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency    TEXT         NOT NULL DEFAULT 'EGP',
    interval    TEXT         NOT NULL DEFAULT 'MONTHLY',
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    features    JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT subscription_plan_interval_chk
        CHECK (interval IN ('MONTHLY','YEARLY','FOREVER')),
    CONSTRAINT subscription_plan_price_chk
        CHECK (price >= 0)
);

CREATE INDEX IF NOT EXISTS idx_subscription_plan_is_active
    ON "SubscriptionPlan" (is_active);

-- ---------------------------------------------------------------------------
-- 2. UserSubscription
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "UserSubscription" (
    id                       UUID         PRIMARY KEY,
    user_id                  UUID         NOT NULL,
    plan_id                  UUID         NOT NULL,
    status                   TEXT         NOT NULL DEFAULT 'PENDING',
    start_date               TIMESTAMPTZ  NOT NULL,
    end_date                 TIMESTAMPTZ  NOT NULL,
    auto_renew               BOOLEAN      NOT NULL DEFAULT TRUE,
    paymob_subscription_id   TEXT,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT user_subscription_status_chk
        CHECK (status IN ('ACTIVE','CANCELLED','EXPIRED','PENDING')),
    CONSTRAINT user_subscription_user_fk
        FOREIGN KEY (user_id) REFERENCES "User"(id) ON DELETE CASCADE,
    CONSTRAINT user_subscription_plan_fk
        FOREIGN KEY (plan_id) REFERENCES "SubscriptionPlan"(id)
);

-- Indexes that match what the Go GORM model declares.
CREATE INDEX IF NOT EXISTS idx_user_subscription_user_id
    ON "UserSubscription" (user_id);
CREATE INDEX IF NOT EXISTS idx_user_subscription_plan_id
    ON "UserSubscription" (plan_id);
CREATE INDEX IF NOT EXISTS idx_user_subscription_status
    ON "UserSubscription" (status);
CREATE INDEX IF NOT EXISTS idx_user_subscription_end_date
    ON "UserSubscription" (end_date);
CREATE INDEX IF NOT EXISTS idx_user_subscription_paymob
    ON "UserSubscription" (paymob_subscription_id);

-- Composite index that speeds up the exact query used in
-- fetchActiveSubscription: WHERE user_id = ? AND status = ? AND end_date > ?
CREATE INDEX IF NOT EXISTS idx_user_subscription_active_lookup
    ON "UserSubscription" (user_id, status, end_date);

-- ---------------------------------------------------------------------------
-- 3. Performance indexes (PostgreSQL only — guarded by IF NOT EXISTS)
-- ---------------------------------------------------------------------------

-- Speed up `SELECT * FROM "User" WHERE id = $1` (517ms slow query in logs)
CREATE INDEX IF NOT EXISTS idx_user_pkey_id
    ON "User" (id);

-- Speed up `UPDATE "UserSession" SET ... WHERE (id = ? AND refresh_token_hash = ? AND is_active = ?)`
-- (the 569-669ms slow queries reported in session_repo.go:66)
-- We already have a unique index on refresh_token_hash; this composite
-- index makes the lookup by id+hash+is_active a single index scan.
CREATE INDEX IF NOT EXISTS idx_user_session_id_hash_active
    ON "UserSession" (id, refresh_token_hash, is_active);

-- Speed up payment history lookups
CREATE INDEX IF NOT EXISTS idx_payment_user_created
    ON "Payment" (user_id, created_at DESC);

-- Speed up notification lookups (unread count, list)
CREATE INDEX IF NOT EXISTS idx_notification_user_read_created
    ON "Notification" (user_id, is_read, created_at DESC);

-- ---------------------------------------------------------------------------
-- 4. Seed a few default plans (only if the table is empty)
-- ---------------------------------------------------------------------------
INSERT INTO "SubscriptionPlan" (id, name, name_ar, description, price, currency, interval, is_active, features, created_at, updated_at)
SELECT
    gen_random_uuid(),
    plan.name,
    plan.name_ar,
    plan.description,
    plan.price,
    'EGP',
    plan.interval::"PlanInterval",
    TRUE,
    plan.features::text[],
    NOW(),
    NOW()
FROM (VALUES
    ('FREE',        'مجاني',        'الخطة المجانية للطلاب',          0.0,    'FOREVER', ARRAY['study_planner','basic_exams','community_access']),
    ('BASIC',       'الأساسية',     'الخطة الأساسية',                  99.0,   'MONTHLY', ARRAY['study_planner','all_exams','ai_assistant_limited']),
    ('PREMIUM',     'المتميزة',     'الخطة المتميزة مع كل المميزات',  249.0,  'MONTHLY', ARRAY['unlimited_ai','unlimited_exams','certificates','priority_support']),
    ('ANNUAL',      'السنوية',      'الخطة السنوية بخصم 30%',          1999.0, 'YEARLY',  ARRAY['unlimited_ai','unlimited_exams','certificates','priority_support'])
) AS plan(name, name_ar, description, price, interval, features)
WHERE NOT EXISTS (SELECT 1 FROM "SubscriptionPlan" WHERE "SubscriptionPlan".name = plan.name);

-- ---------------------------------------------------------------------------
-- 5. Fix Book table camelCase column duplicates causing NOT NULL constraint violations
-- ---------------------------------------------------------------------------
DO $$
BEGIN
  -- Sync data from camelCase columns to snake_case columns if they have data
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='downloadUrl') THEN
    UPDATE "Book" SET download_url = "downloadUrl" WHERE download_url IS NULL AND "downloadUrl" IS NOT NULL;
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='coverUrl') THEN
    UPDATE "Book" SET cover_url = "coverUrl" WHERE cover_url IS NULL AND "coverUrl" IS NOT NULL;
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='subjectId') THEN
    UPDATE "Book" SET subject_id = CAST("subjectId" AS uuid) WHERE subject_id IS NULL AND "subjectId" IS NOT NULL;
  END IF;

  -- Rename createdAt and updatedAt to created_at and updated_at if they exist
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='createdAt') THEN
    ALTER TABLE "Book" RENAME COLUMN "createdAt" TO "created_at";
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='updatedAt') THEN
    ALTER TABLE "Book" RENAME COLUMN "updatedAt" TO "updated_at";
  END IF;

  -- Now drop the duplicate camelCase columns
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='downloadUrl') THEN
    ALTER TABLE "Book" DROP COLUMN "downloadUrl";
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='coverUrl') THEN
    ALTER TABLE "Book" DROP COLUMN "coverUrl";
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='subjectId') THEN
    ALTER TABLE "Book" DROP COLUMN "subjectId";
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='Book' AND column_name='uploaderId') THEN
    ALTER TABLE "Book" DROP COLUMN "uploaderId";
  END IF;
END $$;

COMMIT;
