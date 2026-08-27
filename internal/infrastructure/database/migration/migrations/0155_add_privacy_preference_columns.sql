-- ============================================================
-- Migration 0155: Add remaining privacy/data-preference columns to
-- UserSettings so the Privacy settings page's toggles actually persist
-- instead of being silently dropped by validateSettingsPatch's
-- unknown-key-ignore behavior.
-- ============================================================

ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS "showLastSeen" boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS "showAchievements" boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS "allowMessages" text DEFAULT 'everyone'::text;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS "allowFriendRequests" boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS "dataCollection" boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS "personalization" boolean DEFAULT true;
ALTER TABLE public."UserSettings" ADD COLUMN IF NOT EXISTS "analytics" boolean DEFAULT true;
