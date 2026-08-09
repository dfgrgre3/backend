-- ============================================================
-- Migration 0128: Add Optimized Database Indexes for /api/notifications
-- ============================================================
-- Optimizes:
-- 1. Main endpoint: GET /api/notifications
--    Query: WHERE user_id = $1 ORDER BY created_at DESC LIMIT n OFFSET m
--    Covering B-Tree index to perform Index Only Scan on requested columns.
--
-- 2. Unread notification count endpoint: GET /api/notifications/unread-count
--    Query: WHERE user_id = $1 AND is_read = false
--    Partial index filtering only unread records.
--
-- 3. Unread notification fetching (NotificationService.GetUnreadNotifications):
--    Query: WHERE user_id = $1 AND status IN ('delivered', 'sent') ORDER BY created_at DESC
--    Composite partial index for active/unread delivery status filtering.
-- ============================================================

-- Composite covering index for general user notification pagination
CREATE INDEX IF NOT EXISTS idx_notifications_user_id_created_at_covering
ON public."Notification" ("user_id", "created_at" DESC)
INCLUDE ("id", "title", "message", "type", "is_read", "link", "icon");

-- Partial composite index for counting/fetching unread notifications per user
CREATE INDEX IF NOT EXISTS idx_notifications_user_id_unread_partial
ON public."Notification" ("user_id", "created_at" DESC)
WHERE "is_read" = false;

-- Composite partial index for delivered/sent notification filtering
CREATE INDEX IF NOT EXISTS idx_notifications_user_id_status_delivered_sent
ON public."Notification" ("user_id", "created_at" DESC)
WHERE "status" IN ('delivered', 'sent');

-- Update PostgreSQL statistics for the Notification table
ANALYZE public."Notification";
