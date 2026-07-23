-- 0111_add_affiliates.sql
-- Create Affiliate and AffiliateReferral tables for the affiliate/referral marketing system.

CREATE TABLE IF NOT EXISTS "Affiliate" (
    "id"             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id"        UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "code"           VARCHAR(64) NOT NULL,
    "status"         VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    "commission_rate" DOUBLE PRECISION NOT NULL DEFAULT 10,
    "tier"           VARCHAR(32) NOT NULL DEFAULT 'BRONZE',
    "total_earned"   DOUBLE PRECISION NOT NULL DEFAULT 0,
    "total_paid"     DOUBLE PRECISION NOT NULL DEFAULT 0,
    "created_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "deleted_at"     TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS "Affiliate_user_id_key" ON "Affiliate"("user_id") WHERE "deleted_at" IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS "Affiliate_code_key" ON "Affiliate"("code") WHERE "deleted_at" IS NULL;
CREATE INDEX IF NOT EXISTS "Affiliate_status_idx" ON "Affiliate"("status");
CREATE INDEX IF NOT EXISTS "Affiliate_tier_idx" ON "Affiliate"("tier");
CREATE INDEX IF NOT EXISTS "Affiliate_created_at_idx" ON "Affiliate"("created_at");
CREATE INDEX IF NOT EXISTS "Affiliate_deleted_at_idx" ON "Affiliate"("deleted_at");

CREATE TABLE IF NOT EXISTS "AffiliateReferral" (
    "id"           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "affiliate_id" UUID NOT NULL REFERENCES "Affiliate"("id") ON DELETE CASCADE,
    "user_id"      UUID REFERENCES "User"("id") ON DELETE SET NULL,
    "amount"       DOUBLE PRECISION NOT NULL,
    "commission"   DOUBLE PRECISION NOT NULL,
    "status"       VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    "created_at"   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "deleted_at"   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "AffiliateReferral_affiliate_id_idx" ON "AffiliateReferral"("affiliate_id");
CREATE INDEX IF NOT EXISTS "AffiliateReferral_user_id_idx" ON "AffiliateReferral"("user_id");
CREATE INDEX IF NOT EXISTS "AffiliateReferral_status_idx" ON "AffiliateReferral"("status");
CREATE INDEX IF NOT EXISTS "AffiliateReferral_created_at_idx" ON "AffiliateReferral"("created_at");
CREATE INDEX IF NOT EXISTS "AffiliateReferral_deleted_at_idx" ON "AffiliateReferral"("deleted_at");
