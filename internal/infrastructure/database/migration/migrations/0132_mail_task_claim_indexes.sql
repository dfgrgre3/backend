-- ============================================================
-- Migration 0132: Support atomic claiming of MailTask rows
-- ============================================================
-- The worker now claims rows with FOR UPDATE SKIP LOCKED and moves them to an
-- intermediate 'processing' status, so that several worker processes (rolling
-- deploys, APP_MODE=worker replicas) cannot dispatch the same email twice.
--
-- The claim query is:
--   SELECT id FROM "MailTask"
--   WHERE deleted_at IS NULL
--     AND status IN ('pending','retry')
--     AND (status = 'pending' OR updated_at <= NOW() - make_interval(...))
--   ORDER BY created_at ASC LIMIT n FOR UPDATE SKIP LOCKED
-- which is already served by idx_mail_task_status_deleted_created (0126/0127).
--
-- What is still missing is an index for the reaper query that returns rows
-- abandoned by a crashed worker:
--   WHERE status = 'processing' AND deleted_at IS NULL AND updated_at < cutoff

CREATE INDEX IF NOT EXISTS idx_mail_task_processing_stale
ON public."MailTask" ("updated_at" ASC)
WHERE "deleted_at" IS NULL AND "status" = 'processing';

-- The retry branch of the claim query filters on updated_at, so keep it in the
-- index to avoid a heap fetch per candidate row.
CREATE INDEX IF NOT EXISTS idx_mail_task_retry_ready
ON public."MailTask" ("created_at" ASC)
INCLUDE ("updated_at", "attempts", "max_retry")
WHERE "deleted_at" IS NULL AND "status" = 'retry';

ANALYZE public."MailTask";
