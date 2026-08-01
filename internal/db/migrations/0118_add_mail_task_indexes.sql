-- Add composite indexes for MailTask table to optimize the pending/retry task query
-- The query filters on: status IN ('pending', 'retry') AND deleted_at IS NULL
-- Ordered by: created_at ASC
-- Limited to: 100 rows

-- NOTE: CREATE INDEX CONCURRENTLY was intentionally NOT used here because the
-- migration framework wraps statements in a transaction, which CONCURRENTLY
-- cannot run inside. For zero-downtime production deployments, these indexes
-- should be created manually with CONCURRENTLY outside of the migration tool.

-- Composite partial index for active tasks (most common query pattern)
-- This covers the exact query: WHERE status IN ('pending', 'retry') AND deleted_at IS NULL ORDER BY created_at ASC
-- Optimizes the mail queue worker query that was taking 2137ms
CREATE INDEX IF NOT EXISTS idx_mail_task_status_deleted_created 
ON "MailTask" ("status", "deleted_at", "created_at") 
WHERE "deleted_at" IS NULL AND "status" IN ('pending', 'retry');

-- Partial index for active tasks only (status + deleted_at)
-- Optimizes the WHERE clause filtering
CREATE INDEX IF NOT EXISTS idx_mail_task_status_deleted_active
ON "MailTask" ("status", "deleted_at")
WHERE "deleted_at" IS NULL AND "status" IN ('pending', 'retry', 'sent', 'failed');

-- Individual index on status for other query patterns (e.g., counting by status)
CREATE INDEX IF NOT EXISTS idx_mail_task_status
ON "MailTask" ("status");

-- Individual index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_mail_task_deleted_at
ON "MailTask" ("deleted_at");

-- Update table statistics to help query planner
ANALYZE "MailTask";