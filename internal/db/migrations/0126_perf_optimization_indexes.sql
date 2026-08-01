-- Migration 0126: Add composite indexes for MailTask, SystemSetting, Notification, and UserSession tables
-- Fixes slow SQL queries reported during server startup and runtime operations.

-- 1. Index for MailTask worker query (WHERE status IN ('pending', 'retry') AND deleted_at IS NULL ORDER BY created_at ASC)
CREATE INDEX IF NOT EXISTS idx_mail_task_status_deleted_created 
ON public."MailTask" ("status", "created_at" ASC) 
WHERE deleted_at IS NULL AND status IN ('pending', 'retry');

-- 2. Index for SystemSetting key lookup (WHERE key = $1 AND deleted_at IS NULL)
CREATE INDEX IF NOT EXISTS idx_system_setting_key_deleted 
ON public."SystemSetting" ("key") 
WHERE deleted_at IS NULL;

-- 3. Index for Notification fetch per user (WHERE user_id = $1 ORDER BY created_at DESC)
CREATE INDEX IF NOT EXISTS idx_notification_user_created 
ON public."Notification" ("user_id", "created_at" DESC);

-- 4. Index for active UserSession count and lookup (WHERE user_id = $1 AND is_active = true AND deleted_at IS NULL)
CREATE INDEX IF NOT EXISTS idx_user_session_user_active 
ON public."UserSession" ("user_id", "is_active", "last_accessed" ASC) 
WHERE deleted_at IS NULL AND is_active = true;

-- Update optimizer stats for affected tables
ANALYZE public."MailTask";
ANALYZE public."SystemSetting";
ANALYZE public."Notification";
ANALYZE public."UserSession";
