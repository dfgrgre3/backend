-- 0112_add_affiliates_advanced.sql
-- Extend the affiliate system with tier rules, campaigns, links, payouts,
-- settings, bonuses and link-click tracking.

-- ---------------------------------------------------------------------------
-- 1. Augment Affiliate with lifecycle + tracking columns
-- ---------------------------------------------------------------------------
ALTER TABLE "Affiliate"
    ADD COLUMN IF NOT EXISTS "approved_at"         TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "approved_by"         UUID REFERENCES "User"("id") ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS "payout_method"       VARCHAR(32),
    ADD COLUMN IF NOT EXISTS "payout_details"      JSONB,
    ADD COLUMN IF NOT EXISTS "minimum_payout"      DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "hold_days"           INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "clicks_count"        INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "conversions_count"   INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "last_activity_at"    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "notes"               TEXT;

CREATE INDEX IF NOT EXISTS "Affiliate_last_activity_at_idx"
    ON "Affiliate"("last_activity_at");

-- ---------------------------------------------------------------------------
-- 2. AffiliateTierRule: configurable commission/tier rules
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "AffiliateTierRule" (
    "id"             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "tier"           VARCHAR(32) NOT NULL UNIQUE,
    "name_ar"        VARCHAR(64) NOT NULL,
    "commission_rate" DOUBLE PRECISION NOT NULL DEFAULT 10,
    "min_revenue"    DOUBLE PRECISION NOT NULL DEFAULT 0,
    "min_referrals"  INT NOT NULL DEFAULT 0,
    "bonus_rate"     DOUBLE PRECISION NOT NULL DEFAULT 0,
    "color"          VARCHAR(32) NOT NULL DEFAULT 'amber',
    "sort_order"     INT NOT NULL DEFAULT 0,
    "active"         BOOLEAN NOT NULL DEFAULT TRUE,
    "metadata"       JSONB NOT NULL DEFAULT '{}'::jsonb,
    "created_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "deleted_at"     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "AffiliateTierRule_active_idx" ON "AffiliateTierRule"("active");
CREATE INDEX IF NOT EXISTS "AffiliateTierRule_sort_order_idx" ON "AffiliateTierRule"("sort_order");

-- ---------------------------------------------------------------------------
-- 3. AffiliateCampaign: marketing campaigns affiliates can join
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "AffiliateCampaign" (
    "id"             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "name"           VARCHAR(128) NOT NULL,
    "slug"           VARCHAR(128) NOT NULL UNIQUE,
    "description"    TEXT,
    "status"         VARCHAR(32) NOT NULL DEFAULT 'DRAFT',
    "start_date"     TIMESTAMPTZ,
    "end_date"       TIMESTAMPTZ,
    "commission_rate" DOUBLE PRECISION,
    "budget"         DOUBLE PRECISION,
    "spent"          DOUBLE PRECISION NOT NULL DEFAULT 0,
    "banner_url"     TEXT,
    "landing_url"    TEXT,
    "promo_code"     VARCHAR(64),
    "metadata"       JSONB NOT NULL DEFAULT '{}'::jsonb,
    "created_by"     UUID REFERENCES "User"("id") ON DELETE SET NULL,
    "created_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "deleted_at"     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "AffiliateCampaign_status_idx" ON "AffiliateCampaign"("status");
CREATE INDEX IF NOT EXISTS "AffiliateCampaign_start_date_idx" ON "AffiliateCampaign"("start_date");
CREATE INDEX IF NOT EXISTS "AffiliateCampaign_end_date_idx" ON "AffiliateCampaign"("end_date");

-- ---------------------------------------------------------------------------
-- 4. AffiliateLink: dedicated tracking links per (affiliate × campaign × channel)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "AffiliateLink" (
    "id"             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "affiliate_id"   UUID NOT NULL REFERENCES "Affiliate"("id") ON DELETE CASCADE,
    "campaign_id"    UUID REFERENCES "AffiliateCampaign"("id") ON DELETE SET NULL,
    "name"           VARCHAR(128) NOT NULL,
    "slug"           VARCHAR(128) NOT NULL UNIQUE,
    "destination_url" TEXT NOT NULL,
    "utm_source"     VARCHAR(64),
    "utm_medium"     VARCHAR(64),
    "utm_campaign"   VARCHAR(64),
    "clicks_count"   INT NOT NULL DEFAULT 0,
    "unique_clicks_count" INT NOT NULL DEFAULT 0,
    "conversions_count" INT NOT NULL DEFAULT 0,
    "active"         BOOLEAN NOT NULL DEFAULT TRUE,
    "metadata"       JSONB NOT NULL DEFAULT '{}'::jsonb,
    "created_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "deleted_at"     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "AffiliateLink_affiliate_id_idx" ON "AffiliateLink"("affiliate_id");
CREATE INDEX IF NOT EXISTS "AffiliateLink_campaign_id_idx" ON "AffiliateLink"("campaign_id");
CREATE INDEX IF NOT EXISTS "AffiliateLink_active_idx" ON "AffiliateLink"("active");

-- ---------------------------------------------------------------------------
-- 5. AffiliateLinkClick: per-click tracking log (cap to 1M rows for sanity)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "AffiliateLinkClick" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "link_id"       UUID NOT NULL REFERENCES "AffiliateLink"("id") ON DELETE CASCADE,
    "affiliate_id"  UUID NOT NULL REFERENCES "Affiliate"("id") ON DELETE CASCADE,
    "ip_hash"       VARCHAR(128),
    "user_agent"    TEXT,
    "referer"       TEXT,
    "country"       VARCHAR(8),
    "device"        VARCHAR(32),
    "converted"     BOOLEAN NOT NULL DEFAULT FALSE,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS "AffiliateLinkClick_link_id_created_at_idx"
    ON "AffiliateLinkClick"("link_id", "created_at");
CREATE INDEX IF NOT EXISTS "AffiliateLinkClick_affiliate_id_idx"
    ON "AffiliateLinkClick"("affiliate_id");

-- ---------------------------------------------------------------------------
-- 6. AffiliatePayout: batch / individual payout history (in addition to ad-hoc pay endpoint)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "AffiliatePayout" (
    "id"             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "affiliate_id"   UUID NOT NULL REFERENCES "Affiliate"("id") ON DELETE CASCADE,
    "amount"         DOUBLE PRECISION NOT NULL,
    "currency"       VARCHAR(8) NOT NULL DEFAULT 'EGP',
    "status"         VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    "method"         VARCHAR(32),
    "reference"      VARCHAR(128),
    "notes"          TEXT,
    "processed_by"   UUID REFERENCES "User"("id") ON DELETE SET NULL,
    "processed_at"   TIMESTAMPTZ,
    "referral_ids"   JSONB NOT NULL DEFAULT '[]'::jsonb,
    "metadata"       JSONB NOT NULL DEFAULT '{}'::jsonb,
    "created_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS "AffiliatePayout_affiliate_id_idx" ON "AffiliatePayout"("affiliate_id");
CREATE INDEX IF NOT EXISTS "AffiliatePayout_status_idx" ON "AffiliatePayout"("status");
CREATE INDEX IF NOT EXISTS "AffiliatePayout_created_at_idx" ON "AffiliatePayout"("created_at");

-- ---------------------------------------------------------------------------
-- 7. AffiliateSetting: key/value system settings (singleton row key='default')
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "AffiliateSetting" (
    "id"                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "key"                         VARCHAR(64) NOT NULL UNIQUE,
    "default_commission_rate"     DOUBLE PRECISION NOT NULL DEFAULT 10,
    "default_tier"                VARCHAR(32) NOT NULL DEFAULT 'BRONZE',
    "auto_approve"                BOOLEAN NOT NULL DEFAULT TRUE,
    "minimum_payout"              DOUBLE PRECISION NOT NULL DEFAULT 0,
    "hold_days"                   INT NOT NULL DEFAULT 0,
    "cookie_days"                 INT NOT NULL DEFAULT 30,
    "allow_self_referral"         BOOLEAN NOT NULL DEFAULT FALSE,
    "email_template_welcome"      TEXT,
    "email_template_payout"       TEXT,
    "notify_on_signup"            BOOLEAN NOT NULL DEFAULT TRUE,
    "notify_on_payout"            BOOLEAN NOT NULL DEFAULT TRUE,
    "metadata"                    JSONB NOT NULL DEFAULT '{}'::jsonb,
    "updated_by"                  UUID REFERENCES "User"("id") ON DELETE SET NULL,
    "created_at"                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- 8. AffiliateAudit: dedicated audit log for affiliate actions (in addition to generic LogAudit)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "AffiliateAudit" (
    "id"           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "affiliate_id" UUID REFERENCES "Affiliate"("id") ON DELETE CASCADE,
    "actor_id"     UUID REFERENCES "User"("id") ON DELETE SET NULL,
    "action"       VARCHAR(64) NOT NULL,
    "target"       VARCHAR(64),
    "details"      JSONB NOT NULL DEFAULT '{}'::jsonb,
    "ip"           VARCHAR(64),
    "created_at"   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS "AffiliateAudit_affiliate_id_idx" ON "AffiliateAudit"("affiliate_id");
CREATE INDEX IF NOT EXISTS "AffiliateAudit_action_idx" ON "AffiliateAudit"("action");
CREATE INDEX IF NOT EXISTS "AffiliateAudit_created_at_idx" ON "AffiliateAudit"("created_at");

-- ---------------------------------------------------------------------------
-- 9. Seed default tier rules + default setting row (idempotent)
-- ---------------------------------------------------------------------------
INSERT INTO "AffiliateTierRule" ("tier", "name_ar", "commission_rate", "min_revenue", "min_referrals", "bonus_rate", "color", "sort_order", "active")
VALUES
    ('BRONZE',   'برونزي',   10,     0,    0, 0,  'amber',  1, TRUE),
    ('SILVER',   'فضي',      12,  5000,   10, 1,  'slate',  2, TRUE),
    ('GOLD',     'ذهبي',     15, 20000,   30, 2,  'yellow', 3, TRUE),
    ('PLATINUM', 'بلاتيني',  20, 50000,  100, 3,  'cyan',   4, TRUE)
ON CONFLICT ("tier") DO NOTHING;

INSERT INTO "AffiliateSetting" ("key", "default_commission_rate", "default_tier", "auto_approve")
VALUES ('default', 10, 'BRONZE', TRUE)
ON CONFLICT ("key") DO NOTHING;