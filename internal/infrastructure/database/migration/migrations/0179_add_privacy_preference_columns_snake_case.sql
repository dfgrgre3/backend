-- ============================================================
-- Migration 0179: Add the snake_case counterparts of the privacy/data
-- preference columns introduced in 0155.
--
-- 0155 added these columns in camelCase ("showLastSeen", etc.), matching
-- the older Prisma-style columns already present on UserSettings. GORM's
-- naming strategy, however, writes/reads snake_case columns (as it does
-- for every other UserSettings field, e.g. font_size, high_contrast), so
-- inserts from the Go application fail with:
--   column "show_last_seen" of relation "UserSettings" does not exist
-- ============================================================

ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS show_last_seen boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS show_achievements boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS allow_messages text DEFAULT 'everyone'::text;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS allow_friend_requests boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS data_collection boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS personalization boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS analytics boolean DEFAULT true;

-- Backfill snake_case values from the legacy camelCase columns where they
-- already diverge from default (best-effort; both sets are DEFAULT true /
-- 'everyone', so this mainly matters for rows touched by application code
-- that historically wrote only to the camelCase columns).
UPDATE public."UserSettings" SET show_last_seen = "showLastSeen" WHERE "showLastSeen" IS NOT NULL;
UPDATE public."UserSettings" SET show_achievements = "showAchievements" WHERE "showAchievements" IS NOT NULL;
UPDATE public."UserSettings" SET allow_messages = "allowMessages" WHERE "allowMessages" IS NOT NULL;
UPDATE public."UserSettings" SET allow_friend_requests = "allowFriendRequests" WHERE "allowFriendRequests" IS NOT NULL;
UPDATE public."UserSettings" SET data_collection = "dataCollection" WHERE "dataCollection" IS NOT NULL;
UPDATE public."UserSettings" SET personalization = "personalization" WHERE "personalization" IS NOT NULL;
UPDATE public."UserSettings" SET analytics = "analytics" WHERE "analytics" IS NOT NULL;
