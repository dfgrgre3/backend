-- ============================================================
-- Migration 0161: Persisted visibility for admin announcements
-- ============================================================
-- Admin announcements share the shared "Notification" table. The
-- admin panel previously sent an "isActive" flag that the backend
-- could not persist (it was hard-coded to true on read), so
-- publish/unpublish had no real effect.
--
-- This migration adds a real is_active column so the admin panel
-- can publish / unpublish announcements (and notifications) across
-- the platform. Existing rows default to active, so nothing
-- disappears after the upgrade.
-- ============================================================

ALTER TABLE public."Notification" ADD COLUMN IF NOT EXISTS is_active boolean NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS idx_notification_is_active ON public."Notification" (is_active);
