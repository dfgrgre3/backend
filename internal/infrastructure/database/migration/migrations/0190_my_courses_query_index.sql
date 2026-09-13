-- =============================================================
-- 0190: Optimize the per-user my-courses listing query.
-- =============================================================
-- Covers the legacy and cursor endpoints:
--   WHERE user_id = $1 AND deleted_at IS NULL
--   ORDER BY updated_at DESC, id DESC
--
-- The existing user-only index still requires a sort before LIMIT can
-- be applied. This partial composite index lets PostgreSQL read the
-- newest active enrollment rows in the requested order.
CREATE INDEX IF NOT EXISTS idx_subject_enrollment_user_updated_active
ON public."SubjectEnrollment" ("user_id", "updated_at" DESC, "id" DESC)
WHERE "deleted_at" IS NULL;